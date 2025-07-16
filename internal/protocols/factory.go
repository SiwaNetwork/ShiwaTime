package protocols

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// NewTimeSourceHandler создает обработчик источника времени
func NewTimeSourceHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	logger.WithFields(logrus.Fields{
		"type":      config.Type,
		"host":      config.Host,
		"interface": config.Interface,
		"device":    config.Device,
	}).Info("Creating time source handler")

	switch strings.ToLower(config.Type) {
	case "ntp":
		return NewNTPHandler(config, logger)
	case "ptp":
		return NewPTPHandler(config, logger)
	case "pps":
		return NewPPSHandler(config, logger)
	case "phc":
		return NewPHCHandler(config, logger)
	case "nmea":
		return NewNMEAHandler(config, logger)
	case "timecard":
		return NewTimecardHandler(config, logger)
	case "mock":
		return NewMockHandler(config, logger)
	default:
		return nil, fmt.Errorf("unknown time source type: %s", config.Type)
	}
}

// GetSupportedProtocols возвращает список поддерживаемых протоколов
func GetSupportedProtocols() []string {
	return []string{
		"ntp",
		"ptp", 
		"pps",
		"phc",
		"nmea",
		"timecard",
		"mock",
	}
}

// IsProtocolSupported проверяет, поддерживается ли протокол
func IsProtocolSupported(protocol string) bool {
	for _, supported := range GetSupportedProtocols() {
		if strings.EqualFold(protocol, supported) {
			return true
		}
	}
	return false
}

// GetProtocolDescription возвращает описание протокола
func GetProtocolDescription(protocol string) string {
	switch strings.ToLower(protocol) {
	case "ntp":
		return "Network Time Protocol - синхронизация времени через сеть"
	case "ptp":
		return "Precision Time Protocol (IEEE 1588) - высокоточная сетевая синхронизация"
	case "pps":
		return "Pulse Per Second - аппаратные импульсы синхронизации"
	case "phc":
		return "Precision Hardware Clock - аппаратные часы сетевых адаптеров"
	case "nmea":
		return "NMEA - синхронизация с GPS/GNSS приемников"
	case "timecard":
		return "Timecard - специализированные карты точного времени"
	case "mock":
		return "Mock - тестовый источник времени"
	default:
		return "Неизвестный протокол"
	}
}

// ValidateConfig проверяет конфигурацию источника времени
func ValidateConfig(config config.TimeSourceConfig) error {
	if config.Type == "" {
		return fmt.Errorf("type field is required")
	}

	if !IsProtocolSupported(config.Type) {
		return fmt.Errorf("unsupported protocol: %s", config.Type)
	}

	// Проверки для конкретных протоколов
	switch strings.ToLower(config.Type) {
	case "ntp":
		if config.Host == "" {
			return fmt.Errorf("host is required for NTP")
		}
		if config.Port == 0 {
			config.Port = 123 // Default NTP port
		}

	case "ptp":
		if config.Interface == "" {
			return fmt.Errorf("interface is required for PTP")
		}
		if config.Domain < 0 || config.Domain > 255 {
			return fmt.Errorf("PTP domain must be between 0 and 255")
		}

	case "pps":
		if config.Device == "" && config.GPIOPin == 0 {
			return fmt.Errorf("either device or gpio_pin is required for PPS")
		}

	case "phc":
		if config.Device == "" && config.PHCIndex == 0 && config.Interface == "" {
			return fmt.Errorf("device, phc_index or interface is required for PHC")
		}

	case "nmea":
		if config.Device == "" {
			return fmt.Errorf("device is required for NMEA")
		}
		if config.BaudRate == 0 {
			config.BaudRate = 9600 // Default baud rate
		}

	case "timecard":
		if config.Device == "" {
			return fmt.Errorf("device is required for Timecard")
		}
	}

	return nil
}

// GetDefaultConfig возвращает конфигурацию по умолчанию для протокола
func GetDefaultConfig(protocol string) config.TimeSourceConfig {
	cfg := config.TimeSourceConfig{
		Type:   protocol,
		Weight: 1,
	}

	switch strings.ToLower(protocol) {
	case "ntp":
		cfg.Port = 123
		cfg.PollingInterval = 64 // seconds
		cfg.MaxOffset = 1000    // milliseconds
		cfg.MaxDelay = 100      // milliseconds

	case "ptp":
		cfg.Domain = 0
		cfg.TransportType = "UDPv4"
		cfg.LogAnnounceInterval = 1
		cfg.LogSyncInterval = 0
		cfg.LogDelayReqInterval = 0

	case "pps":
		cfg.PPSMode = "rising"
		cfg.PPSKernel = true

	case "phc":
		cfg.PHCIndex = 0

	case "nmea":
		cfg.BaudRate = 9600
		cfg.DataBits = 8
		cfg.StopBits = 1
		cfg.Parity = "none"

	case "timecard":
		// Default timecard config
	}

	return cfg
}