package types

import (
	"time"
)

// TimeSample represents a time measurement from a source
type TimeSample struct {
	// LocalTime is the local system time when the sample was taken
	LocalTime time.Time
	
	// SourceTime is the time from the source
	SourceTime time.Time
	
	// Offset is the difference between source and local time
	Offset time.Duration
	
	// Delay is the round-trip delay to the source
	Delay time.Duration
	
	// Error is the estimated error of this sample
	Error time.Duration
	
	// Valid indicates if this sample is valid
	Valid bool
	
	// Source identifies the source of this sample
	Source string
	
	// Quality indicates the quality of this sample (0-100)
	Quality int
}