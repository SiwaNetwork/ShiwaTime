package steering

import (
	"math"
	"sort"

	"github.com/shiwatime/shiwatime/pkg/types"
)

// OutlierFilter filters outlier samples
type OutlierFilter interface {
	Filter(samples []*types.TimeSample) []*types.TimeSample
}

// NewOutlierFilter creates a new outlier filter based on type
func NewOutlierFilter(filterType string) OutlierFilter {
	switch filterType {
	case "strict":
		return &StrictOutlierFilter{}
	case "moderate":
		return &ModerateOutlierFilter{}
	case "relaxed":
		return &RelaxedOutlierFilter{}
	default:
		return &StrictOutlierFilter{}
	}
}

// StrictOutlierFilter implements strict outlier filtering
type StrictOutlierFilter struct{}

// Filter filters samples using strict criteria
func (f *StrictOutlierFilter) Filter(samples []*types.TimeSample) []*types.TimeSample {
	if len(samples) < 3 {
		return samples
	}

	// Calculate median offset
	offsets := make([]float64, len(samples))
	for i, s := range samples {
		offsets[i] = s.Offset.Seconds()
	}
	
	median := calculateMedian(offsets)
	
	// Calculate MAD (Median Absolute Deviation)
	mad := calculateMAD(offsets, median)
	
	// Filter samples outside 3 MAD from median
	threshold := 3.0 * mad
	if threshold < 1e-6 {
		threshold = 1e-6 // Minimum threshold
	}
	
	var filtered []*types.TimeSample
	for _, sample := range samples {
		deviation := math.Abs(sample.Offset.Seconds() - median)
		if deviation <= threshold {
			filtered = append(filtered, sample)
		}
	}
	
	// Keep at least 1 sample
	if len(filtered) == 0 && len(samples) > 0 {
		return samples[:1]
	}
	
	return filtered
}

// ModerateOutlierFilter implements moderate outlier filtering
type ModerateOutlierFilter struct{}

// Filter filters samples using moderate criteria
func (f *ModerateOutlierFilter) Filter(samples []*types.TimeSample) []*types.TimeSample {
	if len(samples) < 3 {
		return samples
	}

	// Use 5 MAD threshold instead of 3
	offsets := make([]float64, len(samples))
	for i, s := range samples {
		offsets[i] = s.Offset.Seconds()
	}
	
	median := calculateMedian(offsets)
	mad := calculateMAD(offsets, median)
	
	threshold := 5.0 * mad
	if threshold < 1e-6 {
		threshold = 1e-6
	}
	
	var filtered []*types.TimeSample
	for _, sample := range samples {
		deviation := math.Abs(sample.Offset.Seconds() - median)
		if deviation <= threshold {
			filtered = append(filtered, sample)
		}
	}
	
	if len(filtered) == 0 && len(samples) > 0 {
		return samples[:1]
	}
	
	return filtered
}

// RelaxedOutlierFilter implements relaxed outlier filtering
type RelaxedOutlierFilter struct{}

// Filter filters samples using relaxed criteria
func (f *RelaxedOutlierFilter) Filter(samples []*types.TimeSample) []*types.TimeSample {
	if len(samples) < 3 {
		return samples
	}

	// Use interquartile range method
	offsets := make([]float64, len(samples))
	for i, s := range samples {
		offsets[i] = s.Offset.Seconds()
	}
	
	sort.Float64s(offsets)
	
	q1 := percentile(offsets, 25)
	q3 := percentile(offsets, 75)
	iqr := q3 - q1
	
	lowerBound := q1 - 1.5*iqr
	upperBound := q3 + 1.5*iqr
	
	var filtered []*types.TimeSample
	for _, sample := range samples {
		offset := sample.Offset.Seconds()
		if offset >= lowerBound && offset <= upperBound {
			filtered = append(filtered, sample)
		}
	}
	
	if len(filtered) == 0 && len(samples) > 0 {
		return samples[:1]
	}
	
	return filtered
}

// calculateMedian calculates the median of a slice
func calculateMedian(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)
	
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// calculateMAD calculates the Median Absolute Deviation
func calculateMAD(values []float64, median float64) float64 {
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - median)
	}
	return calculateMedian(deviations)
}

// percentile calculates the percentile value
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	
	rank := p * float64(n-1) / 100
	lower := int(rank)
	upper := lower + 1
	
	if upper >= n {
		return sorted[n-1]
	}
	
	weight := rank - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}