# ShiwaTime Makefile

# Variables
BINARY_NAME=shiwatime
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Directories
BUILD_DIR=build
CONFIG_DIR=/etc/shiwatime
LOG_DIR=/var/log/shiwatime

.PHONY: all build clean test run install uninstall help

# Default target
all: build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/shiwatime/main.go

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## run: Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	sudo $(BUILD_DIR)/$(BINARY_NAME) -c configs/shiwatime.yml

## install: Install the application
install: build
	@echo "Installing $(BINARY_NAME)..."
	sudo mkdir -p $(CONFIG_DIR)
	sudo mkdir -p $(LOG_DIR)
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo cp configs/shiwatime.yml $(CONFIG_DIR)/shiwatime.yml.example
	@echo "Installation complete!"
	@echo "Edit $(CONFIG_DIR)/shiwatime.yml before starting the service"

## uninstall: Uninstall the application
uninstall:
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Configuration and logs are preserved in:"
	@echo "  - $(CONFIG_DIR)"
	@echo "  - $(LOG_DIR)"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	$(GOMOD) tidy

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## lint: Run linters
lint: fmt vet
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

## test-config: Test configuration file
test-config: build
	@echo "Testing configuration..."
	$(BUILD_DIR)/$(BINARY_NAME) -test -c configs/shiwatime.yml

## dev: Development build (with race detector)
dev:
	@echo "Building development version with race detector..."
	$(GOBUILD) -race $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-dev cmd/shiwatime/main.go

## systemd: Generate systemd service file
systemd:
	@echo "Generating systemd service file..."
	@echo "[Unit]" > shiwatime.service
	@echo "Description=ShiwaTime - Time Synchronization Service" >> shiwatime.service
	@echo "After=network-online.target" >> shiwatime.service
	@echo "Wants=network-online.target" >> shiwatime.service
	@echo "" >> shiwatime.service
	@echo "[Service]" >> shiwatime.service
	@echo "Type=simple" >> shiwatime.service
	@echo "User=root" >> shiwatime.service
	@echo "ExecStart=/usr/local/bin/shiwatime -c /etc/shiwatime/shiwatime.yml" >> shiwatime.service
	@echo "Restart=always" >> shiwatime.service
	@echo "RestartSec=10" >> shiwatime.service
	@echo "" >> shiwatime.service
	@echo "[Install]" >> shiwatime.service
	@echo "WantedBy=multi-user.target" >> shiwatime.service
	@echo "Systemd service file created: shiwatime.service"

## help: Show this help
help:
	@echo "ShiwaTime Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)