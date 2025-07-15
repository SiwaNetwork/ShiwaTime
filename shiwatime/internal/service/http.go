package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/sirupsen/logrus"
)

// HTTPServer provides HTTP API for ShiwaTime
type HTTPServer struct {
	config  config.HTTPConfig
	service *Service
	server  *http.Server
	logger  *logrus.Entry
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(cfg config.HTTPConfig, svc *Service) *HTTPServer {
	return &HTTPServer{
		config:  cfg,
		service: svc,
		logger: logrus.WithFields(logrus.Fields{
			"component": "http-server",
		}),
	}
}

// Start starts the HTTP server
func (h *HTTPServer) Start() error {
	mux := http.NewServeMux()
	
	// Register handlers
	mux.HandleFunc("/", h.handleRoot)
	mux.HandleFunc("/status", h.handleStatus)
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/metrics", h.handleMetrics)

	addr := fmt.Sprintf("%s:%d", h.config.BindHost, h.config.BindPort)
	h.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	h.logger.WithField("address", addr).Info("Starting HTTP server")

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.WithError(err).Error("HTTP server error")
		}
	}()

	return nil
}

// Stop stops the HTTP server
func (h *HTTPServer) Stop() error {
	if h.server == nil {
		return nil
	}

	h.logger.Info("Stopping HTTP server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return h.server.Shutdown(ctx)
}

// handleRoot handles the root endpoint
func (h *HTTPServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "ShiwaTime v%s\n", Version)
	fmt.Fprintf(w, "Build: %s\n", BuildTime)
	fmt.Fprintf(w, "Commit: %s\n", GitCommit)
}

// handleStatus handles the status endpoint
func (h *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := h.service.GetStatus()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleHealth handles the health check endpoint
func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := h.service.GetStatus()
	
	if !status.Running {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "Service not running\n")
		return
	}

	// Check if we have active time sources
	if status.ClockStatus.ActiveSources == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "No active time sources\n")
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK\n")
}

// handleMetrics handles the metrics endpoint
func (h *HTTPServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement Prometheus metrics format
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "# Metrics endpoint not yet implemented\n")
}