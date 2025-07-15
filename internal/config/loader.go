package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"gopkg.in/yaml.v3"
)

// LoadConfig загружает конфигурацию из файла
func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path is required")
	}

	// Проверяем существование файла
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	// Читаем файл напрямую
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Парсим YAML напрямую
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Валидируем конфигурацию
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// Устанавливаем значения по умолчанию
	setDefaults(&config)

	return &config, nil
}

// LoadConfigFromBytes загружает конфигурацию из байтов
func LoadConfigFromBytes(data []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	setDefaults(&config)
	return &config, nil
}

// validateConfig проверяет корректность конфигурации
func validateConfig(config *Config) error {
	// Проверяем наличие источников времени
	if len(config.ShiwaTime.ClockSync.PrimaryClocks) == 0 &&
		len(config.ShiwaTime.ClockSync.SecondaryClocks) == 0 {
		return fmt.Errorf("at least one time source (primary or secondary) must be configured")
	}

	// Проверяем корректность протоколов
	for i, source := range config.ShiwaTime.ClockSync.PrimaryClocks {
		if err := validateTimeSource(source, fmt.Sprintf("primary_clocks[%d]", i)); err != nil {
			return err
		}
	}

	for i, source := range config.ShiwaTime.ClockSync.SecondaryClocks {
		if err := validateTimeSource(source, fmt.Sprintf("secondary_clocks[%d]", i)); err != nil {
			return err
		}
	}

	// Проверяем step_limit
	if config.ShiwaTime.ClockSync.StepLimit != "" {
		if _, err := parseDuration(config.ShiwaTime.ClockSync.StepLimit); err != nil {
			return fmt.Errorf("invalid step_limit format: %w", err)
		}
	}

	// Проверяем настройки CLI
	if config.ShiwaTime.CLI.Enable {
		if config.ShiwaTime.CLI.BindPort <= 0 || config.ShiwaTime.CLI.BindPort > 65535 {
			return fmt.Errorf("invalid CLI bind_port: must be between 1 and 65535")
		}
	}

	// Проверяем настройки HTTP
	if config.ShiwaTime.HTTP.Enable {
		if config.ShiwaTime.HTTP.BindPort <= 0 || config.ShiwaTime.HTTP.BindPort > 65535 {
			return fmt.Errorf("invalid HTTP bind_port: must be between 1 and 65535")
		}
	}

	return nil
}

// validateTimeSource проверяет корректность конфигурации источника времени
func validateTimeSource(source TimeSourceConfig, context string) error {
	supportedProtocols := []string{"ptp", "ntp", "pps", "nmea", "phc", "timebeat_opentimecard", "timebeat_opentimecard_mini", "ocp_timecard"}
	
	// Проверяем протокол
	if source.Protocol == "" {
		return fmt.Errorf("%s: protocol is required", context)
	}

	found := false
	for _, p := range supportedProtocols {
		if source.Protocol == p {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%s: unsupported protocol '%s', supported: %s", 
			context, source.Protocol, strings.Join(supportedProtocols, ", "))
	}

	// Протокол-специфичная валидация
	switch source.Protocol {
	case "ptp":
		if source.Domain < 0 || source.Domain > 255 {
			return fmt.Errorf("%s: PTP domain must be between 0 and 255", context)
		}
		if source.DelayStrategy != "" && source.DelayStrategy != "e2e" && source.DelayStrategy != "p2p" {
			return fmt.Errorf("%s: PTP delay_strategy must be 'e2e' or 'p2p'", context)
		}
	case "ntp":
		if source.IP == "" {
			return fmt.Errorf("%s: NTP IP address is required", context)
		}
		if source.PollInterval != "" {
			if _, err := parseDuration(source.PollInterval); err != nil {
				return fmt.Errorf("%s: invalid NTP poll interval: %w", context, err)
			}
		}
	case "nmea":
		if source.Device == "" {
			return fmt.Errorf("%s: NMEA device path is required", context)
		}
		if source.Baud <= 0 {
			return fmt.Errorf("%s: NMEA baud rate must be positive", context)
		}
	}

	return nil
}

// setDefaults устанавливает значения по умолчанию
func setDefaults(config *Config) {
	// Значения по умолчанию для ShiwaTime
	if config.ShiwaTime.ClockSync.AdjustClock == false && 
		config.ShiwaTime.ClockSync.StepLimit == "" {
		config.ShiwaTime.ClockSync.AdjustClock = true // Timebeat default
	}

	if config.ShiwaTime.ClockSync.StepLimit == "" {
		config.ShiwaTime.ClockSync.StepLimit = "15m"
	}

	// CLI значения по умолчанию
	if config.ShiwaTime.CLI.Enable && config.ShiwaTime.CLI.BindPort == 0 {
		config.ShiwaTime.CLI.BindPort = 65129
	}
	if config.ShiwaTime.CLI.Enable && config.ShiwaTime.CLI.BindHost == "" {
		config.ShiwaTime.CLI.BindHost = "127.0.0.1"
	}
	if config.ShiwaTime.CLI.Enable && config.ShiwaTime.CLI.Username == "" {
		config.ShiwaTime.CLI.Username = "admin"
	}

	// HTTP значения по умолчанию
	if config.ShiwaTime.HTTP.Enable && config.ShiwaTime.HTTP.BindPort == 0 {
		config.ShiwaTime.HTTP.BindPort = 8088
	}
	if config.ShiwaTime.HTTP.Enable && config.ShiwaTime.HTTP.BindHost == "" {
		config.ShiwaTime.HTTP.BindHost = "127.0.0.1"
	}

	// Elasticsearch значения по умолчанию
	if len(config.Output.Elasticsearch.Hosts) == 0 {
		config.Output.Elasticsearch.Hosts = []string{"localhost:9200"}
	}
	if config.Output.Elasticsearch.Protocol == "" {
		config.Output.Elasticsearch.Protocol = "http"
	}

	// ILM значения по умолчанию
	if config.Setup.ILM.PolicyName == "" {
		config.Setup.ILM.PolicyName = "shiwatime"
	}
	config.Setup.ILM.Enabled = true
	config.Setup.ILM.CheckExists = true

	// Установка значений по умолчанию для источников времени
	for i := range config.ShiwaTime.ClockSync.PrimaryClocks {
		setTimeSourceDefaults(&config.ShiwaTime.ClockSync.PrimaryClocks[i])
	}
	for i := range config.ShiwaTime.ClockSync.SecondaryClocks {
		setTimeSourceDefaults(&config.ShiwaTime.ClockSync.SecondaryClocks[i])
	}

	// PTP Tuning значения по умолчанию
	if config.ShiwaTime.PTPTuning.PTPStandard == "" {
		config.ShiwaTime.PTPTuning.PTPStandard = "1588-2008"
	}
	if config.ShiwaTime.PTPTuning.PHC.TAIOffset == "" {
		config.ShiwaTime.PTPTuning.PHC.TAIOffset = "auto"
	}
}

// setTimeSourceDefaults устанавливает значения по умолчанию для источника времени
func setTimeSourceDefaults(source *TimeSourceConfig) {
	switch source.Protocol {
	case "ptp":
		if source.AnnounceInterval == 0 {
			source.AnnounceInterval = 1
		}
		if source.Priority1 == 0 {
			source.Priority1 = 128
		}
		if source.Priority2 == 0 {
			source.Priority2 = 128
		}
		if source.DelayStrategy == "" {
			source.DelayStrategy = "e2e"
		}
	case "ntp":
		if source.PollInterval == "" {
			source.PollInterval = "4s"
		}
	case "nmea":
		if source.Baud == 0 {
			source.Baud = 9600
		}
	}
}

// parseDuration парсит строку длительности
func parseDuration(s string) (time.Duration, error) {
	// Поддерживаем различные форматы: 15m, 30s, 1h, 2d
	if strings.HasSuffix(s, "d") {
		// Обрабатываем дни
		days := strings.TrimSuffix(s, "d")
		d, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, err
		}
		return d * 24, nil
	}
	return time.ParseDuration(s)
}

// SaveConfig сохраняет конфигурацию в файл
func SaveConfig(config *Config, path string) error {
	// Создаем директорию если не существует
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Сериализуем в YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Записываем файл
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}