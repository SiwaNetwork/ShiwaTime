package clock

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
	"github.com/shiwatime/shiwatime/internal/protocols"
	"golang.org/x/sys/unix"
)

// ClockState представляет состояние системных часов
type ClockState int

const (
	ClockStateUnknown ClockState = iota
	ClockStateSynchronized
	ClockStateUnsynchronized
	ClockStateFreeRunning
	ClockStateHoldover
	ClockStateStepping
	ClockStateSynchronizing
)

func (cs ClockState) String() string {
	switch cs {
	case ClockStateSynchronized:
		return "synchronized"
	case ClockStateUnsynchronized:
		return "unsynchronized"
	case ClockStateFreeRunning:
		return "free_running"
	case ClockStateHoldover:
		return "holdover"
	case ClockStateStepping:
		return "stepping"
	case ClockStateSynchronizing:
		return "synchronizing"
	default:
		return "unknown"
	}
}

// PIDController implements a PID controller for clock discipline
type PIDController struct {
	Kp, Ki, Kd       float64  // PID gains
	integral         float64  // Integral term accumulator
	prevError        float64  // Previous error for derivative
	integralLimit    float64  // Integral windup limit
	outputLimit      float64  // Output saturation limit
	lastTime         time.Time
}

// NewPIDController creates a new PID controller
func NewPIDController(kp, ki, kd, integralLimit, outputLimit float64) *PIDController {
	return &PIDController{
		Kp:            kp,
		Ki:            ki, 
		Kd:            kd,
		integralLimit: integralLimit,
		outputLimit:   outputLimit,
		lastTime:      time.Now(),
	}
}

// Update calculates PID controller output
func (pid *PIDController) Update(error, dt float64) float64 {
	// Proportional term
	p := pid.Kp * error
	
	// Integral term with windup protection
	pid.integral += error * dt
	if pid.integral > pid.integralLimit {
		pid.integral = pid.integralLimit
	} else if pid.integral < -pid.integralLimit {
		pid.integral = -pid.integralLimit
	}
	i := pid.Ki * pid.integral
	
	// Derivative term
	d := 0.0
	if dt > 0 {
		d = pid.Kd * (error - pid.prevError) / dt
	}
	pid.prevError = error
	
	// Calculate output with saturation
	output := p + i + d
	if output > pid.outputLimit {
		output = pid.outputLimit
	} else if output < -pid.outputLimit {
		output = -pid.outputLimit
	}
	
	return output
}

// Reset resets the PID controller state
func (pid *PIDController) Reset() {
	pid.integral = 0
	pid.prevError = 0
	pid.lastTime = time.Now()
}

// Manager manages time sources and clock synchronization
type Manager struct {
	config        config.ShiwaTimeConfig
	logger        *logrus.Logger
	
	mu            sync.RWMutex
	running       bool
	sources       map[string]protocols.TimeSourceHandler
	selectedSource protocols.TimeSourceHandler // Currently selected time source
	state         ClockState
	
	// PID controller state
	pidController *PIDController
	
	// Statistics and filtering
	offsetHistory    []time.Duration
	delayHistory     []time.Duration
	jitterHistory    []time.Duration
	filterWindow     int
	
	// Clock discipline parameters
	sigma            float64  // Allan deviation threshold
	rho              float64  // Correlation threshold
	
	// Frequency correction
	freqOffset       float64  // Current frequency offset in ppb
	freqDrift        float64  // Frequency drift rate
	
	// Kernel discipline
	kernelSync       bool
	
	ctx    context.Context
	cancel context.CancelFunc
}

// NewManager создает новый менеджер часов
func NewManager(config config.ShiwaTimeConfig, logger *logrus.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize PID controller with default values
	pidController := NewPIDController(
		1.0,   // KP
		0.1,   // KI  
		0.01,  // KD
		100.0, // Integrator limit
		1000000, // 1 second output limit
	)
	
	return &Manager{
		config:        config,
		logger:        logger,
		state:         ClockStateUnknown,
		sources:       make(map[string]protocols.TimeSourceHandler),
		pidController: pidController,
		filterWindow:  50,    // Default filter window
		sigma:         1e-6,  // Default sigma threshold
		rho:           0.8,   // Default rho threshold
		kernelSync:    true,  // Default kernel sync
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start запускает менеджер часов
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		return fmt.Errorf("clock manager already running")
	}
	
	m.logger.Info("Starting clock manager")
	m.running = true
	
	// Инициализируем источники времени (объединяем primary и secondary)
	allSources := append(m.config.ClockSync.PrimaryClocks, m.config.ClockSync.SecondaryClocks...)
	for i, sourceConfig := range allSources {
		name := fmt.Sprintf("source_%d", i)
		
		handler, err := protocols.NewTimeSourceHandler(sourceConfig, m.logger)
		if err != nil {
			m.logger.WithError(err).Errorf("Failed to create handler for source %d", i)
			continue
		}
		
		if err := handler.Start(); err != nil {
			m.logger.WithError(err).Errorf("Failed to start source %d", i)
			continue
		}
		
		m.sources[name] = handler
		m.logger.WithField("source", name).Info("Time source started")
	}
	
	// Запускаем цикл синхронизации
	go m.syncLoop()
	
	return nil
}

// Stop останавливает менеджер часов
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.running {
		return nil
	}
	
	m.logger.Info("Stopping clock manager")
	
	m.cancel()
	m.running = false
	
	// Останавливаем все источники времени
	for name, handler := range m.sources {
		if err := handler.Stop(); err != nil {
			m.logger.WithError(err).WithField("source", name).Error("Failed to stop time source")
		}
	}
	
	return nil
}

// GetState возвращает текущее состояние часов
func (m *Manager) GetState() ClockState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// GetSources возвращает источники времени
func (m *Manager) GetSources() map[string]protocols.TimeSourceHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	sources := make(map[string]protocols.TimeSourceHandler)
	for name, handler := range m.sources {
		sources[name] = handler
	}
	return sources
}

// GetSelectedSource возвращает текущий выбранный источник времени
func (m *Manager) GetSelectedSource() protocols.TimeSourceHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.selectedSource
}

// GetSourcesByPriority возвращает источники, разделенные на первичные и вторичные
func (m *Manager) GetSourcesByPriority() (map[string]protocols.TimeSourceHandler, map[string]protocols.TimeSourceHandler) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	primary := make(map[string]protocols.TimeSourceHandler)
	secondary := make(map[string]protocols.TimeSourceHandler)
	
	for name, handler := range m.sources {
		config := handler.GetConfig()
		// Use weight as priority indicator: higher weight = primary
		if config.Weight >= 5 { // High weight sources (5+) are primary
			primary[name] = handler
		} else { // Lower weight sources (<5) are secondary
			secondary[name] = handler
		}
	}
	
	return primary, secondary
}

// syncLoop основной цикл синхронизации
func (m *Manager) syncLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.synchronizeClock(); err != nil {
				m.logger.WithError(err).Debug("Clock synchronization failed")
			}
		}
	}
}

// synchronizeClock выполняет синхронизацию часов
func (m *Manager) synchronizeClock() error {
	source := m.selectBestSource()
	if source == nil {
		m.mu.Lock()
		m.selectedSource = nil
		m.state = ClockStateUnsynchronized
		m.mu.Unlock()
		return fmt.Errorf("no suitable time source available")
	}
	
	// Update selected source
	m.mu.Lock()
	m.selectedSource = source
	m.mu.Unlock()
	
	timeInfo, err := source.GetTimeInfo()
	if err != nil {
		return err
	}
	
	// Обновляем статистику
	m.updateStatistics(timeInfo)
	
	// Проверяем нужно ли делать step или adjustment
	offset := timeInfo.Offset
	stepThreshold := 500 * time.Millisecond // Default threshold
	
	if math.Abs(float64(offset)) > float64(stepThreshold) {
		return m.stepClock(offset)
	}
	
	// Используем PID контроллер для плавной подстройки
	return m.adjustClockPID(offset)
}

// selectBestSource выбирает лучший источник времени
func (m *Manager) selectBestSource() protocols.TimeSourceHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var bestHandler protocols.TimeSourceHandler
	var bestScore float64
	
	for _, handler := range m.sources {
		status := handler.GetStatus()
		if !status.Connected {
			continue
		}
		
		timeInfo, err := handler.GetTimeInfo()
		if err != nil {
			continue
		}
		
		// Простой scoring algorithm
		score := float64(timeInfo.Quality)
		config := handler.GetConfig()
		score *= float64(config.Weight)
		
		if score > bestScore {
			bestScore = score
			bestHandler = handler
		}
	}
	
	return bestHandler
}

// updateStatistics обновляет статистику времени
func (m *Manager) updateStatistics(info *protocols.TimeInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Add to history with sliding window
	m.offsetHistory = append(m.offsetHistory, info.Offset)
	m.delayHistory = append(m.delayHistory, info.Delay)
	
	// Calculate jitter
	if len(m.offsetHistory) > 1 {
		prevOffset := m.offsetHistory[len(m.offsetHistory)-2]
		jitter := time.Duration(math.Abs(float64(info.Offset - prevOffset)))
		m.jitterHistory = append(m.jitterHistory, jitter)
	}
	
	// Maintain window size
	if len(m.offsetHistory) > m.filterWindow {
		m.offsetHistory = m.offsetHistory[1:]
	}
	if len(m.delayHistory) > m.filterWindow {
		m.delayHistory = m.delayHistory[1:]
	}
	if len(m.jitterHistory) > m.filterWindow {
		m.jitterHistory = m.jitterHistory[1:]
	}
}

// stepClock делает step системных часов
func (m *Manager) stepClock(offset time.Duration) error {
	m.logger.WithField("offset", offset).Info("Stepping system clock")
	
	if m.kernelSync {
		// Use adjtimex to step the clock
		var timex unix.Timex
		timex.Modes = unix.ADJ_SETOFFSET
		
		if offset >= 0 {
			timex.Time.Sec = int64(offset / time.Second)
			timex.Time.Usec = int64((offset % time.Second) / time.Microsecond)
		} else {
			negOffset := -offset
			timex.Time.Sec = -int64(negOffset / time.Second)
			timex.Time.Usec = -int64((negOffset % time.Second) / time.Microsecond)
		}
		
		_, err := unix.Adjtimex(&timex)
		if err != nil {
			return fmt.Errorf("failed to step clock: %w", err)
		}
	}
	
	m.state = ClockStateStepping
	m.pidController.Reset() // Reset PID after step
	
	return nil
}

// adjustClockPID подстраивает часы используя PID контроллер
func (m *Manager) adjustClockPID(offset time.Duration) error {
	now := time.Now()
	dt := now.Sub(m.pidController.lastTime).Seconds()
	m.pidController.lastTime = now
	
	if dt <= 0 {
		return nil
	}
	
	// Convert offset to seconds for PID calculation
	errorSeconds := float64(offset) / float64(time.Second)
	
	// Calculate PID output (frequency adjustment in ppb)
	freqAdjustment := m.pidController.Update(errorSeconds, dt)
	
	// Apply frequency adjustment
	if m.kernelSync {
		err := m.adjustKernelFrequency(freqAdjustment)
		if err != nil {
			return err
		}
	}
	
	m.freqOffset = freqAdjustment
	m.state = ClockStateSynchronizing
	
	m.logger.WithFields(logrus.Fields{
		"offset":           offset,
		"freq_adjustment":  freqAdjustment,
		"error_seconds":    errorSeconds,
		"dt":               dt,
	}).Debug("PID clock adjustment")
	
	return nil
}

// adjustKernelFrequency подстраивает частоту ядра
func (m *Manager) adjustKernelFrequency(ppb float64) error {
	var timex unix.Timex
	timex.Modes = unix.ADJ_FREQUENCY
	
	// Convert ppb to kernel frequency units (2^-16 ppm)
	timex.Freq = int64(ppb * 65536 / 1000000)
	
	_, err := unix.Adjtimex(&timex)
	if err != nil {
		return fmt.Errorf("failed to adjust kernel frequency: %w", err)
	}
	
	return nil
}

// GetStatistics возвращает статистику часов
func (m *Manager) GetStatistics() ClockStatistics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := ClockStatistics{
		State:         m.state,
		FreqOffset:    m.freqOffset,
		FreqDrift:     m.freqDrift,
		KernelSync:    m.kernelSync,
		SourceCount:   len(m.sources),
	}
	
	if len(m.offsetHistory) > 0 {
		stats.MeanOffset = m.calculateMean(m.offsetHistory)
		stats.MaxOffset = m.calculateMax(m.offsetHistory)
		stats.MinOffset = m.calculateMin(m.offsetHistory)
	}
	
	if len(m.delayHistory) > 0 {
		stats.MeanDelay = m.calculateMean(m.delayHistory)
	}
	
	if len(m.jitterHistory) > 0 {
		stats.MeanJitter = m.calculateMean(m.jitterHistory)
	}
	
	stats.AllanDeviation = m.calculateAllanDeviation()
	stats.Stable = len(m.offsetHistory) > 10 && stats.AllanDeviation < m.sigma
	
	return stats
}

// Helper functions for statistics
func (m *Manager) calculateMean(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	
	var sum time.Duration
	for _, v := range values {
		sum += v
	}
	return sum / time.Duration(len(values))
}

func (m *Manager) calculateMax(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	
	max := values[0]
	for _, v := range values[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

func (m *Manager) calculateMin(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	
	min := values[0]
	for _, v := range values[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

func (m *Manager) calculateAllanDeviation() float64 {
	if len(m.offsetHistory) < 3 {
		return 0
	}
	
	n := len(m.offsetHistory)
	sum := 0.0
	
	for i := 0; i < n-2; i++ {
		x1 := float64(m.offsetHistory[i])
		x2 := float64(m.offsetHistory[i+1])
		x3 := float64(m.offsetHistory[i+2])
		
		diff := (x3 - 2*x2 + x1) / 2.0
		sum += diff * diff
	}
	
	variance := sum / float64(n-2)
	return math.Sqrt(variance / 2.0)
}

// ClockStatistics содержит статистику работы часов
type ClockStatistics struct {
	State           ClockState    `json:"state"`
	FreqOffset      float64       `json:"freq_offset"`
	FreqDrift       float64       `json:"freq_drift"`
	KernelSync      bool          `json:"kernel_sync"`
	SourceCount     int           `json:"source_count"`
	
	// Statistics
	MeanOffset      time.Duration `json:"mean_offset"`
	MaxOffset       time.Duration `json:"max_offset"`
	MinOffset       time.Duration `json:"min_offset"`
	MeanDelay       time.Duration `json:"mean_delay"`
	MeanJitter      time.Duration `json:"mean_jitter"`
	AllanDeviation  float64       `json:"allan_deviation"`
	Stable          bool          `json:"stable"`
}

