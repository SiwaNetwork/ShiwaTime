package protocols

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// NTPPacket представляет NTP пакет
type NTPPacket struct {
	Settings       uint8  // LI, VN, Mode
	Stratum        uint8  // Stratum уровень
	Poll           uint8  // Интервал опроса
	Precision      uint8  // Точность
	RootDelay      uint32 // Корневая задержка
	RootDispersion uint32 // Корневая дисперсия
	ReferenceID    uint32 // Reference ID
	RefTimeSec     uint32 // Reference timestamp seconds
	RefTimeFrac    uint32 // Reference timestamp fraction
	OrigTimeSec    uint32 // Origin timestamp seconds
	OrigTimeFrac   uint32 // Origin timestamp fraction
	RxTimeSec      uint32 // Receive timestamp seconds
	RxTimeFrac     uint32 // Receive timestamp fraction
	TxTimeSec      uint32 // Transmit timestamp seconds
	TxTimeFrac     uint32 // Transmit timestamp fraction
}

// ntpHandler реализация NTP обработчика
type ntpHandler struct {
	config       config.TimeSourceConfig
	logger       *logrus.Logger
	
	mu           sync.RWMutex
	running      bool
	status       ConnectionStatus
	conn         *net.UDPConn
	
	// NTP специфичные поля
	stratum      int
	referenceID  string
	rootDelay    time.Duration
	rootDispersion time.Duration
	
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewNTPHandler создает новый NTP обработчик
func NewNTPHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	h := &ntpHandler{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		status: ConnectionStatus{},
	}
	
	return h, nil
}

// Start запускает NTP обработчик
func (h *ntpHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("NTP handler already running")
	}
	
	h.logger.WithFields(logrus.Fields{
		"server": h.config.IP,
	}).Info("Starting NTP handler")
	
	// Устанавливаем соединение
	serverAddr, err := net.ResolveUDPAddr("udp", h.config.IP+":123")
	if err != nil {
		return fmt.Errorf("failed to resolve NTP server address: %w", err)
	}
	
	h.conn, err = net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to NTP server: %w", err)
	}
	
	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()
	
	return nil
}

// Stop останавливает NTP обработчик
func (h *ntpHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping NTP handler")
	
	h.cancel()
	h.running = false
	h.status.Connected = false
	
	if h.conn != nil {
		h.conn.Close()
		h.conn = nil
	}
	
	return nil
}

// GetTimeInfo получает информацию о времени от NTP сервера
func (h *ntpHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	running := h.running
	conn := h.conn
	h.mu.RUnlock()
	
	if !running || conn == nil {
		return nil, fmt.Errorf("NTP handler not running")
	}
	
	// Отправляем NTP запрос
	req := &NTPPacket{
		Settings: 0x1B, // LI=0, VN=3, Mode=3 (client)
	}
	
	t1 := time.Now()
	
	err := h.sendPacket(req)
	if err != nil {
		h.mu.Lock()
		h.status.ErrorCount++
		h.status.LastError = err
		h.mu.Unlock()
		return nil, fmt.Errorf("failed to send NTP request: %w", err)
	}
	
	// Получаем ответ
	resp, t4, err := h.receivePacket()
	if err != nil {
		h.mu.Lock()
		h.status.ErrorCount++
		h.status.LastError = err
		h.mu.Unlock()
		return nil, fmt.Errorf("failed to receive NTP response: %w", err)
	}
	
	// Обновляем статус
	h.mu.Lock()
	h.status.LastActivity = time.Now()
	h.status.PacketsRx++
	h.status.PacketsTx++
	h.status.LastError = nil
	h.stratum = int(resp.Stratum)
	h.rootDelay = ntpToTime(resp.RootDelay, 0).Sub(time.Unix(0, 0))
	h.rootDispersion = ntpToTime(resp.RootDispersion, 0).Sub(time.Unix(0, 0))
	h.mu.Unlock()
	
	// Вычисляем времена
	t2 := ntpToTime(resp.RxTimeSec, resp.RxTimeFrac)   // Время получения на сервере
	t3 := ntpToTime(resp.TxTimeSec, resp.TxTimeFrac)   // Время отправки с сервера
	
	// Вычисляем смещение и задержку
	// offset = ((t2 - t1) + (t3 - t4)) / 2
	// delay = (t4 - t1) - (t3 - t2)
	offset := (t2.Sub(t1) + t3.Sub(t4)) / 2
	delay := t4.Sub(t1) - t3.Sub(t2)
	
	// Определяем качество на основе stratum
	quality := 255 - int(resp.Stratum)*10
	if quality < 0 {
		quality = 0
	}
	
	info := &TimeInfo{
		Timestamp: t3,
		Offset:    offset,
		Delay:     delay,
		Quality:   quality,
		Stratum:   int(resp.Stratum),
		Precision: int(resp.Precision),
	}
	
	h.logger.WithFields(logrus.Fields{
		"server":    h.config.IP,
		"offset":    offset,
		"delay":     delay,
		"stratum":   resp.Stratum,
		"precision": resp.Precision,
	}).Debug("Received NTP response")
	
	return info, nil
}

// GetStatus возвращает статус соединения
func (h *ntpHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

// GetConfig возвращает конфигурацию
func (h *ntpHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// GetStratum возвращает stratum сервера
func (h *ntpHandler) GetStratum() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stratum
}

// GetReferenceID возвращает reference ID
func (h *ntpHandler) GetReferenceID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.referenceID
}

// GetRootDelay возвращает root delay
func (h *ntpHandler) GetRootDelay() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rootDelay
}

// GetRootDispersion возвращает root dispersion
func (h *ntpHandler) GetRootDispersion() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rootDispersion
}

// SendRequest отправляет NTP запрос
func (h *ntpHandler) SendRequest() error {
	req := &NTPPacket{
		Settings: 0x1B, // LI=0, VN=3, Mode=3 (client)
	}
	
	return h.sendPacket(req)
}

// ParseResponse парсит NTP ответ
func (h *ntpHandler) ParseResponse(data []byte) (*TimeInfo, error) {
	if len(data) != 48 {
		return nil, fmt.Errorf("invalid NTP packet size: %d", len(data))
	}
	
	resp := &NTPPacket{}
	if err := h.parsePacket(data, resp); err != nil {
		return nil, err
	}
	
	// Простое извлечение времени без вычисления смещения
	t3 := ntpToTime(resp.TxTimeSec, resp.TxTimeFrac)
	
	info := &TimeInfo{
		Timestamp: t3,
		Offset:    0, // Нужны дополнительные измерения для точного вычисления
		Delay:     0,
		Quality:   255 - int(resp.Stratum)*10,
		Stratum:   int(resp.Stratum),
		Precision: int(resp.Precision),
	}
	
	return info, nil
}

// sendPacket отправляет NTP пакет
func (h *ntpHandler) sendPacket(packet *NTPPacket) error {
	data := make([]byte, 48)
	
	data[0] = packet.Settings
	data[1] = packet.Stratum
	data[2] = packet.Poll
	data[3] = packet.Precision
	
	binary.BigEndian.PutUint32(data[4:8], packet.RootDelay)
	binary.BigEndian.PutUint32(data[8:12], packet.RootDispersion)
	binary.BigEndian.PutUint32(data[12:16], packet.ReferenceID)
	
	// Устанавливаем время передачи
	now := time.Now()
	sec, frac := timeToNtp(now)
	binary.BigEndian.PutUint32(data[40:44], sec)
	binary.BigEndian.PutUint32(data[44:48], frac)
	
	_, err := h.conn.Write(data)
	return err
}

// receivePacket получает NTP пакет
func (h *ntpHandler) receivePacket() (*NTPPacket, time.Time, error) {
	buffer := make([]byte, 48)
	
	h.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := h.conn.Read(buffer)
	receiveTime := time.Now()
	
	if err != nil {
		return nil, receiveTime, err
	}
	
	if n != 48 {
		return nil, receiveTime, fmt.Errorf("invalid NTP packet size: %d", n)
	}
	
	packet := &NTPPacket{}
	if err := h.parsePacket(buffer, packet); err != nil {
		return nil, receiveTime, err
	}
	
	return packet, receiveTime, nil
}

// parsePacket парсит данные в NTP пакет
func (h *ntpHandler) parsePacket(data []byte, packet *NTPPacket) error {
	if len(data) != 48 {
		return fmt.Errorf("invalid packet size")
	}
	
	packet.Settings = data[0]
	packet.Stratum = data[1]
	packet.Poll = data[2]
	packet.Precision = data[3]
	
	packet.RootDelay = binary.BigEndian.Uint32(data[4:8])
	packet.RootDispersion = binary.BigEndian.Uint32(data[8:12])
	packet.ReferenceID = binary.BigEndian.Uint32(data[12:16])
	
	packet.RefTimeSec = binary.BigEndian.Uint32(data[16:20])
	packet.RefTimeFrac = binary.BigEndian.Uint32(data[20:24])
	
	packet.OrigTimeSec = binary.BigEndian.Uint32(data[24:28])
	packet.OrigTimeFrac = binary.BigEndian.Uint32(data[28:32])
	
	packet.RxTimeSec = binary.BigEndian.Uint32(data[32:36])
	packet.RxTimeFrac = binary.BigEndian.Uint32(data[36:40])
	
	packet.TxTimeSec = binary.BigEndian.Uint32(data[40:44])
	packet.TxTimeFrac = binary.BigEndian.Uint32(data[44:48])
	
	return nil
}

// timeToNtp конвертирует время в NTP формат
func timeToNtp(t time.Time) (uint32, uint32) {
	// NTP timestamp начинается с 1 января 1900
	const ntpEpochOffset = 2208988800
	
	sec := uint32(t.Unix() + ntpEpochOffset)
	frac := uint32((t.UnixNano() % 1e9) * (1 << 32) / 1e9)
	
	return sec, frac
}

// ntpToTime конвертирует NTP формат в время
func ntpToTime(sec, frac uint32) time.Time {
	const ntpEpochOffset = 2208988800
	
	unixSec := int64(sec) - ntpEpochOffset
	unixNano := int64(frac) * 1e9 / (1 << 32)
	
	return time.Unix(unixSec, unixNano)
}