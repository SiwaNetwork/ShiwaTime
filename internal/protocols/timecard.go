package protocols

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
    tcdrv "github.com/shiwatime/shiwatime/internal/timecard"
)

// timecardHandler реализация Timecard обработчика
type timecardHandler struct {
	config    config.TimeSourceConfig
	logger    *logrus.Logger
	
	mu        sync.RWMutex
	running   bool
	status    ConnectionStatus
	// card telemetry
	ppsCount     uint64
	lastPPSTime  time.Time
	gnssFixValid bool
	lastOffset   time.Duration
	// internal
	wg       sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	drv       tcdrv.Driver
}

// NewTimecardHandler создает новый Timecard обработчик
func NewTimecardHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &timecardHandler{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		status: ConnectionStatus{},
	}, nil
}

// Start запускает Timecard обработчик
func (h *timecardHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("Timecard handler already running")
	}
	
	h.logger.WithField("device", h.config.Device).Info("Starting Timecard handler")
	
	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()

	// Try to open PCI driver if pci_addr provided
	if h.config.Options != nil {
		if addr, ok := h.config.Options["pci_addr"]; ok && addr != "" {
			d, err := tcdrv.OpenPCI(addr)
			if err != nil {
				h.logger.WithError(err).Warn("time-card: failed to open PCI device, will use status file")
			} else {
				h.drv = d
				h.logger.WithField("pci_addr", addr).Info("time-card: PCI BAR0 mapped")
			}
		}
	}
 
 	// TODO: Реализовать работу с timecard устройствами
 
 	return nil
}

// Stop останавливает Timecard обработчик
func (h *timecardHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping Timecard handler")
	
	h.cancel()
	h.running = false
	h.status.Connected = false
	h.wg.Wait()
 
 	if h.drv != nil {
 		h.drv.Close()
 		h.drv = nil
 	}
 	return nil
}

// GetTimeInfo получает информацию о времени от Timecard
func (h *timecardHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	ppsTime := h.lastPPSTime
	offset := h.lastOffset
	ppsValid := !ppsTime.IsZero()
	h.mu.RUnlock()

	if !ppsValid {
		return nil, fmt.Errorf("no PPS data from time-card yet")
	}

	quality := 240
	if !h.gnssFixValid {
		// без GNSS фикс – понижаем оценку
		quality = 150
	}

	return &TimeInfo{
		Timestamp: ppsTime,
		Offset:    offset,
		Delay:     0,
		Quality:   quality,
		Precision: -9,
	}, nil
}

// GetStatus возвращает статус соединения
func (h *timecardHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

// GetConfig возвращает конфигурацию
func (h *timecardHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// monitorLoop периодически считывает статус карты или имитирует данные
func (h *timecardHandler) monitorLoop() {
	defer h.wg.Done()
	statusPath := "/dev/timecard0-status"
	if h.config.Device != "" {
		statusPath = h.config.Device
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			var (
				ppsTime time.Time
				ppsCnt uint64
				gnssFix bool
				err error
			)
			if h.drv != nil {
				ppsTime, ppsCnt, gnssFix, err = h.readRegisters()
			} else {
				ppsTime, ppsCnt, gnssFix, err = h.readStatus(statusPath)
			}
			if err != nil {
				h.logger.WithError(err).Debug("time-card: status read failed, switching to simulated mode")
				// fabricate PPS each second to keep pipeline alive
				ppsTime = time.Now().Truncate(time.Second)
				ppsCnt = h.ppsCount + 1
			}

			h.mu.Lock()
			h.lastPPSTime = ppsTime
			h.ppsCount = ppsCnt
			h.gnssFixValid = gnssFix
			h.lastOffset = time.Since(ppsTime)
			h.status.LastActivity = time.Now()
			h.mu.Unlock()
		}
	}
}

// readStatus пытается прочитать статус time-card.
// Формат (текстовый) ожидается:
//   PPS_COUNT=<num>\nLAST_PPS_NS=<unix-ns>\nGNSS_FIX=<0|1>\n
func (h *timecardHandler) readStatus(path string) (time.Time, uint64, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, 0, false, err
	}
	var ppsCnt uint64
	var lastNs int64
	gnssFix := false
	for _, ln := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(ln, "PPS_COUNT=") {
			fmt.Sscanf(ln, "PPS_COUNT=%d", &ppsCnt)
		} else if strings.HasPrefix(ln, "LAST_PPS_NS=") {
			fmt.Sscanf(ln, "LAST_PPS_NS=%d", &lastNs)
		} else if strings.HasPrefix(ln, "GNSS_FIX=") {
			var v int
			fmt.Sscanf(ln, "GNSS_FIX=%d", &v)
			gnssFix = v == 1
		}
	}
	ts := time.Unix(0, lastNs)
	return ts, ppsCnt, gnssFix, nil
}

// readRegisters reads GNSS/PPS info via pciDriver
func (h *timecardHandler) readRegisters() (time.Time, uint64, bool, error) {
	if h.drv == nil {
		return time.Time{}, 0, false, fmt.Errorf("no driver")
	}
	// Read PPS counter 64-bit
	lo := uint64(h.drv.ReadU32(tcRegPpsCountL))
	hi := uint64(h.drv.ReadU32(tcRegPpsCountH))
	ppsCnt := (hi << 32) | lo
	lastNs := int64(h.drv.ReadU32(tcRegPpsLastNs))
	// GNSS fix / sats
	fixReg := h.drv.ReadU32(tcRegGnssFix)
	gnssValid := (fixReg & 0x1) == 1
	// Build timestamp from ToD
	secLo := uint64(h.drv.ReadU32(tcRegTodSecL))
	secHi := uint64(h.drv.ReadU32(tcRegTodSecH) & 0xFFFF)
	utcSec := (secHi << 32) | secLo
	ns := uint64(h.drv.ReadU32(tcRegTodNs))
	ppsTime := time.Unix(int64(utcSec), int64(ns))
	if lastNs != 0 {
		ppsTime = time.Unix(int64(utcSec), lastNs)
	}
	return ppsTime, ppsCnt, gnssValid, nil
}