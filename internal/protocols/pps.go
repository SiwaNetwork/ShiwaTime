package protocols

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// PPSHandler реализация обработчика PPS
type PPSHandler struct {
	config     config.TimeSourceConfig
	logger     *logrus.Logger
	
	// PPS специфичные поля
	pin        int
	index      int
	cableDelay time.Duration
	edgeMode   string
	atomic     bool
	linkedDevice string
	
	// Состояние
	mu           sync.RWMutex
	running      bool
	stopChan     chan struct{}
	
	// Статистика
	pulseCount   uint64
	lastPulseTime time.Time
	lastError     error
	
	// GPIO файловые дескрипторы
	gpioFd       *os.File
	gpioValue    *os.File
}

// NewPPSHandler создает новый PPS обработчик
func NewPPSHandler(cfg config.TimeSourceConfig, logger *logrus.Logger) (*PPSHandler, error) {
	handler := &PPSHandler{
		config:    cfg,
		logger:    logger,
		pin:       cfg.Pin,
		index:     cfg.Index,
		edgeMode:  cfg.EdgeMode,
		atomic:    cfg.Atomic,
		linkedDevice: cfg.LinkedDevice,
		stopChan:  make(chan struct{}),
	}
	
	// Парсим cable delay
	if cfg.CableDelay > 0 {
		handler.cableDelay = time.Duration(cfg.CableDelay) * time.Nanosecond
	}
	
	// Устанавливаем значения по умолчанию
	if handler.pin == 0 {
		handler.pin = 18 // GPIO18 по умолчанию
	}
	if handler.edgeMode == "" {
		handler.edgeMode = "rising"
	}
	
	return handler, nil
}

// Start запускает PPS обработчик
func (h *PPSHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("PPS handler already running")
	}
	
	h.logger.WithFields(logrus.Fields{
		"pin":       h.pin,
		"edge_mode": h.edgeMode,
		"atomic":    h.atomic,
	}).Info("Starting PPS handler")
	
	// Экспортируем GPIO пин
	if err := h.exportGPIO(); err != nil {
		return fmt.Errorf("failed to export GPIO: %w", err)
	}
	
	// Настраиваем направление и edge detection
	if err := h.setupGPIO(); err != nil {
		h.unexportGPIO()
		return fmt.Errorf("failed to setup GPIO: %w", err)
	}
	
	h.running = true
	
	// Запускаем мониторинг импульсов
	go h.monitorPulses()
	
	return nil
}

// Stop останавливает PPS обработчик
func (h *PPSHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping PPS handler")
	
	close(h.stopChan)
	h.running = false
	
	// Закрываем файловые дескрипторы
	if h.gpioFd != nil {
		h.gpioFd.Close()
	}
	if h.gpioValue != nil {
		h.gpioValue.Close()
	}
	
	// Убираем экспорт GPIO
	h.unexportGPIO()
	
	return nil
}

// GetTimeInfo получает информацию о времени от PPS
func (h *PPSHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if !h.running {
		return nil, fmt.Errorf("PPS handler not running")
	}
	
	if h.lastPulseTime.IsZero() {
		return nil, fmt.Errorf("no PPS pulses received yet")
	}
	
	now := time.Now()
	offset := now.Sub(h.lastPulseTime)
	
	// Применяем компенсацию кабельной задержки
	if h.cableDelay > 0 {
		offset -= h.cableDelay
	}
	
	return &TimeInfo{
		Timestamp: h.lastPulseTime,
		Offset:    offset,
		Delay:     0, // PPS не имеет сетевой задержки
		Quality:   255, // Максимальное качество для PPS
		Stratum:   1,
		Precision: -9, // Наносекундная точность
	}, nil
}

// GetStatus получает статус соединения
func (h *PPSHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	status := ConnectionStatus{
		Connected:    h.running,
		LastActivity: h.lastPulseTime,
		ErrorCount:   0,
		LastError:    h.lastError,
		PacketsRx:    h.pulseCount,
		PacketsTx:    0, // PPS только принимает
		BytesRx:      0,
		BytesTx:      0,
	}
	
	return status
}

// GetConfig получает конфигурацию
func (h *PPSHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// GetPulseCount получает количество импульсов
func (h *PPSHandler) GetPulseCount() uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.pulseCount
}

// GetLastPulseTime получает время последнего импульса
func (h *PPSHandler) GetLastPulseTime() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastPulseTime
}

// EnablePulseOutput включает выход PPS
func (h *PPSHandler) EnablePulseOutput() error {
	// Реализация выхода PPS (если поддерживается)
	h.logger.Info("PPS output enabled")
	return nil
}

// DisablePulseOutput выключает выход PPS
func (h *PPSHandler) DisablePulseOutput() error {
	// Реализация выключения выхода PPS
	h.logger.Info("PPS output disabled")
	return nil
}

// exportGPIO экспортирует GPIO пин
func (h *PPSHandler) exportGPIO() error {
	exportPath := "/sys/class/gpio/export"
	exportData := strconv.Itoa(h.pin)
	
	return os.WriteFile(exportPath, []byte(exportData), 0644)
}

// unexportGPIO убирает экспорт GPIO пина
func (h *PPSHandler) unexportGPIO() error {
	unexportPath := "/sys/class/gpio/unexport"
	unexportData := strconv.Itoa(h.pin)
	
	return os.WriteFile(unexportPath, []byte(unexportData), 0644)
}

// setupGPIO настраивает GPIO пин
func (h *PPSHandler) setupGPIO() error {
	// Устанавливаем направление как input
	directionPath := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", h.pin)
	if err := os.WriteFile(directionPath, []byte("in"), 0644); err != nil {
		return fmt.Errorf("failed to set GPIO direction: %w", err)
	}
	
	// Устанавливаем edge detection
	edgePath := fmt.Sprintf("/sys/class/gpio/gpio%d/edge", h.pin)
	edgeData := h.edgeMode
	if err := os.WriteFile(edgePath, []byte(edgeData), 0644); err != nil {
		return fmt.Errorf("failed to set GPIO edge: %w", err)
	}
	
	// Открываем файл значения для чтения
	valuePath := fmt.Sprintf("/sys/class/gpio/gpio%d/value", h.pin)
	valueFile, err := os.Open(valuePath)
	if err != nil {
		return fmt.Errorf("failed to open GPIO value file: %w", err)
	}
	h.gpioValue = valueFile
	
	return nil
}

// monitorPulses мониторит PPS импульсы
func (h *PPSHandler) monitorPulses() {
	// Используем epoll для эффективного мониторинга
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		h.logger.WithError(err).Error("Failed to create epoll")
		return
	}
	defer syscall.Close(epfd)
	
	// Добавляем GPIO файл в epoll
	event := syscall.EpollEvent{
		Events: syscall.EPOLLIN | syscall.EPOLLET,
		Fd:     int32(h.gpioValue.Fd()),
	}
	
	if err := syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, int(event.Fd), &event); err != nil {
		h.logger.WithError(err).Error("Failed to add GPIO to epoll")
		return
	}
	
	events := make([]syscall.EpollEvent, 1)
	
	for {
		select {
		case <-h.stopChan:
			return
		default:
			// Ждем события с таймаутом
			n, err := syscall.EpollWait(epfd, events, 1000) // 1 секунда таймаут
			if err != nil {
				if err == syscall.EINTR {
					continue
				}
				h.logger.WithError(err).Error("Epoll wait failed")
				return
			}
			
			if n > 0 {
				// Обрабатываем импульс
				h.handlePulse()
			}
		}
	}
}

// handlePulse обрабатывает PPS импульс
func (h *PPSHandler) handlePulse() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	now := time.Now()
	h.pulseCount++
	h.lastPulseTime = now
	h.lastError = nil
	
	h.logger.WithFields(logrus.Fields{
		"pulse_count": h.pulseCount,
		"timestamp":   now.Format(time.RFC3339Nano),
	}).Debug("PPS pulse received")
}