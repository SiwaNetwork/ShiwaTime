package protocols

import (
	"bufio"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// NMEAHandler реализация обработчика NMEA
type NMEAHandler struct {
	config     config.TimeSourceConfig
	logger     *logrus.Logger
	
	// NMEA специфичные поля
	device     string
	baud       int
	offset     time.Duration
	
	// Состояние
	mu           sync.RWMutex
	running      bool
	stopChan     chan struct{}
	
	// GNSS данные
	gnssStatus   *GNSSStatus
	position     *Position
	lastTime     time.Time
	lastError    error
	
	// Статистика
	packetsRx    uint64
	validPackets uint64
	errorCount   int
	
	// Регулярные выражения для парсинга NMEA
	ggaRegex     *regexp.Regexp
	rmcRegex     *regexp.Regexp
	zdaRegex     *regexp.Regexp
}

// NewNMEAHandler создает новый NMEA обработчик
func NewNMEAHandler(cfg config.TimeSourceConfig, logger *logrus.Logger) (*NMEAHandler, error) {
	handler := &NMEAHandler{
		config:    cfg,
		logger:    logger,
		device:    cfg.Device,
		baud:      cfg.Baud,
		stopChan:  make(chan struct{}),
	}
	
	// Парсим offset
	if cfg.Offset > 0 {
		handler.offset = time.Duration(cfg.Offset) * time.Nanosecond
	}
	
	// Устанавливаем значения по умолчанию
	if handler.device == "" {
		handler.device = "/dev/ttyUSB0"
	}
	if handler.baud == 0 {
		handler.baud = 9600
	}
	
	// Компилируем регулярные выражения
	handler.ggaRegex = regexp.MustCompile(`^\$..GGA,([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*)$`)
	handler.rmcRegex = regexp.MustCompile(`^\$..RMC,([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*)$`)
	handler.zdaRegex = regexp.MustCompile(`^\$..ZDA,([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*),([^,]*)$`)
	
	// Инициализируем GNSS статус
	handler.gnssStatus = &GNSSStatus{}
	handler.position = &Position{}
	
	return handler, nil
}

// Start запускает NMEA обработчик
func (h *NMEAHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("NMEA handler already running")
	}
	
	h.logger.WithFields(logrus.Fields{
		"device": h.device,
		"baud":   h.baud,
	}).Info("Starting NMEA handler")
	
	h.running = true
	
	// Запускаем чтение NMEA данных
	go h.readNMEAData()
	
	return nil
}

// Stop останавливает NMEA обработчик
func (h *NMEAHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping NMEA handler")
	
	close(h.stopChan)
	h.running = false
	
	return nil
}

// GetTimeInfo получает информацию о времени от NMEA
func (h *NMEAHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if !h.running {
		return nil, fmt.Errorf("NMEA handler not running")
	}
	
	if h.lastTime.IsZero() {
		return nil, fmt.Errorf("no valid time received from GNSS")
	}
	
	now := time.Now()
	offset := now.Sub(h.lastTime)
	
	// Применяем конфигурируемый offset
	if h.offset > 0 {
		offset += h.offset
	}
	
	quality := 0
	if h.gnssStatus != nil {
		quality = h.gnssStatus.FixQuality * 50 // Масштабируем качество
	}
	
	return &TimeInfo{
		Timestamp: h.lastTime,
		Offset:    offset,
		Delay:     0, // GNSS не имеет сетевой задержки
		Quality:   quality,
		Stratum:   1,
		Precision: -6, // Микросекундная точность
	}, nil
}

// GetStatus получает статус соединения
func (h *NMEAHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	status := ConnectionStatus{
		Connected:    h.running,
		LastActivity: h.lastTime,
		ErrorCount:   h.errorCount,
		LastError:    h.lastError,
		PacketsRx:    h.packetsRx,
		PacketsTx:    0, // NMEA только принимает
		BytesRx:      h.packetsRx * 50, // Примерная оценка
		BytesTx:      0,
	}
	
	return status
}

// GetConfig получает конфигурацию
func (h *NMEAHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// GetGNSSStatus получает статус GNSS
func (h *NMEAHandler) GetGNSSStatus() *GNSSStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.gnssStatus
}

// GetSatelliteCount получает количество видимых спутников
func (h *NMEAHandler) GetSatelliteCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.gnssStatus != nil {
		return h.gnssStatus.SatellitesVisible
	}
	return 0
}

// GetPosition получает текущую позицию
func (h *NMEAHandler) GetPosition() *Position {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.position
}

// ParseNMEA парсит NMEA сообщение
func (h *NMEAHandler) ParseNMEA(line string) error {
	h.packetsRx++
	
	// Проверяем контрольную сумму
	if !h.validateChecksum(line) {
		h.errorCount++
		return fmt.Errorf("invalid checksum")
	}
	
	// Определяем тип сообщения
	if strings.Contains(line, "GGA") {
		return h.parseGGA(line)
	} else if strings.Contains(line, "RMC") {
		return h.parseRMC(line)
	} else if strings.Contains(line, "ZDA") {
		return h.parseZDA(line)
	}
	
	return nil
}

// readNMEAData читает NMEA данные из устройства
func (h *NMEAHandler) readNMEAData() {
	// Здесь должна быть реализация чтения из последовательного порта
	// Для демонстрации используем симуляцию
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-h.stopChan:
			return
		case <-ticker.C:
			// Симулируем получение NMEA данных
			h.simulateNMEAData()
		}
	}
}

// simulateNMEAData симулирует получение NMEA данных
func (h *NMEAHandler) simulateNMEAData() {
	now := time.Now()
	
	// Симулируем GGA сообщение
	gga := fmt.Sprintf("$GPGGA,%s,5544.1234,N,03736.5678,E,1,08,1.2,156.7,M,45.9,M,,*47",
		now.Format("150405.000"))
	
	if err := h.ParseNMEA(gga); err != nil {
		h.logger.WithError(err).Error("Failed to parse simulated GGA")
	}
	
	// Симулируем RMC сообщение
	rmc := fmt.Sprintf("$GPRMC,%s,A,5544.1234,N,03736.5678,E,0.0,0.0,%s,,,A*56",
		now.Format("150405.000"), now.Format("020106"))
	
	if err := h.ParseNMEA(rmc); err != nil {
		h.logger.WithError(err).Error("Failed to parse simulated RMC")
	}
}

// validateChecksum проверяет контрольную сумму NMEA сообщения
func (h *NMEAHandler) validateChecksum(line string) bool {
	if !strings.Contains(line, "*") {
		return false
	}
	
	parts := strings.Split(line, "*")
	if len(parts) != 2 {
		return false
	}
	
	message := parts[0]
	checksum := parts[1]
	
	// Вычисляем контрольную сумму
	calculated := 0
	for _, char := range message {
		calculated ^= int(char)
	}
	
	expected, err := strconv.ParseInt(checksum, 16, 32)
	if err != nil {
		return false
	}
	
	return calculated == int(expected)
}

// parseGGA парсит GGA сообщение
func (h *NMEAHandler) parseGGA(line string) error {
	matches := h.ggaRegex.FindStringSubmatch(line)
	if len(matches) < 16 {
		return fmt.Errorf("invalid GGA format")
	}
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Обновляем GNSS статус
	if h.gnssStatus == nil {
		h.gnssStatus = &GNSSStatus{}
	}
	
	// Парсим время
	timeStr := matches[1]
	if timeStr != "" {
		if t, err := h.parseTime(timeStr); err == nil {
			h.lastTime = t
		}
	}
	
	// Парсим позицию
	if lat, err := h.parseLatitude(matches[2], matches[3]); err == nil {
		h.position.Latitude = lat
	}
	if lon, err := h.parseLongitude(matches[4], matches[5]); err == nil {
		h.position.Longitude = lon
	}
	
	// Парсим качество фикса
	if fix, err := strconv.Atoi(matches[6]); err == nil {
		h.gnssStatus.FixType = fix
	}
	
	// Парсим количество спутников
	if sats, err := strconv.Atoi(matches[7]); err == nil {
		h.gnssStatus.SatellitesUsed = sats
		h.gnssStatus.SatellitesVisible = sats
	}
	
	// Парсим HDOP
	if hdop, err := strconv.ParseFloat(matches[8], 64); err == nil {
		h.gnssStatus.HDOP = hdop
	}
	
	h.validPackets++
	h.lastError = nil
	
	return nil
}

// parseRMC парсит RMC сообщение
func (h *NMEAHandler) parseRMC(line string) error {
	matches := h.rmcRegex.FindStringSubmatch(line)
	if len(matches) < 13 {
		return fmt.Errorf("invalid RMC format")
	}
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Парсим время и дату
	timeStr := matches[1]
	dateStr := matches[9]
	if timeStr != "" && dateStr != "" {
		if t, err := h.parseDateTime(dateStr, timeStr); err == nil {
			h.lastTime = t
		}
	}
	
	h.validPackets++
	h.lastError = nil
	
	return nil
}

// parseZDA парсит ZDA сообщение
func (h *NMEAHandler) parseZDA(line string) error {
	matches := h.zdaRegex.FindStringSubmatch(line)
	if len(matches) < 8 {
		return fmt.Errorf("invalid ZDA format")
	}
	
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Парсим время и дату
	timeStr := matches[1]
	dayStr := matches[2]
	monthStr := matches[3]
	yearStr := matches[4]
	
	if timeStr != "" && dayStr != "" && monthStr != "" && yearStr != "" {
		if t, err := h.parseZDADateTime(dayStr, monthStr, yearStr, timeStr); err == nil {
			h.lastTime = t
		}
	}
	
	h.validPackets++
	h.lastError = nil
	
	return nil
}

// parseTime парсит время в формате HHMMSS.SSS
func (h *NMEAHandler) parseTime(timeStr string) (time.Time, error) {
	if len(timeStr) < 6 {
		return time.Time{}, fmt.Errorf("invalid time format")
	}
	
	hour, err := strconv.Atoi(timeStr[:2])
	if err != nil {
		return time.Time{}, err
	}
	
	minute, err := strconv.Atoi(timeStr[2:4])
	if err != nil {
		return time.Time{}, err
	}
	
	second, err := strconv.Atoi(timeStr[4:6])
	if err != nil {
		return time.Time{}, err
	}
	
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, time.UTC), nil
}

// parseDateTime парсит дату и время
func (h *NMEAHandler) parseDateTime(dateStr, timeStr string) (time.Time, error) {
	if len(dateStr) != 6 || len(timeStr) < 6 {
		return time.Time{}, fmt.Errorf("invalid date/time format")
	}
	
	day, err := strconv.Atoi(dateStr[:2])
	if err != nil {
		return time.Time{}, err
	}
	
	month, err := strconv.Atoi(dateStr[2:4])
	if err != nil {
		return time.Time{}, err
	}
	
	year, err := strconv.Atoi(dateStr[4:6])
	if err != nil {
		return time.Time{}, err
	}
	year += 2000 // Предполагаем 21 век
	
	t, err := h.parseTime(timeStr)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Date(year, time.Month(month), day, t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
}

// parseZDADateTime парсит дату и время из ZDA
func (h *NMEAHandler) parseZDADateTime(dayStr, monthStr, yearStr, timeStr string) (time.Time, error) {
	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return time.Time{}, err
	}
	
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		return time.Time{}, err
	}
	
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return time.Time{}, err
	}
	
	t, err := h.parseTime(timeStr)
	if err != nil {
		return time.Time{}, err
	}
	
	return time.Date(year, time.Month(month), day, t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
}

// parseLatitude парсит широту
func (h *NMEAHandler) parseLatitude(latStr, dir string) (float64, error) {
	if latStr == "" {
		return 0, fmt.Errorf("empty latitude")
	}
	
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return 0, err
	}
	
	degrees := math.Floor(lat / 100)
	minutes := lat - (degrees * 100)
	latitude := degrees + (minutes / 60)
	
	if dir == "S" {
		latitude = -latitude
	}
	
	return latitude, nil
}

// parseLongitude парсит долготу
func (h *NMEAHandler) parseLongitude(lonStr, dir string) (float64, error) {
	if lonStr == "" {
		return 0, fmt.Errorf("empty longitude")
	}
	
	lon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		return 0, err
	}
	
	degrees := math.Floor(lon / 100)
	minutes := lon - (degrees * 100)
	longitude := degrees + (minutes / 60)
	
	if dir == "W" {
		longitude = -longitude
	}
	
	return longitude, nil
}