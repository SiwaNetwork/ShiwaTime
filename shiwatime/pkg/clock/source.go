package clock

import (
	"context"
	"time"
	
	"github.com/shiwatime/shiwatime/pkg/types"
)

// TimeSource represents a source of time synchronization
type TimeSource interface {
	// Start starts the time source
	Start(ctx context.Context) error
	
	// Stop stops the time source
	Stop() error
	
	// GetSample returns the current time sample
	GetSample() (*types.TimeSample, error)
	
	// GetStatus returns the current status of the source
	GetStatus() SourceStatus
	
	// GetProtocol returns the protocol name
	GetProtocol() string
	
	// IsAvailable returns true if the source is available
	IsAvailable() bool
	
	// GetPriority returns the priority of this source
	GetPriority() int
}

// SourceStatus represents the status of a time source
type SourceStatus struct {
	// State is the current state of the source
	State SourceState
	
	// LastSync is the time of the last successful sync
	LastSync time.Time
	
	// SyncCount is the number of successful syncs
	SyncCount uint64
	
	// ErrorCount is the number of errors
	ErrorCount uint64
	
	// LastError is the last error message
	LastError string
	
	// Stratum is the stratum level of this source
	Stratum int
	
	// RootDelay is the total delay to the root source
	RootDelay time.Duration
	
	// RootDispersion is the total dispersion to the root source
	RootDispersion time.Duration
}

// SourceState represents the state of a time source
type SourceState int

const (
	// StateUnknown is the initial state
	StateUnknown SourceState = iota
	
	// StateInitializing means the source is starting up
	StateInitializing
	
	// StateSyncing means the source is actively syncing
	StateSyncing
	
	// StateSynchronized means the source is synchronized
	StateSynchronized
	
	// StateHoldover means the source is in holdover mode
	StateHoldover
	
	// StateError means the source has an error
	StateError
	
	// StateStopped means the source is stopped
	StateStopped
)

// String returns the string representation of the state
func (s SourceState) String() string {
	switch s {
	case StateUnknown:
		return "unknown"
	case StateInitializing:
		return "initializing"
	case StateSyncing:
		return "syncing"
	case StateSynchronized:
		return "synchronized"
	case StateHoldover:
		return "holdover"
	case StateError:
		return "error"
	case StateStopped:
		return "stopped"
	default:
		return "invalid"
	}
}

// BaseTimeSource provides common functionality for all time sources
type BaseTimeSource struct {
	protocol string
	priority int
	status   SourceStatus
}

// NewBaseTimeSource creates a new base time source
func NewBaseTimeSource(protocol string, priority int) *BaseTimeSource {
	return &BaseTimeSource{
		protocol: protocol,
		priority: priority,
		status: SourceStatus{
			State: StateUnknown,
		},
	}
}

// GetProtocol returns the protocol name
func (b *BaseTimeSource) GetProtocol() string {
	return b.protocol
}

// GetPriority returns the priority
func (b *BaseTimeSource) GetPriority() int {
	return b.priority
}

// GetStatus returns the current status
func (b *BaseTimeSource) GetStatus() SourceStatus {
	return b.status
}

// SetState sets the current state
func (b *BaseTimeSource) SetState(state SourceState) {
	b.status.State = state
}

// IncrementSyncCount increments the sync count
func (b *BaseTimeSource) IncrementSyncCount() {
	b.status.SyncCount++
	b.status.LastSync = time.Now()
}

// IncrementErrorCount increments the error count and sets the last error
func (b *BaseTimeSource) IncrementErrorCount(err string) {
	b.status.ErrorCount++
	b.status.LastError = err
}