package steering

import (
	"time"

	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/shiwatime/shiwatime/pkg/types"
)

// ClockSteerer is the interface for clock steering algorithms
type ClockSteerer interface {
	// CalculateAdjustment calculates the clock adjustment based on time samples
	CalculateAdjustment(samples []*types.TimeSample) (*ClockAdjustment, error)
	
	// GetName returns the name of the algorithm
	GetName() string
	
	// Reset resets the algorithm state
	Reset()
}

// ClockAdjustment represents a clock adjustment to be applied
type ClockAdjustment struct {
	// Offset is the time offset to adjust
	Offset time.Duration
	
	// Frequency is the frequency adjustment in parts per million (PPM)
	Frequency float64
	
	// Step indicates if this should be a step adjustment (true) or slew (false)
	Step bool
	
	// Timestamp when this adjustment was calculated
	Timestamp time.Time
}

// NewSteerer creates a new steerer based on configuration
func NewSteerer(cfg config.SteeringConfig) (ClockSteerer, error) {
	switch cfg.Algo {
	case "sigma":
		return NewSigmaSteerer(cfg)
	case "alpha":
		return NewAlphaSteerer(cfg)
	case "beta":
		return NewBetaSteerer(cfg)
	case "gamma":
		return NewGammaSteerer(cfg)
	case "rho":
		return NewRhoSteerer(cfg)
	default:
		// Default to sigma
		return NewSigmaSteerer(cfg)
	}
}