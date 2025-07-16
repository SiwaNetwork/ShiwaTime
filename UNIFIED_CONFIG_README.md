# Timebeat Unified Configuration

This document describes the unified Timebeat configuration that includes the Timebeat Timecard Mini functionality.

## Overview

The unified configuration combines all Timebeat protocols and features into a single, comprehensive configuration file. This includes the newly added **Timebeat Timecard Mini** support.

## Configuration Files

### 1. `timebeat-unified.yml` - Main Unified Configuration
This is the complete unified configuration file that includes:
- PTP (Precision Time Protocol) configuration
- NTP (Network Time Protocol) configuration  
- PPS (Pulse Per Second) configuration
- NMEA-GNSS configuration
- PHC (PTP Hardware Clock) configuration
- **Timebeat Timecard Mini** configuration
- Elasticsearch output configuration
- Logging and monitoring settings

### 2. `timebeat-active-protocols.yml` - Active Protocols Configuration
Contains all active protocols with the Timebeat Timecard Mini enabled.

### 3. `timebeat.yml` - Original Configuration (Updated)
The original configuration file has been updated to include the Timebeat Timecard Mini configuration.

## Timebeat Timecard Mini Configuration

The Timebeat Timecard Mini has been added with the following configuration:

```yaml
- protocol:     timebeat_opentimecard_mini   # Timebeat Timecard Mini
  device:       '/dev/ttyS4'             # Serial device path
  baud:         9600                     # Serial device baud rate
  card_config:  ['gnss1:signal:gps+glonass+galileo']
  offset:       0                        # Static offset of RMC line
  atomic:       false                    # Indicate if oscillator is atomic
  monitor_only: false
  group:        timecard_mini_secondary
  logsource:    'Timebeat Timecard Mini'
```

### Configuration Parameters

- **protocol**: `timebeat_opentimecard_mini` - Specifies the Timebeat Timecard Mini protocol
- **device**: `/dev/ttyS4` - Serial device path for communication
- **baud**: `9600` - Serial communication baud rate
- **card_config**: `['gnss1:signal:gps+glonass+galileo']` - GNSS signal configuration
  - Supports GPS, GLONASS, and Galileo satellite systems
- **offset**: `0` - Static offset for RMC line (in nanoseconds)
- **atomic**: `false` - Indicates if the oscillator is atomic
- **monitor_only**: `false` - Enables clock steering (not just monitoring)
- **group**: `timecard_mini_secondary` - Groups this source with other secondary clocks
- **logsource**: `'Timebeat Timecard Mini'` - Identifies this source in logs

## Installation and Setup

### Using the Setup Script

The `setup_unified_config.sh` script provides easy management of the unified configuration:

```bash
# Install the unified configuration
sudo ./setup_unified_config.sh install

# Check current status
sudo ./setup_unified_config.sh status

# Validate configuration
sudo ./setup_unified_config.sh validate

# Restart Timebeat service
sudo ./setup_unified_config.sh restart

# Backup existing configuration
sudo ./setup_unified_config.sh backup
```

### Manual Installation

1. **Backup existing configuration**:
   ```bash
   sudo cp /etc/timebeat/timebeat.yml /etc/timebeat/timebeat.yml.backup
   ```

2. **Install unified configuration**:
   ```bash
   sudo cp timebeat-unified.yml /etc/timebeat/timebeat.yml
   sudo chmod 644 /etc/timebeat/timebeat.yml
   sudo chown root:root /etc/timebeat/timebeat.yml
   ```

3. **Validate configuration**:
   ```bash
   sudo timebeat test config -c /etc/timebeat/timebeat.yml
   ```

4. **Restart Timebeat service**:
   ```bash
   sudo systemctl restart timebeat
   ```

## Hardware Requirements

### Timebeat Timecard Mini
- **Serial Interface**: `/dev/ttyS4` (configurable)
- **Baud Rate**: 9600 (configurable)
- **GNSS Support**: GPS, GLONASS, Galileo
- **Communication**: 8N1 (8 data bits, no parity, 1 stop bit)

### System Requirements
- Linux system with serial port support
- Timebeat software installed
- Proper permissions for serial device access

## Verification

### Check Device Availability
```bash
# Check if serial device exists
ls -la /dev/ttyS4

# Check device permissions
ls -la /dev/ttyS*
```

### Check Configuration
```bash
# Verify Timebeat Timecard Mini is configured
grep -A 10 "timebeat_opentimecard_mini" /etc/timebeat/timebeat.yml

# Check Timebeat service status
systemctl status timebeat
```

### Check Logs
```bash
# View Timebeat logs
journalctl -u timebeat -f

# Check for Timecard Mini messages
journalctl -u timebeat | grep "Timebeat Timecard Mini"
```

## Troubleshooting

### Common Issues

1. **Serial device not found**:
   - Verify the device path `/dev/ttyS4` exists
   - Check if the device is properly connected
   - Ensure proper permissions (user should be in `dialout` group)

2. **Permission denied**:
   ```bash
   sudo usermod -a -G dialout $USER
   # Log out and back in, or reboot
   ```

3. **Configuration validation fails**:
   - Check YAML syntax
   - Verify all required parameters are present
   - Ensure proper indentation

4. **Service won't start**:
   - Check logs: `journalctl -u timebeat -n 50`
   - Verify configuration: `timebeat test config`
   - Check file permissions

### Debug Mode

Enable debug logging by modifying the configuration:

```yaml
logging.level: debug
```

## Configuration Groups

The unified configuration organizes time sources into groups:

- **Primary Clocks**: `ptp_primary`, `ntp_primary`, `pps_primary`
- **Secondary Clocks**: `nmea_secondary`, `phc_secondary`, `timecard_mini_secondary`, `ntp_backup`

This grouping allows for better management and monitoring of different time sources.

## Monitoring

### Elasticsearch Integration
The configuration includes Elasticsearch output for centralized monitoring:

```yaml
output.elasticsearch:
  hosts: ["localhost:9200"]
  index: "timebeat-%{+yyyy.MM.dd}"
```

### Logging
Comprehensive logging is configured:

```yaml
logging.level: info
logging.to_files: true
logging.files:
  path: /var/log/timebeat
  name: timebeat
```

## Security Considerations

- Configuration files have restricted permissions (644)
- Service runs with appropriate security settings
- Seccomp is disabled for compatibility
- Monitoring is enabled for security oversight

## Support

For issues related to:
- **Timebeat Timecard Mini**: Contact Timebeat support
- **Configuration**: Check this documentation and logs
- **Hardware**: Verify serial connections and device availability

## Version Information

- **Configuration Version**: 1.0
- **Timebeat Timecard Mini**: Added and configured
- **Last Updated**: $(date)
- **Compatible Timebeat Version**: 2.2.20+