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
	"golang.org/x/sys/unix"
)

const (
	// PTP Message Types
	PTPMsgSync         = 0x0
	PTPMsgDelayReq     = 0x1
	PTPMsgPDelayReq    = 0x2
	PTPMsgPDelayResp   = 0x3
	PTPMsgFollowUp     = 0x8
	PTPMsgDelayResp    = 0x9
	PTPMsgPDelayRespFollowUp = 0xA
	PTPMsgAnnounce     = 0xB
	PTPMsgSignaling    = 0xC
	PTPMsgManagement   = 0xD

	// PTP Ports
	PTPEventPort   = 319
	PTPGeneralPort = 320

	// PTP Header size
	PTPHeaderSize = 34

	// Hardware timestamping constants
	SOF_TIMESTAMPING_TX_HARDWARE = 1 << 0
	SOF_TIMESTAMPING_TX_SOFTWARE = 1 << 1
	SOF_TIMESTAMPING_RX_HARDWARE = 1 << 2
	SOF_TIMESTAMPING_RX_SOFTWARE = 1 << 3
	SOF_TIMESTAMPING_SOFTWARE   = 1 << 4
	SOF_TIMESTAMPING_SYS_HARDWARE = 1 << 5
	SOF_TIMESTAMPING_RAW_HARDWARE = 1 << 6
)

// PTPHeader представляет заголовок PTP сообщения
type PTPHeader struct {
	MessageType      uint8
	VersionPTP       uint8
	MessageLength    uint16
	DomainNumber     uint8
	Reserved1        uint8
	FlagField        uint16
	CorrectionField  int64
	Reserved2        uint32
	SourcePortIdentity [10]byte
	SequenceID       uint16
	ControlField     uint8
	LogMessageInterval int8
}

// PTPMessage представляет полное PTP сообщение
type PTPMessage struct {
	Header    PTPHeader
	Timestamp PTPTimestamp
	Data      []byte
}

// PTPTimestamp представляет PTP временную метку
type PTPTimestamp struct {
	SecondsField     uint64
	NanosecondsField uint32
}

// PTPClockIdentity представляет идентификатор PTP часов
type PTPClockIdentity [8]byte

// PTPPortIdentity представляет идентификатор PTP порта
type PTPPortIdentity struct {
	ClockIdentity PTPClockIdentity
	PortNumber    uint16
}

// PTPAnnounceMessage представляет Announce сообщение
type PTPAnnounceMessage struct {
	Header                    PTPHeader
	OriginTimestamp          PTPTimestamp
	CurrentUTCOffset         int16
	Reserved                 uint8
	GrandmasterPriority1     uint8
	GrandmasterClockQuality  PTPClockQuality
	GrandmasterPriority2     uint8
	GrandmasterIdentity      PTPClockIdentity
	StepsRemoved             uint16
	TimeSource               uint8
}

// PTPClockQuality представляет качество PTP часов
type PTPClockQuality struct {
	ClockClass               uint8
	ClockAccuracy            uint8
	OffsetScaledLogVariance  uint16
}

// ptpHandler реализация PTP обработчика
type ptpHandler struct {
	config       config.TimeSourceConfig
	logger       *logrus.Logger
	
	mu           sync.RWMutex
	running      bool
	status       ConnectionStatus
	
	// PTP сокеты
	eventConn    *net.UDPConn
	generalConn  *net.UDPConn
	
	// PTP состояние
	portState    PTPPortState
	clockID      PTPClockIdentity
	portID       PTPPortIdentity
	domain       uint8
	sequenceID   uint16
	
	// Master информация
	masterInfo   *PTPMasterInfo
	
	// Временные метки
	t1, t2, t3, t4 time.Time
	
	// Аппаратные метки времени
	hwTimestamping bool
	phcIndex       int
	
	// Статистика
	syncCount    uint64
	announceCount uint64
	delayReqCount uint64
	
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewPTPHandler создает новый PTP обработчик
func NewPTPHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Генерируем Clock Identity
	clockID := generateClockIdentity()
	
	h := &ptpHandler{
		config:     config,
		logger:     logger,
		clockID:    clockID,
		domain:     uint8(config.Domain),
		portState:  PTPPortStateInitializing,
		ctx:        ctx,
		cancel:     cancel,
		status:     ConnectionStatus{},
	}
	
	// Инициализируем Port Identity
	h.portID = PTPPortIdentity{
		ClockIdentity: clockID,
		PortNumber:    1,
	}
	
	return h, nil
}

// Start запускает PTP обработчик
func (h *ptpHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("PTP handler already running")
	}
	
	h.logger.WithFields(logrus.Fields{
		"domain":    h.domain,
		"interface": h.config.Interface,
	}).Info("Starting PTP handler")
	
	// Настраиваем сокеты
	if err := h.setupSockets(); err != nil {
		return fmt.Errorf("failed to setup PTP sockets: %w", err)
	}
	
	// Проверяем поддержку аппаратных меток времени
	if err := h.checkHardwareTimestamping(); err != nil {
		h.logger.WithError(err).Warn("Hardware timestamping not available, using software timestamps")
		h.hwTimestamping = false
	} else {
		h.logger.Info("Hardware timestamping enabled")
		h.hwTimestamping = true
	}
	
	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()
	h.portState = PTPPortStateListening
	
	// Запускаем обработчики
	go h.handleEventMessages()
	go h.handleGeneralMessages()
	go h.sendDelayRequests()
	
	return nil
}

// Stop останавливает PTP обработчик
func (h *ptpHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping PTP handler")
	
	h.cancel()
	h.running = false
	h.status.Connected = false
	h.portState = PTPPortStateDisabled
	
	if h.eventConn != nil {
		h.eventConn.Close()
		h.eventConn = nil
	}
	
	if h.generalConn != nil {
		h.generalConn.Close()
		h.generalConn = nil
	}
	
	return nil
}

// GetTimeInfo получает информацию о времени от PTP
func (h *ptpHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	running := h.running
	masterInfo := h.masterInfo
	t1, t2, t3, t4 := h.t1, h.t2, h.t3, h.t4
	h.mu.RUnlock()
	
	if !running {
		return nil, fmt.Errorf("PTP handler not running")
	}
	
	if masterInfo == nil || t1.IsZero() || t2.IsZero() || t3.IsZero() || t4.IsZero() {
		return nil, fmt.Errorf("insufficient timing data for PTP calculation")
	}
	
	// Вычисляем offset и delay
	// offset = ((t2 - t1) + (t3 - t4)) / 2
	// delay = (t4 - t1) - (t3 - t2)
	offset := (t2.Sub(t1) + t3.Sub(t4)) / 2
	delay := t4.Sub(t1) - t3.Sub(t2)
	
	// Определяем качество на основе clock class
	quality := 255 - masterInfo.ClockClass
	if quality < 0 {
		quality = 0
	}
	
	info := &TimeInfo{
		Timestamp: t3,
		Offset:    offset,
		Delay:     delay,
		Quality:   quality,
		Precision: -20, // Наносекундная точность
	}
	
	h.logger.WithFields(logrus.Fields{
		"offset":      offset,
		"delay":       delay,
		"clock_class": masterInfo.ClockClass,
		"hw_ts":       h.hwTimestamping,
	}).Debug("PTP time calculation")
	
	return info, nil
}

// GetStatus возвращает статус соединения
func (h *ptpHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.status
}

// GetConfig возвращает конфигурацию
func (h *ptpHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// GetGNSSInfo возвращает GNSS информацию (PTP не поддерживает GNSS напрямую)
func (h *ptpHandler) GetGNSSInfo() GNSSStatus {
	return GNSSStatus{
		FixType:         0, // No fix
		FixQuality:      0,
		SatellitesUsed:  0,
		SatellitesVisible: 0,
		HDOP:            0,
		VDOP:            0,
	}
}

// GetClockIdentity возвращает clock identity
func (h *ptpHandler) GetClockIdentity() string {
	return fmt.Sprintf("%x", h.clockID)
}

// GetDomain возвращает PTP домен
func (h *ptpHandler) GetDomain() int {
	return int(h.domain)
}

// GetPortState возвращает состояние PTP порта
func (h *ptpHandler) GetPortState() PTPPortState {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.portState
}

// GetMasterInfo возвращает информацию о мастере
func (h *ptpHandler) GetMasterInfo() *PTPMasterInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.masterInfo
}

// setupSockets настраивает UDP сокеты для PTP
func (h *ptpHandler) setupSockets() error {
	var err error
	
	// Event socket (port 319)
	eventAddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: PTPEventPort,
	}
	
	h.eventConn, err = net.ListenUDP("udp4", eventAddr)
	if err != nil {
		return fmt.Errorf("failed to create event socket: %w", err)
	}
	
	// General socket (port 320)
	generalAddr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: PTPGeneralPort,
	}
	
	h.generalConn, err = net.ListenUDP("udp4", generalAddr)
	if err != nil {
		h.eventConn.Close()
		return fmt.Errorf("failed to create general socket: %w", err)
	}
	
	// Настраиваем сокеты для multicast
	if err := h.configureMulticast(h.eventConn); err != nil {
		h.logger.WithError(err).Warn("Failed to configure multicast for event socket")
	}
	
	if err := h.configureMulticast(h.generalConn); err != nil {
		h.logger.WithError(err).Warn("Failed to configure multicast for general socket")
	}
	
	return nil
}

// configureMulticast настраивает multicast для сокета
func (h *ptpHandler) configureMulticast(conn *net.UDPConn) error {
	// Получаем file descriptor
	file, err := conn.File()
	if err != nil {
		return err
	}
	defer file.Close()
	
	fd := int(file.Fd())
	
	// Присоединяемся к PTP multicast группе
	mreq := &unix.IPMreq{
		Multiaddr: [4]byte{224, 0, 1, 129}, // 224.0.1.129
		Interface: [4]byte{0, 0, 0, 0},     // INADDR_ANY
	}
	
	return unix.SetsockoptIPMreq(fd, unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP, mreq)
}

// checkHardwareTimestamping проверяет поддержку аппаратных меток времени
func (h *ptpHandler) checkHardwareTimestamping() error {
	if h.config.Interface == "" {
		return fmt.Errorf("interface not specified")
	}
	
	// Получаем file descriptor event сокета
	file, err := h.eventConn.File()
	if err != nil {
		return err
	}
	defer file.Close()
	
	fd := int(file.Fd())
	
	// Проверяем поддержку SO_TIMESTAMPING
	flags := SOF_TIMESTAMPING_TX_HARDWARE | SOF_TIMESTAMPING_RX_HARDWARE | SOF_TIMESTAMPING_RAW_HARDWARE
	
	err = unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING, flags)
	if err != nil {
		return fmt.Errorf("failed to enable hardware timestamping: %w", err)
	}
	
	// Пытаемся получить PHC index
	h.phcIndex, err = h.getPHCIndex()
	if err != nil {
		h.logger.WithError(err).Warn("Could not get PHC index")
		h.phcIndex = -1
	}
	
	return nil
}

// getPHCIndex получает индекс PHC для интерфейса
func (h *ptpHandler) getPHCIndex() (int, error) {
	// Реализация получения PHC индекса через ethtool
	// Это упрощенная версия - в реальности нужно использовать ioctl
	return 0, nil
}

// handleEventMessages обрабатывает event сообщения (Sync, Delay_Req, etc.)
func (h *ptpHandler) handleEventMessages() {
	buffer := make([]byte, 1500)
	
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
			// Устанавливаем timeout для чтения
			h.eventConn.SetReadDeadline(time.Now().Add(time.Second))
			
			n, addr, err := h.eventConn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				h.logger.WithError(err).Error("Error reading from event socket")
				continue
			}
			
			if n < PTPHeaderSize {
				continue
			}
			
			msg, err := h.parseMessage(buffer[:n])
			if err != nil {
				h.logger.WithError(err).Debug("Failed to parse PTP message")
				continue
			}
			
			if msg.Header.DomainNumber != h.domain {
				continue
			}
			
			h.processEventMessage(msg, addr)
		}
	}
}

// handleGeneralMessages обрабатывает general сообщения (Announce, Follow_Up, etc.)
func (h *ptpHandler) handleGeneralMessages() {
	buffer := make([]byte, 1500)
	
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
			h.generalConn.SetReadDeadline(time.Now().Add(time.Second))
			
			n, addr, err := h.generalConn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				h.logger.WithError(err).Error("Error reading from general socket")
				continue
			}
			
			if n < PTPHeaderSize {
				continue
			}
			
			msg, err := h.parseMessage(buffer[:n])
			if err != nil {
				h.logger.WithError(err).Debug("Failed to parse PTP message")
				continue
			}
			
			if msg.Header.DomainNumber != h.domain {
				continue
			}
			
			h.processGeneralMessage(msg, addr)
		}
	}
}

// processEventMessage обрабатывает event сообщения
func (h *ptpHandler) processEventMessage(msg *PTPMessage, addr *net.UDPAddr) {
	switch msg.Header.MessageType & 0x0F {
	case PTPMsgSync:
		h.processSyncMessage(msg, addr)
	case PTPMsgDelayResp:
		h.processDelayRespMessage(msg, addr)
	}
}

// processGeneralMessage обрабатывает general сообщения
func (h *ptpHandler) processGeneralMessage(msg *PTPMessage, addr *net.UDPAddr) {
	switch msg.Header.MessageType & 0x0F {
	case PTPMsgAnnounce:
		h.processAnnounceMessage(msg, addr)
	case PTPMsgFollowUp:
		h.processFollowUpMessage(msg, addr)
	}
}

// processSyncMessage обрабатывает Sync сообщение
func (h *ptpHandler) processSyncMessage(msg *PTPMessage, addr *net.UDPAddr) {
	h.mu.Lock()
	h.syncCount++
	h.t2 = time.Now() // t2 - время получения Sync
	h.mu.Unlock()
	
	h.logger.WithFields(logrus.Fields{
		"seq_id": msg.Header.SequenceID,
		"addr":   addr,
	}).Debug("Received Sync message")
	
	h.status.PacketsRx++
	h.status.LastActivity = time.Now()
}

// processFollowUpMessage обрабатывает Follow_Up сообщение
func (h *ptpHandler) processFollowUpMessage(msg *PTPMessage, addr *net.UDPAddr) {
	if len(msg.Data) >= 10 {
		// Извлекаем precise origin timestamp
		seconds := binary.BigEndian.Uint64(msg.Data[0:6])
		nanoseconds := binary.BigEndian.Uint32(msg.Data[6:10])
		
		// t1 - точное время отправки Sync с мастера
		h.mu.Lock()
		h.t1 = time.Unix(int64(seconds), int64(nanoseconds))
		h.mu.Unlock()
		
		h.logger.WithFields(logrus.Fields{
			"seq_id": msg.Header.SequenceID,
			"t1":     h.t1,
		}).Debug("Received Follow_Up message")
	}
}

// processAnnounceMessage обрабатывает Announce сообщение
func (h *ptpHandler) processAnnounceMessage(msg *PTPMessage, addr *net.UDPAddr) {
	if len(msg.Data) < 20 {
		return
	}
	
	announce := &PTPAnnounceMessage{}
	
	// Парсим Announce данные
	announce.CurrentUTCOffset = int16(binary.BigEndian.Uint16(msg.Data[10:12]))
	announce.GrandmasterPriority1 = msg.Data[13]
	announce.GrandmasterClockQuality.ClockClass = msg.Data[14]
	announce.GrandmasterClockQuality.ClockAccuracy = msg.Data[15]
	announce.GrandmasterClockQuality.OffsetScaledLogVariance = binary.BigEndian.Uint16(msg.Data[16:18])
	announce.GrandmasterPriority2 = msg.Data[18]
	copy(announce.GrandmasterIdentity[:], msg.Data[19:27])
	announce.StepsRemoved = binary.BigEndian.Uint16(msg.Data[27:29])
	announce.TimeSource = msg.Data[29]
	
	// Обновляем информацию о мастере
	h.mu.Lock()
	h.masterInfo = &PTPMasterInfo{
		ClockIdentity:           fmt.Sprintf("%x", announce.GrandmasterIdentity),
		ClockClass:              int(announce.GrandmasterClockQuality.ClockClass),
		ClockAccuracy:           int(announce.GrandmasterClockQuality.ClockAccuracy),
		OffsetScaledLogVariance: int(announce.GrandmasterClockQuality.OffsetScaledLogVariance),
		Priority1:               int(announce.GrandmasterPriority1),
		Priority2:               int(announce.GrandmasterPriority2),
		TimeSource:              int(announce.TimeSource),
		StepsRemoved:            int(announce.StepsRemoved),
		SourcePortIdentity:      fmt.Sprintf("%x:%d", msg.Header.SourcePortIdentity[:8], binary.BigEndian.Uint16(msg.Header.SourcePortIdentity[8:10])),
	}
	h.announceCount++
	h.portState = PTPPortStateSlave
	h.mu.Unlock()
	
	h.logger.WithFields(logrus.Fields{
		"master_id":    fmt.Sprintf("%x", announce.GrandmasterIdentity),
		"clock_class":  announce.GrandmasterClockQuality.ClockClass,
		"priority1":    announce.GrandmasterPriority1,
		"priority2":    announce.GrandmasterPriority2,
		"steps_removed": announce.StepsRemoved,
	}).Debug("Received Announce message")
}

// processDelayRespMessage обрабатывает Delay_Resp сообщение
func (h *ptpHandler) processDelayRespMessage(msg *PTPMessage, addr *net.UDPAddr) {
	if len(msg.Data) >= 10 {
		// Извлекаем receive timestamp
		seconds := binary.BigEndian.Uint64(msg.Data[0:6])
		nanoseconds := binary.BigEndian.Uint32(msg.Data[6:10])
		
		// t3 - время получения Delay_Req на мастере
		h.mu.Lock()
		h.t3 = time.Unix(int64(seconds), int64(nanoseconds))
		h.mu.Unlock()
		
		h.logger.WithFields(logrus.Fields{
			"seq_id": msg.Header.SequenceID,
			"t3":     h.t3,
		}).Debug("Received Delay_Resp message")
	}
}

// sendDelayRequests отправляет Delay_Req сообщения
func (h *ptpHandler) sendDelayRequests() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			if h.portState == PTPPortStateSlave {
				h.sendDelayReq()
			}
		}
	}
}

// sendDelayReq отправляет Delay_Req сообщение
func (h *ptpHandler) sendDelayReq() error {
	h.mu.Lock()
	seqID := h.sequenceID
	h.sequenceID++
	h.mu.Unlock()
	
	msg := &PTPMessage{
		Header: PTPHeader{
			MessageType:    PTPMsgDelayReq,
			VersionPTP:     0x02,
			MessageLength:  44,
			DomainNumber:   h.domain,
			FlagField:      0x0000,
			CorrectionField: 0,
			SequenceID:     seqID,
			ControlField:   0x01,
			LogMessageInterval: 0x7F,
		},
	}
	
	// Копируем Source Port Identity
	copy(msg.Header.SourcePortIdentity[:8], h.clockID[:])
	binary.BigEndian.PutUint16(msg.Header.SourcePortIdentity[8:10], h.portID.PortNumber)
	
	// Сериализуем сообщение
	data, err := h.serializeMessage(msg)
	if err != nil {
		return err
	}
	
	// Отправляем в multicast
	multicastAddr := &net.UDPAddr{
		IP:   net.IPv4(224, 0, 1, 129),
		Port: PTPEventPort,
	}
	
	h.mu.Lock()
	h.t4 = time.Now() // t4 - время отправки Delay_Req
	h.mu.Unlock()
	
	_, err = h.eventConn.WriteToUDP(data, multicastAddr)
	if err != nil {
		h.logger.WithError(err).Error("Failed to send Delay_Req")
		return err
	}
	
	h.mu.Lock()
	h.delayReqCount++
	h.mu.Unlock()
	
	h.status.PacketsTx++
	
	h.logger.WithFields(logrus.Fields{
		"seq_id": seqID,
		"t4":     h.t4,
	}).Debug("Sent Delay_Req message")
	
	return nil
}

// parseMessage парсит PTP сообщение
func (h *ptpHandler) parseMessage(data []byte) (*PTPMessage, error) {
	if len(data) < PTPHeaderSize {
		return nil, fmt.Errorf("message too short")
	}
	
	msg := &PTPMessage{}
	
	// Парсим заголовок
	msg.Header.MessageType = data[0] & 0x0F
	msg.Header.VersionPTP = data[1] & 0x0F
	msg.Header.MessageLength = binary.BigEndian.Uint16(data[2:4])
	msg.Header.DomainNumber = data[4]
	msg.Header.Reserved1 = data[5]
	msg.Header.FlagField = binary.BigEndian.Uint16(data[6:8])
	msg.Header.CorrectionField = int64(binary.BigEndian.Uint64(data[8:16]))
	msg.Header.Reserved2 = binary.BigEndian.Uint32(data[16:20])
	copy(msg.Header.SourcePortIdentity[:], data[20:30])
	msg.Header.SequenceID = binary.BigEndian.Uint16(data[30:32])
	msg.Header.ControlField = data[32]
	msg.Header.LogMessageInterval = int8(data[33])
	
	// Копируем данные после заголовка
	if len(data) > PTPHeaderSize {
		msg.Data = make([]byte, len(data)-PTPHeaderSize)
		copy(msg.Data, data[PTPHeaderSize:])
	}
	
	return msg, nil
}

// serializeMessage сериализует PTP сообщение
func (h *ptpHandler) serializeMessage(msg *PTPMessage) ([]byte, error) {
	data := make([]byte, msg.Header.MessageLength)
	
	// Сериализуем заголовок
	data[0] = msg.Header.MessageType & 0x0F
	data[1] = msg.Header.VersionPTP & 0x0F
	binary.BigEndian.PutUint16(data[2:4], msg.Header.MessageLength)
	data[4] = msg.Header.DomainNumber
	data[5] = msg.Header.Reserved1
	binary.BigEndian.PutUint16(data[6:8], msg.Header.FlagField)
	binary.BigEndian.PutUint64(data[8:16], uint64(msg.Header.CorrectionField))
	binary.BigEndian.PutUint32(data[16:20], msg.Header.Reserved2)
	copy(data[20:30], msg.Header.SourcePortIdentity[:])
	binary.BigEndian.PutUint16(data[30:32], msg.Header.SequenceID)
	data[32] = msg.Header.ControlField
	data[33] = byte(msg.Header.LogMessageInterval)
	
	// Добавляем timestamp для Delay_Req
	if msg.Header.MessageType == PTPMsgDelayReq {
		// Origin Timestamp (пустой для Delay_Req)
		for i := 34; i < 44; i++ {
			data[i] = 0
		}
	}
	
	return data, nil
}

// SendAnnounce отправляет Announce сообщение (для master режима)
func (h *ptpHandler) SendAnnounce() error {
	// Реализация отправки Announce сообщения
	// Пока не реализована - нужна для master режима
	return fmt.Errorf("master mode not implemented")
}

// SendSync отправляет Sync сообщение (для master режима)
func (h *ptpHandler) SendSync() error {
	// Реализация отправки Sync сообщения
	// Пока не реализована - нужна для master режима
	return fmt.Errorf("master mode not implemented")
}

// HandleMessage обрабатывает входящее PTP сообщение
func (h *ptpHandler) HandleMessage(msgData []byte) error {
	msg, err := h.parseMessage(msgData)
	if err != nil {
		return err
	}
	
	// Обрабатываем сообщение в зависимости от типа
	switch msg.Header.MessageType & 0x0F {
	case PTPMsgSync:
		h.processSyncMessage(msg, nil)
	case PTPMsgAnnounce:
		h.processAnnounceMessage(msg, nil)
	case PTPMsgFollowUp:
		h.processFollowUpMessage(msg, nil)
	case PTPMsgDelayResp:
		h.processDelayRespMessage(msg, nil)
	}
	
	return nil
}

// generateClockIdentity генерирует Clock Identity
func generateClockIdentity() PTPClockIdentity {
	var clockID PTPClockIdentity
	
	// Получаем MAC адрес или генерируем случайный ID
	// Это упрощенная версия - в реальности нужно использовать MAC адрес интерфейса
	for i := range clockID {
		clockID[i] = byte(time.Now().UnixNano() >> (i * 8))
	}
	
	return clockID
}