#!/bin/bash

# Timebeat Unified Configuration Setup Script
# This script helps manage the unified Timebeat configuration

set -e

echo "=== Timebeat Unified Configuration Setup ==="
echo

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root" 
   exit 1
fi

# Function to backup existing config
backup_config() {
    if [ -f "/etc/timebeat/timebeat.yml" ]; then
        echo "Backing up existing configuration..."
        cp /etc/timebeat/timebeat.yml /etc/timebeat/timebeat.yml.backup.$(date +%Y%m%d_%H%M%S)
        echo "Backup created: /etc/timebeat/timebeat.yml.backup.$(date +%Y%m%d_%H%M%S)"
    fi
}

# Function to install unified config
install_unified_config() {
    echo "Installing unified configuration..."
    
    # Copy the unified config to the system
    cp timebeat-unified.yml /etc/timebeat/timebeat.yml
    
    # Set proper permissions
    chmod 644 /etc/timebeat/timebeat.yml
    chown root:root /etc/timebeat/timebeat.yml
    
    echo "Unified configuration installed successfully!"
}

# Function to validate configuration
validate_config() {
    echo "Validating configuration..."
    
    if command -v timebeat >/dev/null 2>&1; then
        if timebeat test config -c /etc/timebeat/timebeat.yml; then
            echo "Configuration validation successful!"
            return 0
        else
            echo "Configuration validation failed!"
            return 1
        fi
    else
        echo "Timebeat not found in PATH. Skipping validation."
        return 0
    fi
}

# Function to restart Timebeat service
restart_service() {
    echo "Restarting Timebeat service..."
    
    if systemctl is-active --quiet timebeat; then
        systemctl restart timebeat
        echo "Timebeat service restarted successfully!"
    else
        echo "Timebeat service is not running. You may need to start it manually."
    fi
}

# Function to show status
show_status() {
    echo "=== Current Configuration Status ==="
    
    if [ -f "/etc/timebeat/timebeat.yml" ]; then
        echo "✓ Main configuration file exists"
        
        # Check if Timebeat Timecard Mini is configured
        if grep -q "timebeat_opentimecard_mini" /etc/timebeat/timebeat.yml; then
            echo "✓ Timebeat Timecard Mini configuration found"
        else
            echo "✗ Timebeat Timecard Mini configuration not found"
        fi
        
        # Check if device exists
        if [ -e "/dev/ttyS4" ]; then
            echo "✓ Serial device /dev/ttyS4 exists"
        else
            echo "✗ Serial device /dev/ttyS4 not found"
        fi
    else
        echo "✗ Main configuration file not found"
    fi
    
    echo
    echo "=== Service Status ==="
    systemctl status timebeat --no-pager -l || echo "Timebeat service not found or not running"
}

# Function to show help
show_help() {
    echo "Usage: $0 [OPTION]"
    echo
    echo "Options:"
    echo "  install    Install the unified configuration"
    echo "  backup     Backup existing configuration"
    echo "  validate   Validate the configuration"
    echo "  restart    Restart Timebeat service"
    echo "  status     Show current status"
    echo "  help       Show this help message"
    echo
    echo "Examples:"
    echo "  $0 install    # Install unified config"
    echo "  $0 status     # Show current status"
    echo "  $0 validate   # Validate configuration"
}

# Main script logic
case "${1:-help}" in
    install)
        backup_config
        install_unified_config
        validate_config
        restart_service
        echo
        echo "=== Installation Complete ==="
        echo "Timebeat Timecard Mini configuration has been added to the unified config."
        echo "The configuration includes:"
        echo "  - Protocol: timebeat_opentimecard_mini"
        echo "  - Device: /dev/ttyS4"
        echo "  - Baud rate: 9600"
        echo "  - GNSS signals: GPS + GLONASS + Galileo"
        echo "  - Group: timecard_mini_secondary"
        echo
        ;;
    backup)
        backup_config
        ;;
    validate)
        validate_config
        ;;
    restart)
        restart_service
        ;;
    status)
        show_status
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo "Unknown option: $1"
        show_help
        exit 1
        ;;
esac