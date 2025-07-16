package protocols

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
	"golang.org/x/sys/unix"
)

const (
	// PHC ioctl constants
	PTP_CLOCK_GETCAPS  = 0x80487001
	PTP_SYS_OFFSET     = 0x80487005
	PTP_SYS_OFFSET_PRECISE = 0x80487008
	PTP_SYS_OFFSET_EXTENDED = 0x80487009
	PTP_PIN_GETFUNC    = 0xc0604706
	PTP_PIN_SETFUNC    = 0x40604707
	PTP_PEROUT_REQUEST = 0x40384703
	PTP_EXTTS_REQUEST  = 0x40104702
	
	// PHC capabilities
	PTP_PF_NONE      = 0
	PTP_PF_EXTTS     = 1
	PTP_PF_PEROUT    = 2
	PTP_PF_PHYSYNC   = 3
)

// PHCCapabilities представляет возможности PHC
type PHCCapabilities struct {
	MaxAdj     int32
	NAlarm     int32
	NExtTS     int32
	NPerOut    int32
	PPS        int32
	NChannel   int32
	CrossTsErr int32
	AdjPhase   int32
}

// PHCSysOffset структура для синхронизации с системными часами
type PHCSysOffset struct {
	NMeasurements uint32
	Reserved      [3]uint32
	TS            [25]PHCTimestamp // max 25 measurements
}

// PHCExtTSRequest запрос на внешние временные метки
type PHCExtTSRequest struct {
	Index uint32
	Flags uint32
}

// phcHandler реализация PHC обработчика
type phcHandler struct {
	config    config.TimeSourceConfig
	logger    *logrus.Logger
	
	mu        sync.RWMutex
	running   bool
	status    ConnectionStatus
	
	// PHC device
	device    string
	fd        int
	index     int
	
	// PHC capabilities
	caps      PHCCapabilities
	
	// Timing data
	lastOffset   time.Duration
	lastPHCTime  time.Time
	lastSysTime  time.Time
	
	// Statistics
	offsetMeasurements uint64
	
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewPHCHandler создает новый PHC обработчик
func NewPHCHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	h := &phcHandler{
		config:  config,
		logger:  logger,
		device:  config.Device,
		index:   config.PHCIndex,
		fd:      -1,
		ctx:     ctx,
		cancel:  cancel,
		status:  ConnectionStatus{},
	}
	
	// Если индекс не указан, но указано устройство, пытаемся определить индекс
	if h.index == 0 && h.device == "" && config.Interface != "" {
		h.device = fmt.Sprintf("/dev/ptp%d", h.findPHCIndexByInterface(config.Interface))
	} else if h.device == "" {
		h.device = fmt.Sprintf("/dev/ptp%d", h.index)
	}
	
	return h, nil
}

// Start запускает PHC обработчик
func (h *phcHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("PHC handler already running")
	}
	
	h.logger.WithFields(logrus.Fields{
		"device": h.device,
		"index":  h.index,
	}).Info("Starting PHC handler")
	
	// Открываем PHC устройство
	if err := h.openPHCDevice(); err != nil {
		return fmt.Errorf("failed to open PHC device: %w", err)
	}
	
	// Получаем возможности устройства
	if err := h.getPHCCapabilities(); err != nil {
		h.closePHCDevice()
		return fmt.Errorf("failed to get PHC capabilities: %w", err)
	}
	
	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()
	
	// Запускаем мониторинг offset'а
	go h.monitorOffset()
	
	return nil
}

// Stop останавливает PHC обработчик
func (h *phcHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping PHC handler")
	
	h.cancel()
	h.running = false
	h.status.Connected = false
	
	h.closePHCDevice()
	
	return nil
}

// GetTimeInfo получает информацию о времени от PHC
func (h *phcHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	running := h.running
	lastOffset := h.lastOffset
	lastPHCTime := h.lastPHCTime
	lastSysTime := h.lastSysTime
	h.mu.RUnlock()
	
	if !running {
		return nil, fmt.Errorf("PHC handler not running")
	}
	
	if lastPHCTime.IsZero() || lastSysTime.IsZero() {
		return nil, fmt.Errorf("no PHC measurements available")
	}
	
	// PHC обеспечивает высокое качество времени (наносекундная точность)
	info := &TimeInfo{
		Timestamp: lastPHCTime,
		Offset:    lastOffset,
		Delay:     0, // PHC не имеет сетевой задержки
		Quality:   250, // Очень высокое качество
		Precision: -9,  // Наносекундная точность
	}
	
	// Проверяем актуальность последнего измерения
	age := time.Since(lastSysTime)
	if age > 10*time.Second {
		info.Quality = 150 // Снижаем качество для старых измерений
	}
	
	h.logger.WithFields(logrus.Fields{
		"phc_time":   lastPHCTime,
		"sys_time":   lastSysTime,
		"offset":     lastOffset,
		"age":        age,
	}).Debug("PHC time info")
	
	return info, nil
}

// GetStatus возвращает статус соединения
func (h *phcHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

// GetConfig возвращает конфигурацию
func (h *phcHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// GetGNSSInfo возвращает GNSS информацию (PHC не поддерживает GNSS напрямую)
func (h *phcHandler) GetGNSSInfo() GNSSStatus {
	return GNSSStatus{
		FixType:         0, // No fix
		FixQuality:      0,
		SatellitesUsed:  0,
		SatellitesVisible: 0,
		HDOP:            0,
		VDOP:            0,
	}
}

// GetCapabilities возвращает возможности PHC
func (h *phcHandler) GetCapabilities() PHCCapabilities {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.caps
}

// GetPHCInfo возвращает информацию о PHC
func (h *phcHandler) GetPHCInfo() *PHCInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	return &PHCInfo{
		Index:        h.index,
		Name:         h.device,
		MaxAdj:       int64(h.caps.MaxAdj),
		NChannels:    int(h.caps.NChannel),
		PPSAvail:     h.caps.PPS > 0,
		CrossTsAvail: h.caps.CrossTsErr == 0,
	}
}

// GetOffset возвращает текущий offset между PHC и системными часами
func (h *phcHandler) GetOffset() (time.Duration, error) {
	offset, err := h.measureOffset()
	if err != nil {
		return 0, err
	}
	
	h.mu.Lock()
	h.lastOffset = offset
	h.offsetMeasurements++
	h.mu.Unlock()
	
	return offset, nil
}

// openPHCDevice открывает PHC устройство
func (h *phcHandler) openPHCDevice() error {
	fd, err := unix.Open(h.device, unix.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open PHC device %s: %w", h.device, err)
	}
	
	h.fd = fd
	return nil
}

// closePHCDevice закрывает PHC устройство
func (h *phcHandler) closePHCDevice() {
	if h.fd >= 0 {
		unix.Close(h.fd)
		h.fd = -1
	}
}

// getPHCCapabilities получает возможности PHC устройства
func (h *phcHandler) getPHCCapabilities() error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(h.fd),
		uintptr(PTP_CLOCK_GETCAPS),
		uintptr(unsafe.Pointer(&h.caps)))
	
	if errno != 0 {
		return errno
	}
	
	h.logger.WithFields(logrus.Fields{
		"max_adj":    h.caps.MaxAdj,
		"n_extts":    h.caps.NExtTS,
		"n_perout":   h.caps.NPerOut,
		"pps_avail":  h.caps.PPS > 0,
		"n_channels": h.caps.NChannel,
	}).Info("PHC capabilities")
	
	return nil
}

// measureOffset измеряет offset между PHC и системными часами
func (h *phcHandler) measureOffset() (time.Duration, error) {
	if h.fd < 0 {
		return 0, fmt.Errorf("PHC device not open")
	}
	
	var sysOffset PHCSysOffset
	sysOffset.NMeasurements = 1
	
	// Используем расширенный ioctl если доступен
	ioctl := PTP_SYS_OFFSET
	if h.caps.CrossTsErr == 0 {
		ioctl = PTP_SYS_OFFSET_EXTENDED
	}
	
	// Получаем время перед вызовом ioctl
	sysBefore := time.Now()
	
	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(h.fd),
		uintptr(ioctl),
		uintptr(unsafe.Pointer(&sysOffset)))
	
	// Получаем время после вызова ioctl
	sysAfter := time.Now()
	
	if errno != 0 {
		return 0, errno
	}
	
	// Вычисляем offset
	// TS[0] - системное время до чтения PHC
	// TS[1] - время PHC
	// TS[2] - системное время после чтения PHC
	if sysOffset.NMeasurements > 0 {
		sys1 := time.Unix(sysOffset.TS[0].Seconds, int64(sysOffset.TS[0].Nanoseconds))
		phc := time.Unix(sysOffset.TS[1].Seconds, int64(sysOffset.TS[1].Nanoseconds))
		sys2 := time.Unix(sysOffset.TS[2].Seconds, int64(sysOffset.TS[2].Nanoseconds))
		
		// Средний системный time
		avgSys := sys1.Add(sys2.Sub(sys1) / 2)
		offset := phc.Sub(avgSys)
		
		h.mu.Lock()
		h.lastPHCTime = phc
		h.lastSysTime = avgSys
		h.mu.Unlock()
		
		return offset, nil
	}
	
	// Fallback - простое измерение
	phcTime, err := h.readPHCTime()
	if err != nil {
		return 0, err
	}
	
	sysTime := sysBefore.Add(sysAfter.Sub(sysBefore) / 2)
	offset := phcTime.Sub(sysTime)
	
	h.mu.Lock()
	h.lastPHCTime = phcTime
	h.lastSysTime = sysTime
	h.mu.Unlock()
	
	return offset, nil
}

// readPHCTime читает текущее время PHC
func (h *phcHandler) readPHCTime() (time.Time, error) {
	if h.fd < 0 {
		return time.Time{}, fmt.Errorf("PHC device not open")
	}
	
	var ts unix.Timespec
	
	// Читаем время PHC используя clock_gettime
	err := unix.ClockGettime(unix.CLOCK_REALTIME, &ts)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Unix(ts.Sec, ts.Nsec), nil
}

// monitorOffset мониторит offset между PHC и системными часами
func (h *phcHandler) monitorOffset() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			_, err := h.GetOffset()
			if err != nil {
				h.logger.WithError(err).Error("Failed to measure PHC offset")
				h.mu.Lock()
				h.status.LastError = err
				h.mu.Unlock()
			} else {
				h.mu.Lock()
				h.status.LastActivity = time.Now()
				h.status.LastError = nil
				h.mu.Unlock()
			}
		}
	}
}

// findPHCIndexByInterface находит PHC индекс по имени сетевого интерфейса
func (h *phcHandler) findPHCIndexByInterface(interfaceName string) int {
	// Пытаемся найти PHC индекс через ethtool или sysfs
	phcPath := fmt.Sprintf("/sys/class/net/%s/device/ptp", interfaceName)
	
	// Читаем содержимое директории
	entries, err := os.ReadDir(phcPath)
	if err != nil {
		h.logger.WithError(err).Warn("Could not read PHC directory for interface")
		return 0
	}
	
	// Ищем ptp* директории
	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 3 && entry.Name()[:3] == "ptp" {
			// Извлекаем номер из имени ptp*
			var index int
			if n, err := fmt.Sscanf(entry.Name(), "ptp%d", &index); n == 1 && err == nil {
				h.logger.WithFields(logrus.Fields{
					"interface": interfaceName,
					"phc_index": index,
				}).Info("Found PHC index for interface")
				return index
			}
		}
	}
	
	h.logger.WithField("interface", interfaceName).Warn("Could not find PHC index for interface")
	return 0
}

// AdjustFrequency корректирует частоту PHC
func (h *phcHandler) AdjustFrequency(ppb int64) error {
	if h.fd < 0 {
		return fmt.Errorf("PHC device not open")
	}
	
	// Проверяем лимиты
	if ppb > int64(h.caps.MaxAdj) || ppb < -int64(h.caps.MaxAdj) {
		return fmt.Errorf("adjustment %d ppb exceeds limits ±%d ppb", ppb, h.caps.MaxAdj)
	}
	
	// Используем adjtimex syscall для корректировки частоты
	var timex unix.Timex
	timex.Modes = unix.ADJ_FREQUENCY
	timex.Freq = ppb * 65536 / 1000000 // Конвертируем ppb в формат ядра
	
	_, err := unix.Adjtimex(&timex)
	if err != nil {
		return fmt.Errorf("failed to adjust PHC frequency: %w", err)
	}
	
	h.logger.WithField("ppb", ppb).Debug("Adjusted PHC frequency")
	return nil
}

// EnableExternalTimestamps включает внешние временные метки
func (h *phcHandler) EnableExternalTimestamps(index int, enable bool) error {
	if h.fd < 0 {
		return fmt.Errorf("PHC device not open")
	}
	
	if index >= int(h.caps.NExtTS) {
		return fmt.Errorf("external timestamp index %d exceeds available channels %d", index, h.caps.NExtTS)
	}
	
	var req PHCExtTSRequest
	req.Index = uint32(index)
	if enable {
		req.Flags = 1 // Enable
	} else {
		req.Flags = 0 // Disable
	}
	
	_, _, errno := unix.Syscall(unix.SYS_IOCTL,
		uintptr(h.fd),
		uintptr(PTP_EXTTS_REQUEST),
		uintptr(unsafe.Pointer(&req)))
	
	if errno != 0 {
		return errno
	}
	
	h.logger.WithFields(logrus.Fields{
		"index":  index,
		"enable": enable,
	}).Info("External timestamps configuration changed")
	
	return nil
}