package protocols

import (
	"fmt"
	"math/rand"
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// MockHandler mock-обработчик для тестирования
type MockHandler struct {
	config  config.TimeSourceConfig
	logger  *logrus.Logger
	running bool
	status  ConnectionStatus
}

// NewMockHandler создает новый mock-обработчик
func NewMockHandler(config config.TimeSourceConfig, logger *logrus.Logger) *MockHandler {
	return &MockHandler{
		config: config,
		logger: logger,
		status: ConnectionStatus{
			Connected: true,
		},
	}
}

// Start запускает mock-обработчик
func (h *MockHandler) Start() error {
	h.logger.WithField("source", "mock").Info("Starting mock time source handler")
	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()
	return nil
}

// Stop останавливает mock-обработчик
func (h *MockHandler) Stop() error {
	h.logger.WithField("source", "mock").Info("Stopping mock time source handler")
	h.running = false
	h.status.Connected = false
	return nil
}

// GetTimeInfo возвращает mock информацию о времени
func (h *MockHandler) GetTimeInfo() (*TimeInfo, error) {
	if !h.running {
		return nil, fmt.Errorf("handler not running")
	}
	
	// Симулируем небольшое случайное смещение
	offsetNs := rand.Int63n(1000000) - 500000 // +/- 500 микросекунд
	delayNs := rand.Int63n(1000000) + 100000  // 100-1100 микросекунд
	
	info := &TimeInfo{
		Timestamp: time.Now(),
		Offset:    time.Duration(offsetNs) * time.Nanosecond,
		Delay:     time.Duration(delayNs) * time.Nanosecond,
		Quality:   200 + rand.Intn(50), // 200-250
		Stratum:   2,
		Precision: -20, // ~1 микросекунда
	}
	
	h.status.LastActivity = time.Now()
	h.status.PacketsRx++
	
	return info, nil
}

// GetStatus возвращает статус соединения
func (h *MockHandler) GetStatus() ConnectionStatus {
	return h.status
}

// GetConfig возвращает конфигурацию
func (h *MockHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}