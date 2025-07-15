package clock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/shiwatime/shiwatime/pkg/steering"
	"github.com/shiwatime/shiwatime/pkg/types"
	"github.com/sirupsen/logrus"
)

// SourceFactory is a function that creates time sources
type SourceFactory func(cfg config.ClockSource, priority int) (TimeSource, error)

// Manager manages all time sources and clock synchronization
type Manager struct {
	config          *config.Config
	primarySources  []TimeSource
	secondarySources []TimeSource
	activeSources   []TimeSource
	steerer         steering.ClockSteerer
	sourceFactory   SourceFactory
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	logger          *logrus.Entry
	adjustClock     bool
	stepLimit       time.Duration
}

// NewManager creates a new clock manager
func NewManager(cfg *config.Config, factory SourceFactory) (*Manager, error) {
	stepLimit, err := time.ParseDuration(cfg.ShiwaTime.ClockSync.StepLimit)
	if err != nil {
		return nil, fmt.Errorf("invalid step limit: %w", err)
	}

	// Create steering algorithm
	steerer, err := steering.NewSteerer(cfg.ShiwaTime.Advanced.Steering)
	if err != nil {
		return nil, fmt.Errorf("failed to create steerer: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		config:        cfg,
		steerer:       steerer,
		sourceFactory: factory,
		ctx:           ctx,
		cancel:        cancel,
		adjustClock:   cfg.ShiwaTime.ClockSync.AdjustClock,
		stepLimit:     stepLimit,
		logger: logrus.WithFields(logrus.Fields{
			"component": "clock-manager",
		}),
	}, nil
}

// Start starts the clock manager
func (m *Manager) Start() error {
	m.logger.Info("Starting clock manager")

	// Create primary sources
	for i, srcCfg := range m.config.ShiwaTime.ClockSync.PrimaryClocks {
		if srcCfg.Disable {
			continue
		}

		source, err := m.sourceFactory(srcCfg, i)
		if err != nil {
			m.logger.WithError(err).Errorf("Failed to create primary source %d", i)
			continue
		}

		m.primarySources = append(m.primarySources, source)
	}

	// Create secondary sources
	for i, srcCfg := range m.config.ShiwaTime.ClockSync.SecondaryClocks {
		if srcCfg.Disable {
			continue
		}

		source, err := m.sourceFactory(srcCfg, i + 100) // Secondary sources have lower priority
		if err != nil {
			m.logger.WithError(err).Errorf("Failed to create secondary source %d", i)
			continue
		}

		m.secondarySources = append(m.secondarySources, source)
	}

	// Start primary sources
	for _, source := range m.primarySources {
		if err := source.Start(m.ctx); err != nil {
			m.logger.WithError(err).Errorf("Failed to start %s source", source.GetProtocol())
		}
	}

	// Start main synchronization loop
	m.wg.Add(1)
	go m.syncLoop()

	// Start source monitoring loop
	m.wg.Add(1)
	go m.monitorSources()

	return nil
}

// Stop stops the clock manager
func (m *Manager) Stop() error {
	m.logger.Info("Stopping clock manager")

	// Cancel context
	m.cancel()

	// Stop all sources
	m.mu.Lock()
	allSources := append(m.primarySources, m.secondarySources...)
	m.mu.Unlock()

	for _, source := range allSources {
		if err := source.Stop(); err != nil {
			m.logger.WithError(err).Errorf("Failed to stop %s source", source.GetProtocol())
		}
	}

	// Wait for goroutines to finish
	m.wg.Wait()

	return nil
}

// syncLoop is the main synchronization loop
func (m *Manager) syncLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.performSync()
		}
	}
}

// performSync performs one synchronization cycle
func (m *Manager) performSync() {
	// Get samples from all active sources
	samples := m.collectSamples()
	
	if len(samples) == 0 {
		m.logger.Debug("No samples available")
		return
	}

	// Select best samples
	bestSamples := m.selectBestSamples(samples)

	// Calculate clock adjustment
	adjustment, err := m.steerer.CalculateAdjustment(bestSamples)
	if err != nil {
		m.logger.WithError(err).Error("Failed to calculate adjustment")
		return
	}

	// Apply adjustment if enabled
	if m.adjustClock {
		if err := m.applyAdjustment(adjustment); err != nil {
			m.logger.WithError(err).Error("Failed to apply adjustment")
			return
		}
	}

	m.logger.WithFields(logrus.Fields{
		"offset":     adjustment.Offset,
		"frequency":  adjustment.Frequency,
		"samples":    len(samples),
		"best":       len(bestSamples),
	}).Debug("Sync cycle completed")
}

// collectSamples collects samples from all active sources
func (m *Manager) collectSamples() []*types.TimeSample {
	m.mu.RLock()
	sources := m.activeSources
	m.mu.RUnlock()

	var samples []*types.TimeSample
	for _, source := range sources {
		sample, err := source.GetSample()
		if err != nil {
			continue
		}

		if sample.Valid {
			samples = append(samples, sample)
		}
	}

	return samples
}

// selectBestSamples selects the best samples for steering
func (m *Manager) selectBestSamples(samples []*types.TimeSample) []*types.TimeSample {
	// Sort samples by quality
	// This is a simplified version - you might want to implement
	// more sophisticated selection algorithms
	
	if len(samples) <= 3 {
		return samples
	}

	// Return top 3 samples by quality
	// In real implementation, you'd sort properly
	return samples[:3]
}

// applyAdjustment applies the clock adjustment
func (m *Manager) applyAdjustment(adjustment *steering.ClockAdjustment) error {
	// Check step limit
	if adjustment.Offset.Abs() > m.stepLimit {
		return fmt.Errorf("offset %v exceeds step limit %v", adjustment.Offset, m.stepLimit)
	}

	// Apply the adjustment
	// This is where you'd interface with the system clock
	// For now, we'll just log it
	m.logger.WithFields(logrus.Fields{
		"offset":    adjustment.Offset,
		"frequency": adjustment.Frequency,
		"step":      adjustment.Step,
	}).Info("Applying clock adjustment")

	// TODO: Implement actual clock adjustment
	// This would involve system calls to adjust the clock

	return nil
}

// monitorSources monitors source availability and switches between primary/secondary
func (m *Manager) monitorSources() {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateActiveSources()
		}
	}
}

// updateActiveSources updates the list of active sources
func (m *Manager) updateActiveSources() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check primary sources
	var availablePrimary []TimeSource
	for _, source := range m.primarySources {
		if source.IsAvailable() {
			availablePrimary = append(availablePrimary, source)
		}
	}

	// If we have available primary sources, use them
	if len(availablePrimary) > 0 {
		m.activeSources = availablePrimary
		
		// Stop secondary sources if running
		for _, source := range m.secondarySources {
			if source.GetStatus().State != StateStopped {
				source.Stop()
			}
		}
		return
	}

	// No primary sources available, start secondary sources
	m.logger.Warn("No primary sources available, switching to secondary")
	
	var availableSecondary []TimeSource
	for _, source := range m.secondarySources {
		// Start secondary source if not already running
		if source.GetStatus().State == StateStopped || source.GetStatus().State == StateUnknown {
			if err := source.Start(m.ctx); err != nil {
				m.logger.WithError(err).Errorf("Failed to start secondary %s source", source.GetProtocol())
				continue
			}
		}
		
		if source.IsAvailable() {
			availableSecondary = append(availableSecondary, source)
		}
	}

	m.activeSources = availableSecondary
}

// GetStatus returns the current status of the clock manager
func (m *Manager) GetStatus() ManagerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := ManagerStatus{
		ActiveSources: len(m.activeSources),
		TotalPrimary:  len(m.primarySources),
		TotalSecondary: len(m.secondarySources),
	}

	// Collect source statuses
	for _, source := range m.activeSources {
		status.Sources = append(status.Sources, SourceInfo{
			Protocol: source.GetProtocol(),
			Status:   source.GetStatus(),
			Priority: source.GetPriority(),
		})
	}

	return status
}

// ManagerStatus represents the status of the clock manager
type ManagerStatus struct {
	ActiveSources  int
	TotalPrimary   int
	TotalSecondary int
	Sources        []SourceInfo
}

// SourceInfo contains information about a time source
type SourceInfo struct {
	Protocol string
	Status   SourceStatus
	Priority int
}