package protocols

import (
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// NewPTPHandler создает новый PTP обработчик (заглушка)
func NewPTPHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	logger.Warn("PTP protocol not yet implemented, using mock handler")
	return NewMockHandler(config, logger), nil
}

// NewPPSHandler создает новый PPS обработчик (заглушка)
func NewPPSHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	logger.Warn("PPS protocol not yet implemented, using mock handler")
	return NewMockHandler(config, logger), nil
}

// NewNMEAHandler создает новый NMEA обработчик (заглушка)
func NewNMEAHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	logger.Warn("NMEA protocol not yet implemented, using mock handler")
	return NewMockHandler(config, logger), nil
}

// NewPHCHandler создает новый PHC обработчик (заглушка)
func NewPHCHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	logger.Warn("PHC protocol not yet implemented, using mock handler")
	return NewMockHandler(config, logger), nil
}