package service

import (
	"fmt"

	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/sirupsen/logrus"
)

// CLIServer provides SSH CLI interface for ShiwaTime
type CLIServer struct {
	config  config.CLIConfig
	service *Service
	logger  *logrus.Entry
}

// NewCLIServer creates a new CLI server
func NewCLIServer(cfg config.CLIConfig, svc *Service) *CLIServer {
	return &CLIServer{
		config:  cfg,
		service: svc,
		logger: logrus.WithFields(logrus.Fields{
			"component": "cli-server",
		}),
	}
}

// Start starts the CLI server
func (c *CLIServer) Start() error {
	// TODO: Implement SSH CLI server
	addr := fmt.Sprintf("%s:%d", c.config.BindHost, c.config.BindPort)
	c.logger.WithField("address", addr).Info("CLI server would start here (not implemented)")
	return nil
}

// Stop stops the CLI server
func (c *CLIServer) Stop() error {
	c.logger.Info("Stopping CLI server")
	return nil
}