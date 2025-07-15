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