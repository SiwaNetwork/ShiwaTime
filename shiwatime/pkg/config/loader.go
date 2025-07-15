package config

import (
	"fmt"
	"os"
	"time"
	
	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	config := &Config{}
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Set defaults
	setDefaults(config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	// Clock sync defaults
	if config.ShiwaTime.ClockSync.AdjustClock == false {
		config.ShiwaTime.ClockSync.AdjustClock = true
	}
	
	if config.ShiwaTime.ClockSync.StepLimit == "" {
		config.ShiwaTime.ClockSync.StepLimit = "15m"
	}

	// Advanced defaults
	if config.ShiwaTime.Advanced.Steering.Algo == "" {
		config.ShiwaTime.Advanced.Steering.Algo = "sigma"
	}
	
	if config.ShiwaTime.Advanced.Steering.OutlierFilterType == "" {
		config.ShiwaTime.Advanced.Steering.OutlierFilterType = "strict"
	}
	
	if config.ShiwaTime.Advanced.InterferenceMonitor.BackoffTimer == "" {
		config.ShiwaTime.Advanced.InterferenceMonitor.BackoffTimer = "5m"
	}

	// Linux specific defaults
	if config.ShiwaTime.Advanced.LinuxSpecific.HardwareTimestamping == false {
		config.ShiwaTime.Advanced.LinuxSpecific.HardwareTimestamping = true
	}
	
	if config.ShiwaTime.Advanced.LinuxSpecific.TAIOffset == "" {
		config.ShiwaTime.Advanced.LinuxSpecific.TAIOffset = "auto"
	}

	// PTP tuning defaults
	if config.ShiwaTime.Advanced.PTPTuning.RelaxDelayRequests == false {
		config.ShiwaTime.Advanced.PTPTuning.RelaxDelayRequests = true
	}
	
	if config.ShiwaTime.Advanced.PTPTuning.MulticastTTL == 0 {
		config.ShiwaTime.Advanced.PTPTuning.MulticastTTL = 1
	}
	
	if config.ShiwaTime.Advanced.PTPTuning.PTPStandard == "" {
		config.ShiwaTime.Advanced.PTPTuning.PTPStandard = "1588-2008"
	}

	// Clock quality defaults
	if config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Auto == false {
		config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Auto = true
	}
	
	if config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Class == 0 {
		config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Class = 248
	}
	
	if config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Accuracy == "" {
		config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Accuracy = "0x23"
	}
	
	if config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Variance == "" {
		config.ShiwaTime.Advanced.PTPTuning.ClockQuality.Variance = "0xFFFF"
	}
	
	if config.ShiwaTime.Advanced.PTPTuning.ClockQuality.TimeSource == "" {
		config.ShiwaTime.Advanced.PTPTuning.ClockQuality.TimeSource = "0xA0"
	}

	// Elasticsearch defaults
	if len(config.Output.Elasticsearch.Hosts) == 0 {
		config.Output.Elasticsearch.Hosts = []string{"localhost:9200"}
	}
	
	if config.Output.Elasticsearch.Protocol == "" {
		config.Output.Elasticsearch.Protocol = "http"
	}

	// Setup defaults
	if config.Setup.ILM.PolicyName == "" {
		config.Setup.ILM.PolicyName = "shiwatime"
	}
	
	if config.Setup.ILM.RolloverAlias == "" {
		config.Setup.ILM.RolloverAlias = "shiwatime"
	}
	
	config.Setup.ILM.CheckExists = true

	// Logging defaults
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	
	config.Logging.ToFiles = true
	
	if config.Logging.Files.Path == "" {
		config.Logging.Files.Path = "/var/log/shiwatime"
	}
	
	if config.Logging.Files.Name == "" {
		config.Logging.Files.Name = "shiwatime"
	}
	
	if config.Logging.Files.RotateEveryBytes == 0 {
		config.Logging.Files.RotateEveryBytes = 10485760 // 10MB
	}
	
	if config.Logging.Files.KeepFiles == 0 {
		config.Logging.Files.KeepFiles = 7
	}
	
	if config.Logging.Files.Permissions == "" {
		config.Logging.Files.Permissions = "0600"
	}

	// Monitoring defaults
	config.Monitoring.Enabled = true
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	// Validate step limit
	if _, err := parseDuration(config.ShiwaTime.ClockSync.StepLimit); err != nil {
		return fmt.Errorf("invalid step_limit: %w", err)
	}

	// Validate clock sources
	for i, clock := range config.ShiwaTime.ClockSync.PrimaryClocks {
		if err := validateClockSource(clock, fmt.Sprintf("primary_clocks[%d]", i)); err != nil {
			return err
		}
	}
	
	for i, clock := range config.ShiwaTime.ClockSync.SecondaryClocks {
		if err := validateClockSource(clock, fmt.Sprintf("secondary_clocks[%d]", i)); err != nil {
			return err
		}
	}

	// Validate steering algorithm
	validAlgos := map[string]bool{
		"alpha": true,
		"beta":  true,
		"gamma": true,
		"rho":   true,
		"sigma": true,
	}
	
	if !validAlgos[config.ShiwaTime.Advanced.Steering.Algo] {
		return fmt.Errorf("invalid steering algorithm: %s", config.ShiwaTime.Advanced.Steering.Algo)
	}

	// Validate outlier filter type
	validFilters := map[string]bool{
		"strict":   true,
		"moderate": true,
		"relaxed":  true,
	}
	
	if !validFilters[config.ShiwaTime.Advanced.Steering.OutlierFilterType] {
		return fmt.Errorf("invalid outlier filter type: %s", config.ShiwaTime.Advanced.Steering.OutlierFilterType)
	}

	// Validate PTP standard
	validStandards := map[string]bool{
		"1588-2008": true,
		"1588-2019": true,
	}
	
	if !validStandards[config.ShiwaTime.Advanced.PTPTuning.PTPStandard] {
		return fmt.Errorf("invalid PTP standard: %s", config.ShiwaTime.Advanced.PTPTuning.PTPStandard)
	}

	return nil
}

// validateClockSource validates a clock source configuration
func validateClockSource(clock ClockSource, path string) error {
	// Skip disabled clocks
	if clock.Disable {
		return nil
	}

	// Validate protocol
	validProtocols := map[string]bool{
		"ptp":                        true,
		"ntp":                        true,
		"pps":                        true,
		"nmea":                       true,
		"phc":                        true,
		"timebeat_opentimecard":      true,
		"timebeat_opentimecard_mini": true,
		"ocp_timecard":               true,
		"fallback":                   true,
		"oscillator":                 true,
	}
	
	if !validProtocols[clock.Protocol] {
		return fmt.Errorf("%s: invalid protocol: %s", path, clock.Protocol)
	}

	// Protocol-specific validation
	switch clock.Protocol {
	case "ptp":
		if clock.Interface == "" && !clock.ServerOnly {
			return fmt.Errorf("%s: PTP requires interface", path)
		}
		
		if clock.DelayStrategy != "" && clock.DelayStrategy != "e2e" && clock.DelayStrategy != "p2p" {
			return fmt.Errorf("%s: invalid delay strategy: %s", path, clock.DelayStrategy)
		}
	
	case "ntp":
		if clock.IP == "" {
			return fmt.Errorf("%s: NTP requires IP", path)
		}
		
		if clock.PollInterval != "" {
			if _, err := parseDuration(clock.PollInterval); err != nil {
				return fmt.Errorf("%s: invalid poll interval: %w", path, err)
			}
		}
	
	case "pps":
		if clock.Interface == "" {
			return fmt.Errorf("%s: PPS requires interface", path)
		}
		
		if clock.EdgeMode != "" && clock.EdgeMode != "rising" && clock.EdgeMode != "falling" && clock.EdgeMode != "both" {
			return fmt.Errorf("%s: invalid edge mode: %s", path, clock.EdgeMode)
		}
	
	case "nmea":
		if clock.Device == "" {
			return fmt.Errorf("%s: NMEA requires device", path)
		}
		
		if clock.Baud == 0 {
			clock.Baud = 9600
		}
	
	case "phc":
		if clock.Device == "" {
			return fmt.Errorf("%s: PHC requires device", path)
		}
	}

	return nil
}

// parseDuration parses a duration string supporting various formats
func parseDuration(s string) (time.Duration, error) {
	// Handle empty string
	if s == "" {
		return 0, nil
	}

	// Try standard Go duration parsing first
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Handle custom formats like "15m", "1h", "1d"
	// This is a simplified version, you might want to expand this
	return time.ParseDuration(s)
}