package clock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
	"github.com/shiwatime/shiwatime/internal/metrics"
	"github.com/shiwatime/shiwatime/internal/protocols"
)

// ClockState представляет состояние системных часов
type ClockState int

const (
	ClockStateUnknown ClockState = iota
	ClockStateSynchronized
	ClockStateUnsynchronized
	ClockStateFreeRunning
	ClockStateHoldover
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
	default:
		return "unknown"
	}
}

// TimeSource представляет источник времени
type TimeSource struct {
	ID       string
	Protocol string
	Config   config.TimeSourceConfig
	Handler  protocols.TimeSourceHandler
	Status   SourceStatus
	Metrics  *SourceMetrics
}

// SourceStatus статус источника времени
type SourceStatus struct {
	Active         bool
	LastSync       time.Time
	Offset         time.Duration
	Quality        int
	ErrorCount     int
	LastError      error
	Selected       bool
	Priority       int
}

// SourceMetrics метрики источника времени
type SourceMetrics struct {
	PacketsReceived uint64
	PacketsSent     uint64
	SyncCount       uint64
	ErrorCount      uint64
	OffsetHistory   []time.Duration
	DelayHistory    []time.Duration
}

// Manager управляет синхронизацией системных часов
type Manager struct {
	config        *config.Config
	logger        *logrus.Logger
	metricsClient *metrics.Client
	
	// Источники времени
	primarySources   []*TimeSource
	secondarySources []*TimeSource
	
	// Состояние
	mu           sync.RWMutex
	state        ClockState
	selectedSource *TimeSource
	lastAdjustment time.Time
	stepLimit      time.Duration
	
	// Контроль
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager создает новый менеджер часов
func NewManager(cfg *config.Config, logger *logrus.Logger, metricsClient *metrics.Client) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	stepLimit, err := parseDuration(cfg.ShiwaTime.ClockSync.StepLimit)
	if err != nil {
		stepLimit = 15 * time.Minute // значение по умолчанию
	}
	
	m := &Manager{
		config:        cfg,
		logger:        logger,
		metricsClient: metricsClient,
		state:         ClockStateUnknown,
		stepLimit:     stepLimit,
		ctx:           ctx,
		cancel:        cancel,
	}
	
	// Инициализируем источники времени
	if err := m.initTimeSources(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize time sources: %w", err)
	}
	
	return m, nil
}

// Start запускает менеджер часов
func (m *Manager) Start() error {
	m.logger.Info("Starting clock manager")
	
	// Запускаем обработчики источников времени
	for _, source := range m.primarySources {
		if !source.Config.Disable {
			m.wg.Add(1)
			go m.runTimeSource(source)
		}
	}
	
	for _, source := range m.secondarySources {
		if !source.Config.Disable {
			m.wg.Add(1)
			go m.runTimeSource(source)
		}
	}
	
	// Запускаем основной цикл управления часами
	m.wg.Add(1)
	go m.clockManagementLoop()
	
	// Запускаем цикл отправки метрик
	m.wg.Add(1)
	go m.metricsLoop()
	
	return nil
}

// Stop останавливает менеджер часов
func (m *Manager) Stop() error {
	m.logger.Info("Stopping clock manager")
	
	m.cancel()
	m.wg.Wait()
	
	// Останавливаем все источники времени
	for _, source := range append(m.primarySources, m.secondarySources...) {
		if source.Handler != nil {
			source.Handler.Stop()
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

// GetSelectedSource возвращает выбранный источник времени
func (m *Manager) GetSelectedSource() *TimeSource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.selectedSource
}

// GetSources возвращает все источники времени
func (m *Manager) GetSources() ([]*TimeSource, []*TimeSource) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.primarySources, m.secondarySources
}

// initTimeSources инициализирует источники времени
func (m *Manager) initTimeSources() error {
	// Инициализируем первичные источники
	for i, sourceConfig := range m.config.ShiwaTime.ClockSync.PrimaryClocks {
		source, err := m.createTimeSource(fmt.Sprintf("primary_%d", i), sourceConfig, true)
		if err != nil {
			return fmt.Errorf("failed to create primary source %d: %w", i, err)
		}
		m.primarySources = append(m.primarySources, source)
	}
	
	// Инициализируем вторичные источники
	for i, sourceConfig := range m.config.ShiwaTime.ClockSync.SecondaryClocks {
		source, err := m.createTimeSource(fmt.Sprintf("secondary_%d", i), sourceConfig, false)
		if err != nil {
			return fmt.Errorf("failed to create secondary source %d: %w", i, err)
		}
		m.secondarySources = append(m.secondarySources, source)
	}
	
	return nil
}

// createTimeSource создает источник времени
func (m *Manager) createTimeSource(id string, sourceConfig config.TimeSourceConfig, isPrimary bool) (*TimeSource, error) {
	handler, err := protocols.CreateHandler(sourceConfig.Protocol, sourceConfig, m.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create handler for protocol %s: %w", sourceConfig.Protocol, err)
	}
	
	priority := 100 // базовый приоритет
	if isPrimary {
		priority = 200
	}
	
	source := &TimeSource{
		ID:       id,
		Protocol: sourceConfig.Protocol,
		Config:   sourceConfig,
		Handler:  handler,
		Status: SourceStatus{
			Priority: priority,
		},
		Metrics: &SourceMetrics{},
	}
	
	return source, nil
}

// runTimeSource запускает обработку источника времени
func (m *Manager) runTimeSource(source *TimeSource) {
	defer m.wg.Done()
	
	m.logger.WithFields(logrus.Fields{
		"source_id": source.ID,
		"protocol":  source.Protocol,
	}).Info("Starting time source")
	
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.processTimeSource(source)
		}
	}
}

// processTimeSource обрабатывает обновления от источника времени
func (m *Manager) processTimeSource(source *TimeSource) {
	if source.Config.Disable || source.Config.MonitorOnly {
		return
	}
	
	// Получаем информацию о времени от источника
	timeInfo, err := source.Handler.GetTimeInfo()
	if err != nil {
		m.mu.Lock()
		source.Status.LastError = err
		source.Status.ErrorCount++
		source.Status.Active = false
		m.mu.Unlock()
		
		source.Metrics.ErrorCount++
		
		m.logger.WithFields(logrus.Fields{
			"source_id": source.ID,
			"error":     err,
		}).Warn("Failed to get time info from source")
		return
	}
	
	// Обновляем статус источника
	m.mu.Lock()
	source.Status.Active = true
	source.Status.LastSync = time.Now()
	source.Status.Offset = timeInfo.Offset
	source.Status.Quality = timeInfo.Quality
	source.Status.LastError = nil
	m.mu.Unlock()
	
	// Обновляем метрики
	source.Metrics.SyncCount++
	source.Metrics.PacketsReceived++
	
	// Сохраняем историю смещений
	if len(source.Metrics.OffsetHistory) >= 100 {
		source.Metrics.OffsetHistory = source.Metrics.OffsetHistory[1:]
	}
	source.Metrics.OffsetHistory = append(source.Metrics.OffsetHistory, timeInfo.Offset)
	
	m.logger.WithFields(logrus.Fields{
		"source_id": source.ID,
		"offset":    timeInfo.Offset,
		"quality":   timeInfo.Quality,
	}).Debug("Received time info from source")
}

// clockManagementLoop основной цикл управления часами
func (m *Manager) clockManagementLoop() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.manageClock()
		}
	}
}

// manageClock управляет синхронизацией системных часов
func (m *Manager) manageClock() {
	// Выбираем лучший источник времени
	selectedSource := m.selectBestSource()
	
	m.mu.Lock()
	previousSelected := m.selectedSource
	m.selectedSource = selectedSource
	
	// Обновляем флаги выбранности
	for _, source := range append(m.primarySources, m.secondarySources...) {
		source.Status.Selected = (source == selectedSource)
	}
	m.mu.Unlock()
	
	if selectedSource == nil {
		m.setState(ClockStateUnsynchronized)
		return
	}
	
	// Логируем смену источника
	if previousSelected != selectedSource {
		m.logger.WithFields(logrus.Fields{
			"new_source": selectedSource.ID,
			"protocol":   selectedSource.Protocol,
		}).Info("Selected new time source")
	}
	
	// Применяем коррекцию времени
	if m.config.ShiwaTime.ClockSync.AdjustClock {
		m.adjustSystemClock(selectedSource)
	}
	
	m.setState(ClockStateSynchronized)
}

// selectBestSource выбирает лучший доступный источник времени
func (m *Manager) selectBestSource() *TimeSource {
	var bestSource *TimeSource
	bestScore := -1
	
	// Сначала проверяем первичные источники
	for _, source := range m.primarySources {
		if !source.Status.Active || source.Config.Disable || source.Config.MonitorOnly {
			continue
		}
		
		score := m.calculateSourceScore(source)
		if score > bestScore {
			bestScore = score
			bestSource = source
		}
	}
	
	// Если нет активных первичных источников, проверяем вторичные
	if bestSource == nil {
		for _, source := range m.secondarySources {
			if !source.Status.Active || source.Config.Disable || source.Config.MonitorOnly {
				continue
			}
			
			score := m.calculateSourceScore(source)
			if score > bestScore {
				bestScore = score
				bestSource = source
			}
		}
	}
	
	return bestSource
}

// calculateSourceScore вычисляет счет качества источника времени
func (m *Manager) calculateSourceScore(source *TimeSource) int {
	score := source.Status.Priority
	
	// Вычитаем баллы за ошибки
	score -= source.Status.ErrorCount * 10
	
	// Добавляем баллы за качество
	score += source.Status.Quality
	
	// Вычитаем баллы за большие смещения
	offsetMs := source.Status.Offset.Milliseconds()
	if offsetMs < 0 {
		offsetMs = -offsetMs
	}
	score -= int(offsetMs)
	
	return score
}

// adjustSystemClock корректирует системные часы
func (m *Manager) adjustSystemClock(source *TimeSource) {
	offset := source.Status.Offset
	
	// Проверяем ограничения на коррекцию
	if offset.Abs() > m.stepLimit {
		m.logger.WithFields(logrus.Fields{
			"offset":     offset,
			"step_limit": m.stepLimit,
		}).Warn("Offset exceeds step limit, clock adjustment skipped")
		return
	}
	
	// Применяем коррекцию (здесь была бы реальная системная коррекция)
	m.logger.WithFields(logrus.Fields{
		"offset":    offset,
		"source_id": source.ID,
	}).Info("Adjusting system clock")
	
	m.lastAdjustment = time.Now()
}

// setState устанавливает состояние часов
func (m *Manager) setState(state ClockState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.state != state {
		m.logger.WithFields(logrus.Fields{
			"old_state": m.state.String(),
			"new_state": state.String(),
		}).Info("Clock state changed")
		m.state = state
	}
}

// metricsLoop отправляет метрики
func (m *Manager) metricsLoop() {
	defer m.wg.Done()
	
	if m.metricsClient == nil {
		return
	}
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.sendMetrics()
		}
	}
}

// sendMetrics отправляет метрики в Elasticsearch
func (m *Manager) sendMetrics() {
	m.mu.RLock()
	state := m.state
	selectedSource := m.selectedSource
	sources := append(m.primarySources, m.secondarySources...)
	m.mu.RUnlock()
	
	// Отправляем общие метрики
	doc := map[string]interface{}{
		"@timestamp":      time.Now(),
		"clock_state":     state.String(),
		"selected_source": "",
	}
	
	if selectedSource != nil {
		doc["selected_source"] = selectedSource.ID
	}
	
	m.metricsClient.SendMetric("shiwatime_clock", doc)
	
	// Отправляем метрики источников
	for _, source := range sources {
		sourceDoc := map[string]interface{}{
			"@timestamp":       time.Now(),
			"source_id":        source.ID,
			"protocol":         source.Protocol,
			"active":           source.Status.Active,
			"selected":         source.Status.Selected,
			"offset_ns":        source.Status.Offset.Nanoseconds(),
			"quality":          source.Status.Quality,
			"error_count":      source.Status.ErrorCount,
			"packets_received": source.Metrics.PacketsReceived,
			"sync_count":       source.Metrics.SyncCount,
		}
		
		if source.Status.LastError != nil {
			sourceDoc["last_error"] = source.Status.LastError.Error()
		}
		
		m.metricsClient.SendMetric("shiwatime_source", sourceDoc)
	}
}

// parseDuration парсит строку длительности с поддержкой дней
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}
	
	// Поддерживаем дни
	if len(s) > 1 && s[len(s)-1] == 'd' {
		days := s[:len(s)-1]
		d, err := time.ParseDuration(days + "h")
		if err != nil {
			return 0, err
		}
		return d * 24, nil
	}
	
	return time.ParseDuration(s)
}