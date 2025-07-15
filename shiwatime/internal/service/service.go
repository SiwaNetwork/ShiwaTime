package service

import (
	"fmt"
	"sync"

	"github.com/shiwatime/shiwatime/pkg/clock"
	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/shiwatime/shiwatime/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// Service represents the main ShiwaTime service
type Service struct {
	config       *config.Config
	clockManager *clock.Manager
	metricsReporter *metrics.Reporter
	httpServer   *HTTPServer
	cliServer    *CLIServer
	mu           sync.Mutex
	running      bool
	logger       *logrus.Entry
}

// New creates a new service instance
func New(cfg *config.Config) (*Service, error) {
	// Create clock manager
	clockManager, err := clock.NewManager(cfg, createTimeSource)
	if err != nil {
		return nil, fmt.Errorf("failed to create clock manager: %w", err)
	}

	// Create metrics reporter
	metricsReporter, err := metrics.NewReporter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics reporter: %w", err)
	}

	svc := &Service{
		config:          cfg,
		clockManager:    clockManager,
		metricsReporter: metricsReporter,
		logger: logrus.WithFields(logrus.Fields{
			"component": "service",
		}),
	}

	// Create HTTP server if enabled
	if cfg.ShiwaTime.Advanced.HTTP.Enable {
		svc.httpServer = NewHTTPServer(cfg.ShiwaTime.Advanced.HTTP, svc)
	}

	// Create CLI server if enabled
	if cfg.ShiwaTime.Advanced.CLI.Enable {
		svc.cliServer = NewCLIServer(cfg.ShiwaTime.Advanced.CLI, svc)
	}

	return svc, nil
}

// Start starts the service
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("service already running")
	}

	s.logger.Info("Starting service components")

	// Start clock manager
	if err := s.clockManager.Start(); err != nil {
		return fmt.Errorf("failed to start clock manager: %w", err)
	}

	// Start metrics reporter
	if err := s.metricsReporter.Start(); err != nil {
		return fmt.Errorf("failed to start metrics reporter: %w", err)
	}

	// Start HTTP server if enabled
	if s.httpServer != nil {
		if err := s.httpServer.Start(); err != nil {
			return fmt.Errorf("failed to start HTTP server: %w", err)
		}
	}

	// Start CLI server if enabled
	if s.cliServer != nil {
		if err := s.cliServer.Start(); err != nil {
			return fmt.Errorf("failed to start CLI server: %w", err)
		}
	}

	s.running = true
	s.logger.Info("All service components started successfully")

	return nil
}

// Stop stops the service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.logger.Info("Stopping service components")

	var errors []error

	// Stop CLI server
	if s.cliServer != nil {
		if err := s.cliServer.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop CLI server: %w", err))
		}
	}

	// Stop HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop HTTP server: %w", err))
		}
	}

	// Stop metrics reporter
	if err := s.metricsReporter.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop metrics reporter: %w", err))
	}

	// Stop clock manager
	if err := s.clockManager.Stop(); err != nil {
		errors = append(errors, fmt.Errorf("failed to stop clock manager: %w", err))
	}

	s.running = false

	if len(errors) > 0 {
		return fmt.Errorf("errors stopping service: %v", errors)
	}

	s.logger.Info("All service components stopped successfully")
	return nil
}

// GetStatus returns the service status
func (s *Service) GetStatus() ServiceStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return ServiceStatus{
		Running:      s.running,
		Version:      Version,
		ClockStatus:  s.clockManager.GetStatus(),
		MetricsStatus: s.metricsReporter.GetStatus(),
	}
}

// ServiceStatus represents the service status
type ServiceStatus struct {
	Running       bool
	Version       string
	ClockStatus   clock.ManagerStatus
	MetricsStatus metrics.ReporterStatus
}

// Version information (should be set by build)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)