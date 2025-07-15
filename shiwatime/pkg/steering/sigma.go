package steering

import (
	"fmt"
	"math"
	"time"

	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/shiwatime/shiwatime/pkg/types"
	"github.com/sirupsen/logrus"
)

// SigmaSteerer implements the Sigma steering algorithm
type SigmaSteerer struct {
	config           config.SteeringConfig
	logger           *logrus.Entry
	outlierFilter    OutlierFilter
	history          []historySample
	maxHistorySize   int
	frequencyEstimate float64
	lastAdjustment   time.Time
}

type historySample struct {
	timestamp time.Time
	offset    time.Duration
	delay     time.Duration
}

// NewSigmaSteerer creates a new Sigma steerer
func NewSigmaSteerer(cfg config.SteeringConfig) (*SigmaSteerer, error) {
	var filter OutlierFilter
	if cfg.OutlierFilterEnabled {
		filter = NewOutlierFilter(cfg.OutlierFilterType)
	}

	return &SigmaSteerer{
		config:         cfg,
		outlierFilter:  filter,
		maxHistorySize: 60, // Keep 60 seconds of history
		logger: logrus.WithFields(logrus.Fields{
			"steerer": "sigma",
		}),
	}, nil
}

// GetName returns the algorithm name
func (s *SigmaSteerer) GetName() string {
	return "sigma"
}

// Reset resets the algorithm state
func (s *SigmaSteerer) Reset() {
	s.history = nil
	s.frequencyEstimate = 0
	s.lastAdjustment = time.Time{}
}

// CalculateAdjustment calculates the clock adjustment
func (s *SigmaSteerer) CalculateAdjustment(samples []*types.TimeSample) (*ClockAdjustment, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("no samples provided")
	}

	// Filter outliers if enabled
	filteredSamples := samples
	if s.outlierFilter != nil {
		filteredSamples = s.outlierFilter.Filter(samples)
		
		if s.config.AlgoLogging {
			s.logger.WithFields(logrus.Fields{
				"original": len(samples),
				"filtered": len(filteredSamples),
			}).Debug("Outlier filtering")
		}
	}

	if len(filteredSamples) == 0 {
		return nil, fmt.Errorf("all samples filtered out")
	}

	// Calculate weighted average offset
	totalWeight := 0.0
	weightedOffset := 0.0
	
	for _, sample := range filteredSamples {
		// Weight based on quality and delay
		weight := float64(sample.Quality) / 100.0
		if sample.Delay > 0 {
			// Reduce weight for high-delay samples
			delayMs := sample.Delay.Seconds() * 1000
			weight *= math.Exp(-delayMs / 100) // Exponential decay
		}
		
		totalWeight += weight
		weightedOffset += weight * sample.Offset.Seconds()
	}

	if totalWeight == 0 {
		return nil, fmt.Errorf("total weight is zero")
	}

	avgOffset := time.Duration(weightedOffset / totalWeight * float64(time.Second))

	// Update history
	s.updateHistory(avgOffset, filteredSamples)

	// Calculate frequency adjustment
	frequency := s.calculateFrequency()

	// Determine if we should step or slew
	step := false
	if avgOffset.Abs() > 128*time.Millisecond {
		step = true
	}

	adjustment := &ClockAdjustment{
		Offset:    avgOffset,
		Frequency: frequency,
		Step:      step,
		Timestamp: time.Now(),
	}

	s.lastAdjustment = adjustment.Timestamp

	if s.config.AlgoLogging {
		s.logger.WithFields(logrus.Fields{
			"offset":    avgOffset,
			"frequency": frequency,
			"step":      step,
			"samples":   len(filteredSamples),
		}).Debug("Calculated adjustment")
	}

	return adjustment, nil
}

// updateHistory updates the history with new samples
func (s *SigmaSteerer) updateHistory(offset time.Duration, samples []*types.TimeSample) {
	// Calculate average delay for this sample set
	avgDelay := time.Duration(0)
	if len(samples) > 0 {
		totalDelay := int64(0)
		for _, sample := range samples {
			totalDelay += int64(sample.Delay)
		}
		avgDelay = time.Duration(totalDelay / int64(len(samples)))
	}

	// Add to history
	s.history = append(s.history, historySample{
		timestamp: time.Now(),
		offset:    offset,
		delay:     avgDelay,
	})

	// Trim history if too long
	if len(s.history) > s.maxHistorySize {
		s.history = s.history[len(s.history)-s.maxHistorySize:]
	}
}

// calculateFrequency calculates frequency adjustment based on history
func (s *SigmaSteerer) calculateFrequency() float64 {
	if len(s.history) < 2 {
		return s.frequencyEstimate
	}

	// Use linear regression on recent history
	// This is a simplified implementation
	
	// Calculate time span
	first := s.history[0]
	last := s.history[len(s.history)-1]
	timeSpan := last.timestamp.Sub(first.timestamp).Seconds()
	
	if timeSpan < 1.0 {
		return s.frequencyEstimate
	}

	// Calculate offset change
	offsetChange := last.offset.Seconds() - first.offset.Seconds()
	
	// Frequency in PPM (parts per million)
	frequency := (offsetChange / timeSpan) * 1e6
	
	// Apply exponential smoothing
	alpha := 0.1 // Smoothing factor
	s.frequencyEstimate = alpha*frequency + (1-alpha)*s.frequencyEstimate
	
	// Clamp frequency to reasonable bounds
	maxFreq := 500.0 // 500 PPM max
	if s.frequencyEstimate > maxFreq {
		s.frequencyEstimate = maxFreq
	} else if s.frequencyEstimate < -maxFreq {
		s.frequencyEstimate = -maxFreq
	}

	return s.frequencyEstimate
}