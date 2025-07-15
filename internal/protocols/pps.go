package protocols

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
	"golang.org/x/sys/unix"
)

const (
	// PPS ioctl constants
	PPS_GETPARAMS = 0x800870a1
	PPS_SETPARAMS = 0x400870a2
	PPS_GETCAP    = 0x800870a3
	PPS_FETCH     = 0xc00870a4

	// PPS capability flags
	PPS_CAPTUREASSERT = 0x01
	PPS_CAPTURECLEAR  = 0x02
	PPS_CAPTUREBOTH   = 0x03
	PPS_OFFSETASSERT  = 0x10
	PPS_OFFSETCLEAR   = 0x20

	// PPS param flags
	PPS_ECHOASSERT = 0x40
	PPS_ECHOCLEAR  = 0x80

	// GPIO constants for manual PPS
	GPIO_SYSFS_PATH = "/sys/class/gpio"
	GPIO_EXPORT_PATH = GPIO_SYSFS_PATH + "/export"
	GPIO_UNEXPORT_PATH = GPIO_SYSFS_PATH + "/unexport"
)

// PPSTimeInfo структура для PPS времени
type PPSTimeInfo struct {
	Assert    PPSTimestamp
	Clear     PPSTimestamp
	AssertSequence uint32
	ClearSequence  uint32
}

// PPSTimestamp структура временной метки PPS
type PPSTimestamp struct {
	Sec  int64
	Nsec int32
}

// PPSParams параметры PPS
type PPSParams struct {
	ApiVersion int32
	Mode       int32
	AssertOffset PPSTimestamp
	ClearOffset  PPSTimestamp
}

// ppsHandler реализация PPS обработчика
type ppsHandler struct {
	config    config.TimeSourceConfig
	logger    *logrus.Logger
	
	mu        sync.RWMutex
	running   bool
	status    ConnectionStatus
	
	// PPS device
	device    string
	fd        int
	
	// GPIO support
	gpioPin   int
	gpioFd    int
	useGPIO   bool
	
	// PPS parameters
	capability int32
	params     PPSParams
	signalType PPSSignalType
	
	// Timing data
	lastTimestamp  time.Time
	eventCount     uint64
	lastEvent      *PPSEvent
	
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPPSHandler создает новый PPS обработчик
func NewPPSHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	h := &ppsHandler{
		config:     config,
		logger:     logger,
		device:     config.Device,
		fd:         -1,
		gpioFd:     -1,
		signalType: PPSSignalRising,
		ctx:        ctx,
		cancel:     cancel,
		status:     ConnectionStatus{},
	}
	
	// Определяем тип сигнала из конфигурации
	if config.PPSMode != "" {
		switch strings.ToLower(config.PPSMode) {
		case "rising", "assert":
			h.signalType = PPSSignalRising
		case "falling", "clear":
			h.signalType = PPSSignalFalling
		case "both":
			h.signalType = PPSSignalBoth
		}
	}
	
	// Проверяем, используем ли мы GPIO
	if config.GPIOPin > 0 {
		h.gpioPin = config.GPIOPin
		h.useGPIO = true
		h.logger.WithField("gpio_pin", h.gpioPin).Info("Using GPIO for PPS")
	}
	
	return h, nil
}

// Start запускает PPS обработчик
func (h *ppsHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("PPS handler already running")
	}
	
	h.logger.WithFields(logrus.Fields{
		"device":      h.device,
		"signal_type": h.signalType,
		"use_gpio":    h.useGPIO,
	}).Info("Starting PPS handler")
	
	var err error
	
	if h.useGPIO {
		err = h.setupGPIO()
	} else {
		err = h.setupPPSDevice()
	}
	
	if err != nil {
		return fmt.Errorf("failed to setup PPS: %w", err)
	}
	
	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()
	
	// Запускаем обработчик событий
	go h.handlePPSEvents()
	
	return nil
}

// Stop останавливает PPS обработчик
func (h *ppsHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping PPS handler")
	
	h.cancel()
	h.running = false
	h.status.Connected = false
	
	if h.fd >= 0 {
		unix.Close(h.fd)
		h.fd = -1
	}
	
	if h.gpioFd >= 0 {
		unix.Close(h.gpioFd)
		h.gpioFd = -1
		h.cleanupGPIO()
	}
	
	return nil
}

// GetTimeInfo получает информацию о времени от PPS
func (h *ppsHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	running := h.running
	lastEvent := h.lastEvent
	lastTimestamp := h.lastTimestamp
	h.mu.RUnlock()
	
	if !running {
		return nil, fmt.Errorf("PPS handler not running")
	}
	
	if lastEvent == nil || lastTimestamp.IsZero() {
		return nil, fmt.Errorf("no PPS events received")
	}
	
	// PPS обеспечивает очень высокое качество времени (обычно микросекундная точность)
	info := &TimeInfo{
		Timestamp: lastEvent.Timestamp,
		Offset:    0, // PPS не предоставляет offset
		Delay:     0, // PPS не имеет сетевой задержки
		Quality:   240, // Очень высокое качество
		Precision: -6,  // Микросекундная точность
	}
	
	// Проверяем актуальность последнего события
	age := time.Since(lastEvent.Timestamp)
	if age > 5*time.Second {
		info.Quality = 100 // Снижаем качество для старых событий
	}
	
	h.logger.WithFields(logrus.Fields{
		"timestamp":  lastEvent.Timestamp,
		"signal_type": lastEvent.SignalType,
		"age":        age,
	}).Debug("PPS time info")
	
	return info, nil
}

// GetStatus возвращает статус соединения
func (h *ppsHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

// GetConfig возвращает конфигурацию
func (h *ppsHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// GetEventCount возвращает количество обработанных PPS событий
func (h *ppsHandler) GetEventCount() uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.eventCount
}

// GetLastEvent возвращает последнее PPS событие
func (h *ppsHandler) GetLastEvent() *PPSEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastEvent
}

// setupPPSDevice настраивает PPS устройство
func (h *ppsHandler) setupPPSDevice() error {
	if h.device == "" {
		h.device = "/dev/pps0"
	}
	
	// Открываем PPS устройство
	fd, err := unix.Open(h.device, unix.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open PPS device %s: %w", h.device, err)
	}
	
	h.fd = fd
	
	// Получаем возможности устройства
	err = h.getPPSCapability()
	if err != nil {
		unix.Close(h.fd)
		h.fd = -1
		return fmt.Errorf("failed to get PPS capability: %w", err)
	}
	
	// Настраиваем параметры
	err = h.setPPSParams()
	if err != nil {
		unix.Close(h.fd)
		h.fd = -1
		return fmt.Errorf("failed to set PPS params: %w", err)
	}
	
	h.logger.WithFields(logrus.Fields{
		"device":     h.device,
		"capability": h.capability,
		"mode":       h.params.Mode,
	}).Info("PPS device configured")
	
	return nil
}

// setupGPIO настраивает GPIO для PPS
func (h *ppsHandler) setupGPIO() error {
	// Экспортируем GPIO pin
	err := h.exportGPIO()
	if err != nil {
		return fmt.Errorf("failed to export GPIO pin %d: %w", h.gpioPin, err)
	}
	
	// Настраиваем как вход
	err = h.setGPIODirection("in")
	if err != nil {
		h.cleanupGPIO()
		return fmt.Errorf("failed to set GPIO direction: %w", err)
	}
	
	// Настраиваем edge detection
	edge := "rising"
	switch h.signalType {
	case PPSSignalFalling:
		edge = "falling"
	case PPSSignalBoth:
		edge = "both"
	}
	
	err = h.setGPIOEdge(edge)
	if err != nil {
		h.cleanupGPIO()
		return fmt.Errorf("failed to set GPIO edge: %w", err)
	}
	
	// Открываем value файл для poll()
	valuePath := fmt.Sprintf("%s/gpio%d/value", GPIO_SYSFS_PATH, h.gpioPin)
	h.gpioFd, err = unix.Open(valuePath, unix.O_RDONLY, 0)
	if err != nil {
		h.cleanupGPIO()
		return fmt.Errorf("failed to open GPIO value file: %w", err)
	}
	
	h.logger.WithFields(logrus.Fields{
		"gpio_pin": h.gpioPin,
		"edge":     edge,
	}).Info("GPIO configured for PPS")
	
	return nil
}

// exportGPIO экспортирует GPIO pin
func (h *ppsHandler) exportGPIO() error {
	// Проверяем, не экспортирован ли уже
	gpioPath := fmt.Sprintf("%s/gpio%d", GPIO_SYSFS_PATH, h.gpioPin)
	if _, err := os.Stat(gpioPath); err == nil {
		return nil // Уже экспортирован
	}
	
	// Экспортируем pin
	exportFile, err := os.OpenFile(GPIO_EXPORT_PATH, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer exportFile.Close()
	
	_, err = exportFile.WriteString(strconv.Itoa(h.gpioPin))
	return err
}

// setGPIODirection устанавливает направление GPIO
func (h *ppsHandler) setGPIODirection(direction string) error {
	directionPath := fmt.Sprintf("%s/gpio%d/direction", GPIO_SYSFS_PATH, h.gpioPin)
	directionFile, err := os.OpenFile(directionPath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer directionFile.Close()
	
	_, err = directionFile.WriteString(direction)
	return err
}

// setGPIOEdge устанавливает edge detection для GPIO
func (h *ppsHandler) setGPIOEdge(edge string) error {
	edgePath := fmt.Sprintf("%s/gpio%d/edge", GPIO_SYSFS_PATH, h.gpioPin)
	edgeFile, err := os.OpenFile(edgePath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer edgeFile.Close()
	
	_, err = edgeFile.WriteString(edge)
	return err
}

// cleanupGPIO очищает GPIO resources
func (h *ppsHandler) cleanupGPIO() {
	unexportFile, err := os.OpenFile(GPIO_UNEXPORT_PATH, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer unexportFile.Close()
	
	unexportFile.WriteString(strconv.Itoa(h.gpioPin))
}

// getPPSCapability получает возможности PPS устройства
func (h *ppsHandler) getPPSCapability() error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(h.fd),
		uintptr(PPS_GETCAP),
		uintptr(unsafe.Pointer(&h.capability)))
	
	if errno != 0 {
		return errno
	}
	
	return nil
}

// setPPSParams устанавливает параметры PPS
func (h *ppsHandler) setPPSParams() error {
	h.params.ApiVersion = 1
	
	// Устанавливаем режим в зависимости от типа сигнала
	switch h.signalType {
	case PPSSignalRising:
		h.params.Mode = PPS_CAPTUREASSERT
	case PPSSignalFalling:
		h.params.Mode = PPS_CAPTURECLEAR
	case PPSSignalBoth:
		h.params.Mode = PPS_CAPTUREBOTH
	}
	
	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(h.fd),
		uintptr(PPS_SETPARAMS),
		uintptr(unsafe.Pointer(&h.params)))
	
	if errno != 0 {
		return errno
	}
	
	return nil
}

// handlePPSEvents обрабатывает PPS события
func (h *ppsHandler) handlePPSEvents() {
	if h.useGPIO {
		h.handleGPIOEvents()
	} else {
		h.handlePPSDeviceEvents()
	}
}

// handlePPSDeviceEvents обрабатывает события от PPS устройства
func (h *ppsHandler) handlePPSDeviceEvents() {
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
			info, err := h.fetchPPSEvent()
			if err != nil {
				h.logger.WithError(err).Error("Failed to fetch PPS event")
				time.Sleep(100 * time.Millisecond)
				continue
			}
			
			h.processPPSInfo(info)
		}
	}
}

// handleGPIOEvents обрабатывает события от GPIO
func (h *ppsHandler) handleGPIOEvents() {
	pollFd := unix.PollFd{
		Fd:     int32(h.gpioFd),
		Events: unix.POLLPRI | unix.POLLERR,
	}
	
	// Первое чтение для очистки
	buf := make([]byte, 64)
	unix.Read(h.gpioFd, buf)
	
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
			// Poll для ожидания события
			_, err := unix.Poll([]unix.PollFd{pollFd}, 1000) // 1 секунда timeout
			if err != nil {
				if err == unix.EINTR {
					continue
				}
				h.logger.WithError(err).Error("GPIO poll error")
				time.Sleep(100 * time.Millisecond)
				continue
			}
			
			if pollFd.Revents&unix.POLLPRI != 0 {
				// Событие GPIO
				timestamp := time.Now()
				
				// Читаем значение для очистки
				unix.Pread(h.gpioFd, buf, 0)
				
				event := &PPSEvent{
					Timestamp:   timestamp,
					SignalType:  h.signalType,
					SequenceNum: h.eventCount + 1,
				}
				
				h.mu.Lock()
				h.eventCount++
				h.lastEvent = event
				h.lastTimestamp = timestamp
				h.status.LastActivity = timestamp
				h.mu.Unlock()
				
				h.logger.WithFields(logrus.Fields{
					"timestamp": timestamp,
					"sequence":  event.SequenceNum,
				}).Debug("GPIO PPS event")
			}
		}
	}
}

// fetchPPSEvent получает PPS событие от устройства
func (h *ppsHandler) fetchPPSEvent() (*PPSTimeInfo, error) {
	var info PPSTimeInfo
	
	// Параметры для FETCH
	fetchParams := struct {
		TSFormat int32
		Timeout  struct {
			Sec  int64
			Nsec int32
		}
	}{
		TSFormat: 0, // PPS_TSFMT_TSPEC
	}
	
	fetchParams.Timeout.Sec = 5 // 5 секунд timeout
	
	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(h.fd),
		uintptr(PPS_FETCH),
		uintptr(unsafe.Pointer(&fetchParams)))
	
	if errno != 0 {
		return nil, errno
	}
	
	return &info, nil
}

// processPPSInfo обрабатывает полученную PPS информацию
func (h *ppsHandler) processPPSInfo(info *PPSTimeInfo) {
	var timestamp time.Time
	var signalType PPSSignalType
	
	// Выбираем timestamp в зависимости от режима
	switch h.signalType {
	case PPSSignalRising:
		timestamp = time.Unix(info.Assert.Sec, int64(info.Assert.Nsec))
		signalType = PPSSignalRising
	case PPSSignalFalling:
		timestamp = time.Unix(info.Clear.Sec, int64(info.Clear.Nsec))
		signalType = PPSSignalFalling
	case PPSSignalBoth:
		// Используем более свежий timestamp
		assertTime := time.Unix(info.Assert.Sec, int64(info.Assert.Nsec))
		clearTime := time.Unix(info.Clear.Sec, int64(info.Clear.Nsec))
		if assertTime.After(clearTime) {
			timestamp = assertTime
			signalType = PPSSignalRising
		} else {
			timestamp = clearTime
			signalType = PPSSignalFalling
		}
	}
	
	if timestamp.IsZero() {
		return
	}
	
	event := &PPSEvent{
		Timestamp:   timestamp,
		SignalType:  signalType,
		SequenceNum: h.eventCount + 1,
	}
	
	h.mu.Lock()
	h.eventCount++
	h.lastEvent = event
	h.lastTimestamp = timestamp
	h.status.LastActivity = timestamp
	h.mu.Unlock()
	
	h.logger.WithFields(logrus.Fields{
		"timestamp":   timestamp,
		"signal_type": signalType,
		"sequence":    event.SequenceNum,
	}).Debug("PPS event")
}