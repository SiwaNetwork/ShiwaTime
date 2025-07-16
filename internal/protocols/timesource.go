package protocols

import (
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// TimeSourceHandlerImpl реализация интерфейса TimeSourceHandler
type TimeSourceHandlerImpl struct {
	config     config.TimeSourceConfig
	logger     *logrus.Logger
	status     ConnectionStatus
	timeInfo   *TimeInfo
	stopChan   chan struct{}
	running    bool
	mutex      sync.RWMutex
	
	// Внутренние компоненты
	ptpHandler     PTPHandler
	ntpHandler     NTPHandler
	ppsHandler     PPSHandler
	phcHandler     TimeSourceHandler
	nmeaHandler    NMEAHandler
	timecardHandler TimeSourceHandler
	mockHandler    TimeSourceHandler
}

// NewTimeSourceHandlerImpl создает новый обработчик источника времени
func NewTimeSourceHandlerImpl(cfg config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	handler := &TimeSourceHandlerImpl{
		config:   cfg,
		logger:   logger,
		stopChan: make(chan struct{}),
		status: ConnectionStatus{
			Connected:    false,
			LastActivity: time.Now(),
		},
	}

	// Инициализация в зависимости от типа протокола
	switch cfg.TimeSourceType {
	case "ptp":
		ptpHandler, err := NewPTPHandler(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create PTP handler: %w", err)
		}
		handler.ptpHandler = ptpHandler.(PTPHandler)
		
	case "ntp":
		ntpHandler, err := NewNTPHandler(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create NTP handler: %w", err)
		}
		handler.ntpHandler = ntpHandler.(NTPHandler)
		
	case "pps":
		ppsHandler, err := NewPPSHandler(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create PPS handler: %w", err)
		}
		handler.ppsHandler = ppsHandler.(PPSHandler)
		
	case "phc":
		phcHandler, err := NewPHCHandler(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create PHC handler: %w", err)
		}
		handler.phcHandler = phcHandler
		
	case "nmea":
		nmeaHandler, err := NewNMEAHandler(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create NMEA handler: %w", err)
		}
		handler.nmeaHandler = nmeaHandler.(NMEAHandler)
		
	case "timecard":
		timecardHandler, err := NewTimecardHandler(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Timecard handler: %w", err)
		}
		handler.timecardHandler = timecardHandler
		
	case "mock":
		mockHandler, err := NewMockHandler(cfg, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create Mock handler: %w", err)
		}
		handler.mockHandler = mockHandler
		
	default:
		return nil, fmt.Errorf("unsupported timesource type: %s", cfg.TimeSourceType)
	}

	return handler, nil
}

// Start запускает обработчик источника времени
func (h *TimeSourceHandlerImpl) Start() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.running {
		return fmt.Errorf("handler is already running")
	}

	h.logger.WithFields(logrus.Fields{
		"type": h.config.Type,
		"host": h.config.Host,
		"device": h.config.Device,
	}).Info("Starting time source handler")

	// Запуск соответствующего обработчика
	switch h.config.TimeSourceType {
	case "ptp":
		if h.ptpHandler != nil {
			if err := h.ptpHandler.Start(); err != nil {
				return fmt.Errorf("failed to start PTP handler: %w", err)
			}
		}
		
	case "ntp":
		if h.ntpHandler != nil {
			if err := h.ntpHandler.Start(); err != nil {
				return fmt.Errorf("failed to start NTP handler: %w", err)
			}
		}
		
	case "pps":
		if h.ppsHandler != nil {
			if err := h.ppsHandler.Start(); err != nil {
				return fmt.Errorf("failed to start PPS handler: %w", err)
			}
		}
		
	case "phc":
		if h.phcHandler != nil {
			if err := h.phcHandler.Start(); err != nil {
				return fmt.Errorf("failed to start PHC handler: %w", err)
			}
		}
		
	case "nmea":
		if h.nmeaHandler != nil {
			if err := h.nmeaHandler.Start(); err != nil {
				return fmt.Errorf("failed to start NMEA handler: %w", err)
			}
		}
		
	case "timecard":
		if h.timecardHandler != nil {
			if err := h.timecardHandler.Start(); err != nil {
				return fmt.Errorf("failed to start Timecard handler: %w", err)
			}
		}
		
	case "mock":
		if h.mockHandler != nil {
			if err := h.mockHandler.Start(); err != nil {
				return fmt.Errorf("failed to start Mock handler: %w", err)
			}
		}
	}

	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()

	// Запуск мониторинга в отдельной горутине
	go h.monitor()

	return nil
}

// Stop останавливает обработчик источника времени
func (h *TimeSourceHandlerImpl) Stop() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if !h.running {
		return nil
	}

	h.logger.WithFields(logrus.Fields{
		"type": h.config.Type,
	}).Info("Stopping time source handler")

	// Остановка соответствующего обработчика
	switch h.config.TimeSourceType {
	case "ptp":
		if h.ptpHandler != nil {
			h.ptpHandler.Stop()
		}
		
	case "ntp":
		if h.ntpHandler != nil {
			h.ntpHandler.Stop()
		}
		
	case "pps":
		if h.ppsHandler != nil {
			h.ppsHandler.Stop()
		}
		
	case "phc":
		if h.phcHandler != nil {
			h.phcHandler.Stop()
		}
		
	case "nmea":
		if h.nmeaHandler != nil {
			h.nmeaHandler.Stop()
		}
		
	case "timecard":
		if h.timecardHandler != nil {
			h.timecardHandler.Stop()
		}
		
	case "mock":
		if h.mockHandler != nil {
			h.mockHandler.Stop()
		}
	}

	// Сигнал остановки мониторинга
	close(h.stopChan)

	h.running = false
	h.status.Connected = false

	return nil
}

// GetTimeInfo получает информацию о времени от источника
func (h *TimeSourceHandlerImpl) GetTimeInfo() (*TimeInfo, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if !h.running {
		return nil, fmt.Errorf("handler is not running")
	}

	// Получение информации от соответствующего обработчика
	switch h.config.TimeSourceType {
	case "ptp":
		if h.ptpHandler != nil {
			return h.ptpHandler.GetTimeInfo()
		}
		
	case "ntp":
		if h.ntpHandler != nil {
			return h.ntpHandler.GetTimeInfo()
		}
		
	case "pps":
		if h.ppsHandler != nil {
			return h.ppsHandler.GetTimeInfo()
		}
		
	case "phc":
		if h.phcHandler != nil {
			return h.phcHandler.GetTimeInfo()
		}
		
	case "nmea":
		if h.nmeaHandler != nil {
			return h.nmeaHandler.GetTimeInfo()
		}
		
	case "timecard":
		if h.timecardHandler != nil {
			return h.timecardHandler.GetTimeInfo()
		}
		
	case "mock":
		if h.mockHandler != nil {
			return h.mockHandler.GetTimeInfo()
		}
	}

	return nil, fmt.Errorf("no handler available for type: %s", h.config.TimeSourceType)
}

// GetStatus получает статус соединения
func (h *TimeSourceHandlerImpl) GetStatus() ConnectionStatus {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	// Получение статуса от соответствующего обработчика
	switch h.config.TimeSourceType {
	case "ptp":
		if h.ptpHandler != nil {
			return h.ptpHandler.GetStatus()
		}
		
	case "ntp":
		if h.ntpHandler != nil {
			return h.ntpHandler.GetStatus()
		}
		
	case "pps":
		if h.ppsHandler != nil {
			return h.ppsHandler.GetStatus()
		}
		
	case "phc":
		if h.phcHandler != nil {
			return h.phcHandler.GetStatus()
		}
		
	case "nmea":
		if h.nmeaHandler != nil {
			return h.nmeaHandler.GetStatus()
		}
		
	case "timecard":
		if h.timecardHandler != nil {
			return h.timecardHandler.GetStatus()
		}
		
	case "mock":
		if h.mockHandler != nil {
			return h.mockHandler.GetStatus()
		}
	}

	return h.status
}

// GetConfig получает конфигурацию обработчика
func (h *TimeSourceHandlerImpl) GetConfig() config.TimeSourceConfig {
	return h.config
}

// monitor мониторинг состояния обработчика
func (h *TimeSourceHandlerImpl) monitor() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			h.updateStatus()
		}
	}
}

// updateStatus обновляет статус обработчика
func (h *TimeSourceHandlerImpl) updateStatus() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if !h.running {
		return
	}

	// Обновление статуса на основе активности
	now := time.Now()
	if now.Sub(h.status.LastActivity) > 30*time.Second {
		h.status.Connected = false
			h.logger.WithFields(logrus.Fields{
		"type": h.config.TimeSourceType,
	}).Warn("Time source handler timeout")
	} else {
		h.status.Connected = true
	}

	// Обновление статистики
	h.status.LastActivity = now
}