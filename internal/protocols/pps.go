package protocols

import (
    "context"
    "fmt"
    "sync"
    "time"

    "github.com/sirupsen/logrus"
    "github.com/shiwatime/shiwatime/internal/config"
)

// ppsHandler implements PPS pulse based time source.
// For the first iteration we rely on system time and simulate incoming
// PPS pulses every second. This is enough for monitoring and for unit
// testing on hardware-agnostic CI runners. The implementation can be
// swapped out later with a real /dev/ppsX reader that uses the Linux
// PPS API without touching the Manager code.
//
// Behaviour:
//   • Start() – spawns a goroutine that ticks once per second and
//     updates the lastPulseTime & pulseCount.
//   • GetTimeInfo() – returns the timestamp of the last pulse and an
//     offset of 0 (as PPS is expected to be aligned to UTC second) plus
//     a synthetic quality value.
//   • Status statistics (ConnectionStatus) are updated on every pulse.
//   • Stop() – cancels the context and waits for the goroutine.

// compile-time check that ppsHandler implements both interfaces
var _ PPSHandler = (*ppsHandler)(nil)

type ppsHandler struct {
    cfg    config.TimeSourceConfig
    logger *logrus.Logger

    mu            sync.RWMutex
    running       bool
    status        ConnectionStatus
    pulseCount    uint64
    lastPulseTime time.Time

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

// NewPPSHandler returns a new PPS time source handler.
func NewPPSHandler(cfg config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
    ctx, cancel := context.WithCancel(context.Background())
    h := &ppsHandler{
        cfg:    cfg,
        logger: logger.WithField("protocol", "pps").Logger,
        ctx:    ctx,
        cancel: cancel,
        status: ConnectionStatus{},
    }
    return h, nil
}

// Start launches the background ticker simulating PPS pulses.
func (h *ppsHandler) Start() error {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.running {
        return fmt.Errorf("PPS handler already running")
    }

    h.logger.Info("Starting PPS handler (simulated)")
    h.running = true
    h.status.Connected = true
    h.status.LastActivity = time.Now()

    // Background goroutine that simulates a rising edge every second.
    h.wg.Add(1)
    go func() {
        defer h.wg.Done()
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        for {
            select {
            case <-h.ctx.Done():
                return
            case t := <-ticker.C:
                h.mu.Lock()
                h.pulseCount++
                h.lastPulseTime = t.Truncate(time.Second)
                h.status.LastActivity = t
                h.status.PacketsRx++ // treat each pulse as a packet for stats
                h.mu.Unlock()
            }
        }
    }()

    return nil
}

// Stop terminates the background goroutine.
func (h *ppsHandler) Stop() error {
    h.mu.Lock()
    defer h.mu.Unlock()

    if !h.running {
        return nil
    }

    h.logger.Info("Stopping PPS handler")
    h.running = false
    h.status.Connected = false
    h.cancel()
    h.wg.Wait()

    return nil
}

// GetTimeInfo returns timing information from the last PPS edge.
func (h *ppsHandler) GetTimeInfo() (*TimeInfo, error) {
    h.mu.RLock()
    running := h.running
    pulseTime := h.lastPulseTime
    h.mu.RUnlock()

    if !running {
        return nil, fmt.Errorf("PPS handler not running")
    }

    // If we haven't seen a pulse yet (e.g., immediately after start),
    // wait briefly for one. In simulation we can just fabricate one.
    if pulseTime.IsZero() {
        pulseTime = time.Now().Truncate(time.Second)
    }

    info := &TimeInfo{
        Timestamp: pulseTime,
        Offset:    0,                 // PPS is assumed aligned to the host clock in this mock implementation
        Delay:     0,
        Quality:   255,               // Highest quality as we assume hardware pulse
        Stratum:   1,
        Precision: -20,              // ~1 µs
    }

    return info, nil
}

// GetStatus returns current connection statistics.
func (h *ppsHandler) GetStatus() ConnectionStatus {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.status
}

// GetConfig returns the original configuration.
func (h *ppsHandler) GetConfig() config.TimeSourceConfig {
    return h.cfg
}

// Below are PPSHandler specific methods.

func (h *ppsHandler) GetPulseCount() uint64 {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.pulseCount
}

func (h *ppsHandler) GetLastPulseTime() time.Time {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.lastPulseTime
}

func (h *ppsHandler) EnablePulseOutput() error {
    // Not supported in simulation; simply log.
    h.logger.Warn("EnablePulseOutput called – not supported in simulated handler")
    return nil
}

func (h *ppsHandler) DisablePulseOutput() error {
    h.logger.Warn("DisablePulseOutput called – not supported in simulated handler")
    return nil
}