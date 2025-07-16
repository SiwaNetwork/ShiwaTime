package protocols

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

const (
	// PTP+Squared protocol constants
	PTPSquaredProtocolID = "/ptpsquared/1.0.0"
	PTPSquaredTopic      = "ptpsquared-sync"
	
	// Message types
	MsgTypeTimeSync    = "time_sync"
	MsgTypeSeatRequest = "seat_request"
	MsgTypeSeatOffer   = "seat_offer"
	MsgTypeSeatAccept  = "seat_accept"
	MsgTypeSeatReject  = "seat_reject"
	MsgTypeHeartbeat   = "heartbeat"
)

// PTPSquaredMessage представляет сообщение PTP+Squared
type PTPSquaredMessage struct {
	Type      string                 `json:"type"`
	PeerID    string                 `json:"peer_id"`
	Timestamp time.Time              `json:"timestamp"`
	Domain    int                    `json:"domain,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ptpsquaredHandler реализация PTP+Squared обработчика
type ptpsquaredHandler struct {
	config       config.TimeSourceConfig
	logger       *logrus.Logger
	
	mu           sync.RWMutex
	running      bool
	status       ConnectionStatus
	
	// libp2p components
	host         host.Host
	pubsub       *pubsub.PubSub
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
	
	// PTP+Squared specific
	peerID       string
	domains      []int
	seatsToOffer int
	seatsToFill  int
	concurrentSources int
	capabilities []string
	preferenceScore int
	reservations []string
	
	// Connected peers
	connectedPeers map[string]*PeerInfo
	seatRequests   map[string]*SeatRequest
	seatOffers     map[string]*SeatOffer
	
	// Time sync data
	timeSources   map[string]*TimeInfo
	networkStats  *PTPSquaredNetworkStats
	
	ctx          context.Context
	cancel       context.CancelFunc
}

// PeerInfo информация о пире
type PeerInfo struct {
	ID            string
	Domains       []int
	Capabilities  []string
	PreferenceScore int
	LastSeen      time.Time
	Latency       time.Duration
	Quality       float64
}

// SeatRequest запрос слота
type SeatRequest struct {
	RequestID string
	FromPeer  string
	ToPeer    string
	Domain    int
	Timestamp time.Time
	Status    string // pending, accepted, rejected
}

// SeatOffer предложение слота
type SeatOffer struct {
	OfferID   string
	FromPeer  string
	ToPeer    string
	Domain    int
	Timestamp time.Time
	Status    string // pending, accepted, rejected
}

// NewPTPSquaredHandler создает новый PTP+Squared обработчик
func NewPTPSquaredHandler(config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Генерируем приватный ключ
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, rand.Reader)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	
	// Создаем libp2p хост
	host, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.Security(noise.ID, noise.New),
		libp2p.EnableAutoRelay(),
		libp2p.EnableNATService(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}
	
	h := &ptpsquaredHandler{
		config:     config,
		logger:     logger,
		host:       host,
		peerID:     host.ID().String(),
		domains:    []int{115, 116}, // Default domains
		seatsToOffer: 4,
		seatsToFill:   3,
		concurrentSources: 1,
		capabilities: []string{"hqosc-1500"},
		preferenceScore: 0,
		reservations: []string{"1500:50%:115,116", "750:25%"},
		connectedPeers: make(map[string]*PeerInfo),
		seatRequests:   make(map[string]*SeatRequest),
		seatOffers:     make(map[string]*SeatOffer),
		timeSources:    make(map[string]*TimeInfo),
		networkStats:   &PTPSquaredNetworkStats{},
		ctx:        ctx,
		cancel:     cancel,
		status:     ConnectionStatus{},
	}
	
	// Настраиваем pubsub
	if err := h.setupPubSub(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup pubsub: %w", err)
	}
	
	// Настраиваем mDNS discovery
	if err := h.setupDiscovery(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to setup discovery: %w", err)
	}
	
	return h, nil
}

// Start запускает PTP+Squared обработчик
func (h *ptpsquaredHandler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if h.running {
		return fmt.Errorf("PTP+Squared handler already running")
	}
	
	h.logger.WithFields(logrus.Fields{
		"peer_id": h.peerID,
		"domains": h.domains,
	}).Info("Starting PTP+Squared handler")
	
	// Запускаем обработку сообщений
	go h.handleMessages()
	
	// Запускаем периодические задачи
	go h.periodicTasks()
	
	h.running = true
	h.status.Connected = true
	h.status.LastActivity = time.Now()
	
	return nil
}

// Stop останавливает PTP+Squared обработчик
func (h *ptpsquaredHandler) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if !h.running {
		return nil
	}
	
	h.logger.Info("Stopping PTP+Squared handler")
	
	h.cancel()
	
	if h.subscription != nil {
		h.subscription.Cancel()
	}
	
	if h.topic != nil {
		h.topic.Close()
	}
	
	if h.host != nil {
		h.host.Close()
	}
	
	h.running = false
	h.status.Connected = false
	
	return nil
}

// GetTimeInfo получает информацию о времени
func (h *ptpsquaredHandler) GetTimeInfo() (*TimeInfo, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// Выбираем лучший источник времени из подключенных пиров
	var bestSource *TimeInfo
	var bestQuality float64
	
	for _, timeInfo := range h.timeSources {
		quality := h.calculateQuality(timeInfo)
		if quality > bestQuality {
			bestQuality = quality
			bestSource = timeInfo
		}
	}
	
	if bestSource == nil {
		return nil, fmt.Errorf("no time sources available")
	}
	
	return bestSource, nil
}

// GetStatus получает статус соединения
func (h *ptpsquaredHandler) GetStatus() ConnectionStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// Обновляем статистику
	h.status.PacketsRx = uint64(len(h.connectedPeers))
	h.status.PacketsTx = uint64(len(h.seatRequests) + len(h.seatOffers))
	
	return h.status
}

// GetConfig получает конфигурацию
func (h *ptpsquaredHandler) GetConfig() config.TimeSourceConfig {
	return h.config
}

// GetGNSSInfo возвращает GNSS информацию (PTP+Squared не поддерживает GNSS напрямую)
func (h *ptpsquaredHandler) GetGNSSInfo() GNSSStatus {
	return GNSSStatus{
		FixType:         0, // No fix
		FixQuality:      0,
		SatellitesUsed:  0,
		SatellitesVisible: 0,
		HDOP:            0,
		VDOP:            0,
	}
}

// GetPeerID получает ID пира
func (h *ptpsquaredHandler) GetPeerID() string {
	return h.peerID
}

// GetDomains получает поддерживаемые домены
func (h *ptpsquaredHandler) GetDomains() []int {
	return h.domains
}

// GetSeatsToOffer получает количество предлагаемых слотов
func (h *ptpsquaredHandler) GetSeatsToOffer() int {
	return h.seatsToOffer
}

// GetSeatsToFill получает количество заполняемых слотов
func (h *ptpsquaredHandler) GetSeatsToFill() int {
	return h.seatsToFill
}

// GetConcurrentSources получает количество одновременных источников
func (h *ptpsquaredHandler) GetConcurrentSources() int {
	return h.concurrentSources
}

// GetCapabilities получает возможности узла
func (h *ptpsquaredHandler) GetCapabilities() []string {
	return h.capabilities
}

// GetPreferenceScore получает предпочтительный балл
func (h *ptpsquaredHandler) GetPreferenceScore() int {
	return h.preferenceScore
}

// GetReservations получает резервирования
func (h *ptpsquaredHandler) GetReservations() []string {
	return h.reservations
}

// GetConnectedPeers получает список подключенных пиров
func (h *ptpsquaredHandler) GetConnectedPeers() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	peers := make([]string, 0, len(h.connectedPeers))
	for peerID := range h.connectedPeers {
		peers = append(peers, peerID)
	}
	
	return peers
}

// GetNetworkStats получает статистику сети
func (h *ptpsquaredHandler) GetNetworkStats() *PTPSquaredNetworkStats {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// Обновляем статистику
	h.networkStats.TotalPeers = len(h.connectedPeers)
	h.networkStats.ActivePeers = 0
	h.networkStats.TotalSeatsOffered = len(h.seatOffers)
	h.networkStats.TotalSeatsFilled = len(h.seatRequests)
	
	var totalLatency time.Duration
	var totalJitter time.Duration
	var totalQuality float64
	count := 0
	
	for _, peer := range h.connectedPeers {
		if time.Since(peer.LastSeen) < 30*time.Second {
			h.networkStats.ActivePeers++
		}
		totalLatency += peer.Latency
		totalQuality += peer.Quality
		count++
	}
	
	if count > 0 {
		h.networkStats.AverageLatency = totalLatency / time.Duration(count)
		h.networkStats.AverageJitter = totalJitter / time.Duration(count)
		h.networkStats.NetworkQuality = totalQuality / float64(count)
	}
	
	return h.networkStats
}

// RequestSeat запрашивает слот у другого узла
func (h *ptpsquaredHandler) RequestSeat(peerID string, domain int) error {
	requestID := fmt.Sprintf("%s-%d-%d", h.peerID, domain, time.Now().Unix())
	
	msg := &PTPSquaredMessage{
		Type:      MsgTypeSeatRequest,
		PeerID:    h.peerID,
		Timestamp: time.Now(),
		Domain:    domain,
		Data: map[string]interface{}{
			"request_id": requestID,
			"capabilities": h.capabilities,
			"preference_score": h.preferenceScore,
		},
	}
	
	if err := h.publishMessage(msg); err != nil {
		return fmt.Errorf("failed to publish seat request: %w", err)
	}
	
	// Сохраняем запрос
	h.mu.Lock()
	h.seatRequests[requestID] = &SeatRequest{
		RequestID: requestID,
		FromPeer:  h.peerID,
		ToPeer:    peerID,
		Domain:    domain,
		Timestamp: time.Now(),
		Status:    "pending",
	}
	h.mu.Unlock()
	
	h.logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"peer_id":    peerID,
		"domain":     domain,
	}).Info("Sent seat request")
	
	return nil
}

// OfferSeat предлагает слот другому узлу
func (h *ptpsquaredHandler) OfferSeat(peerID string, domain int) error {
	offerID := fmt.Sprintf("%s-%d-%d", h.peerID, domain, time.Now().Unix())
	
	msg := &PTPSquaredMessage{
		Type:      MsgTypeSeatOffer,
		PeerID:    h.peerID,
		Timestamp: time.Now(),
		Domain:    domain,
		Data: map[string]interface{}{
			"offer_id": offerID,
			"capabilities": h.capabilities,
			"preference_score": h.preferenceScore,
		},
	}
	
	if err := h.publishMessage(msg); err != nil {
		return fmt.Errorf("failed to publish seat offer: %w", err)
	}
	
	// Сохраняем предложение
	h.mu.Lock()
	h.seatOffers[offerID] = &SeatOffer{
		OfferID:   offerID,
		FromPeer:  h.peerID,
		ToPeer:    peerID,
		Domain:    domain,
		Timestamp: time.Now(),
		Status:    "pending",
	}
	h.mu.Unlock()
	
	h.logger.WithFields(logrus.Fields{
		"offer_id": offerID,
		"peer_id":  peerID,
		"domain":   domain,
	}).Info("Sent seat offer")
	
	return nil
}

// HandleTimeSync обрабатывает синхронизацию времени
func (h *ptpsquaredHandler) HandleTimeSync(peerID string, timeInfo *TimeInfo) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Сохраняем информацию о времени от пира
	h.timeSources[peerID] = timeInfo
	
	// Обновляем информацию о пире
	if peer, exists := h.connectedPeers[peerID]; exists {
		peer.LastSeen = time.Now()
		peer.Quality = float64(timeInfo.Quality) / 255.0
	}
	
	h.logger.WithFields(logrus.Fields{
		"peer_id": peerID,
		"offset":  timeInfo.Offset,
		"quality": timeInfo.Quality,
	}).Debug("Received time sync from peer")
	
	return nil
}

// setupPubSub настраивает pubsub
func (h *ptpsquaredHandler) setupPubSub() error {
	ps, err := pubsub.NewGossipSub(h.ctx, h.host)
	if err != nil {
		return fmt.Errorf("failed to create pubsub: %w", err)
	}
	h.pubsub = ps
	
	// Подписываемся на топик
	topic, err := ps.Join(PTPSquaredTopic)
	if err != nil {
		return fmt.Errorf("failed to join topic: %w", err)
	}
	h.topic = topic
	
	// Подписываемся на сообщения
	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}
	h.subscription = sub
	
	return nil
}

// setupDiscovery настраивает обнаружение узлов
func (h *ptpsquaredHandler) setupDiscovery() error {
	// Настраиваем mDNS discovery
	mdns.NewMdnsService(h.host, "ptpsquared", h)
	
	// Обработчик подключений
	h.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(net network.Network, conn network.Conn) {
			peerID := conn.RemotePeer().String()
			h.logger.WithField("peer_id", peerID).Info("Peer connected")
			
			h.mu.Lock()
			h.connectedPeers[peerID] = &PeerInfo{
				ID:       peerID,
				LastSeen: time.Now(),
			}
			h.mu.Unlock()
		},
		DisconnectedF: func(net network.Network, conn network.Conn) {
			peerID := conn.RemotePeer().String()
			h.logger.WithField("peer_id", peerID).Info("Peer disconnected")
			
			h.mu.Lock()
			delete(h.connectedPeers, peerID)
			delete(h.timeSources, peerID)
			h.mu.Unlock()
		},
	})
	
	return nil
}

// handleMessages обрабатывает входящие сообщения
func (h *ptpsquaredHandler) handleMessages() {
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
			msg, err := h.subscription.Next(h.ctx)
			if err != nil {
				h.logger.WithError(err).Error("Failed to get next message")
				continue
			}
			
			if msg.ReceivedFrom == h.host.ID() {
				continue // Игнорируем собственные сообщения
			}
			
			h.processMessage(msg)
		}
	}
}

// processMessage обрабатывает сообщение
func (h *ptpsquaredHandler) processMessage(msg *pubsub.Message) {
	var ptpMsg PTPSquaredMessage
	if err := json.Unmarshal(msg.Data, &ptpMsg); err != nil {
		h.logger.WithError(err).Error("Failed to unmarshal message")
		return
	}
	
	h.logger.WithFields(logrus.Fields{
		"type":    ptpMsg.Type,
		"peer_id": ptpMsg.PeerID,
	}).Debug("Received message")
	
	switch ptpMsg.Type {
	case MsgTypeTimeSync:
		h.handleTimeSyncMessage(&ptpMsg)
	case MsgTypeSeatRequest:
		h.handleSeatRequestMessage(&ptpMsg)
	case MsgTypeSeatOffer:
		h.handleSeatOfferMessage(&ptpMsg)
	case MsgTypeSeatAccept:
		h.handleSeatAcceptMessage(&ptpMsg)
	case MsgTypeSeatReject:
		h.handleSeatRejectMessage(&ptpMsg)
	case MsgTypeHeartbeat:
		h.handleHeartbeatMessage(&ptpMsg)
	}
}

// handleTimeSyncMessage обрабатывает сообщение синхронизации времени
func (h *ptpsquaredHandler) handleTimeSyncMessage(msg *PTPSquaredMessage) {
	// Извлекаем информацию о времени из данных
	if timeData, ok := msg.Data["time_info"].(map[string]interface{}); ok {
		timeInfo := &TimeInfo{
			Timestamp: msg.Timestamp,
			Quality:   255, // Максимальное качество для PTP+Squared
		}
		
		if offset, ok := timeData["offset"].(float64); ok {
			timeInfo.Offset = time.Duration(offset)
		}
		
		if delay, ok := timeData["delay"].(float64); ok {
			timeInfo.Delay = time.Duration(delay)
		}
		
		h.HandleTimeSync(msg.PeerID, timeInfo)
	}
}

// handleSeatRequestMessage обрабатывает запрос слота
func (h *ptpsquaredHandler) handleSeatRequestMessage(msg *PTPSquaredMessage) {
	requestID, _ := msg.Data["request_id"].(string)
	
	h.logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"peer_id":    msg.PeerID,
		"domain":     msg.Domain,
	}).Info("Received seat request")
	
	// Проверяем, можем ли мы предложить слот
	if h.canOfferSeat(msg.Domain) {
		// Отправляем предложение
		h.OfferSeat(msg.PeerID, msg.Domain)
	} else {
		// Отправляем отказ
		rejectMsg := &PTPSquaredMessage{
			Type:      MsgTypeSeatReject,
			PeerID:    h.peerID,
			Timestamp: time.Now(),
			Domain:    msg.Domain,
			Data: map[string]interface{}{
				"request_id": requestID,
				"reason":     "no_available_seats",
			},
		}
		h.publishMessage(rejectMsg)
	}
}

// handleSeatOfferMessage обрабатывает предложение слота
func (h *ptpsquaredHandler) handleSeatOfferMessage(msg *PTPSquaredMessage) {
	offerID, _ := msg.Data["offer_id"].(string)
	
	h.logger.WithFields(logrus.Fields{
		"offer_id": offerID,
		"peer_id":  msg.PeerID,
		"domain":   msg.Domain,
	}).Info("Received seat offer")
	
	// Принимаем предложение
	acceptMsg := &PTPSquaredMessage{
		Type:      MsgTypeSeatAccept,
		PeerID:    h.peerID,
		Timestamp: time.Now(),
		Domain:    msg.Domain,
		Data: map[string]interface{}{
			"offer_id": offerID,
		},
	}
	h.publishMessage(acceptMsg)
}

// handleSeatAcceptMessage обрабатывает принятие слота
func (h *ptpsquaredHandler) handleSeatAcceptMessage(msg *PTPSquaredMessage) {
	offerID, _ := msg.Data["offer_id"].(string)
	
	h.logger.WithFields(logrus.Fields{
		"offer_id": offerID,
		"peer_id":  msg.PeerID,
	}).Info("Seat offer accepted")
	
	// Обновляем статус предложения
	h.mu.Lock()
	if offer, exists := h.seatOffers[offerID]; exists {
		offer.Status = "accepted"
	}
	h.mu.Unlock()
}

// handleSeatRejectMessage обрабатывает отказ от слота
func (h *ptpsquaredHandler) handleSeatRejectMessage(msg *PTPSquaredMessage) {
	requestID, _ := msg.Data["request_id"].(string)
	reason, _ := msg.Data["reason"].(string)
	
	h.logger.WithFields(logrus.Fields{
		"request_id": requestID,
		"peer_id":    msg.PeerID,
		"reason":     reason,
	}).Info("Seat request rejected")
	
	// Обновляем статус запроса
	h.mu.Lock()
	if request, exists := h.seatRequests[requestID]; exists {
		request.Status = "rejected"
	}
	h.mu.Unlock()
}

// handleHeartbeatMessage обрабатывает heartbeat сообщение
func (h *ptpsquaredHandler) handleHeartbeatMessage(msg *PTPSquaredMessage) {
	h.mu.Lock()
	if peer, exists := h.connectedPeers[msg.PeerID]; exists {
		peer.LastSeen = time.Now()
	}
	h.mu.Unlock()
}

// periodicTasks выполняет периодические задачи
func (h *ptpsquaredHandler) periodicTasks() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.sendHeartbeat()
			h.cleanupOldData()
		}
	}
}

// sendHeartbeat отправляет heartbeat
func (h *ptpsquaredHandler) sendHeartbeat() {
	msg := &PTPSquaredMessage{
		Type:      MsgTypeHeartbeat,
		PeerID:    h.peerID,
		Timestamp: time.Now(),
	}
	
	if err := h.publishMessage(msg); err != nil {
		h.logger.WithError(err).Error("Failed to send heartbeat")
	}
}

// cleanupOldData очищает старые данные
func (h *ptpsquaredHandler) cleanupOldData() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	now := time.Now()
	
	// Очищаем старые запросы слотов
	for id, request := range h.seatRequests {
		if now.Sub(request.Timestamp) > 5*time.Minute {
			delete(h.seatRequests, id)
		}
	}
	
	// Очищаем старые предложения слотов
	for id, offer := range h.seatOffers {
		if now.Sub(offer.Timestamp) > 5*time.Minute {
			delete(h.seatOffers, id)
		}
	}
	
	// Очищаем неактивных пиров
	for id, peer := range h.connectedPeers {
		if now.Sub(peer.LastSeen) > 2*time.Minute {
			delete(h.connectedPeers, id)
			delete(h.timeSources, id)
		}
	}
}

// publishMessage публикует сообщение
func (h *ptpsquaredHandler) publishMessage(msg *PTPSquaredMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	return h.topic.Publish(h.ctx, data)
}

// calculateQuality вычисляет качество источника времени
func (h *ptpsquaredHandler) calculateQuality(timeInfo *TimeInfo) float64 {
	// Простая формула качества на основе offset и delay
	offsetQuality := 1.0 - float64(timeInfo.Offset.Abs())/float64(time.Second)
	delayQuality := 1.0 - float64(timeInfo.Delay)/float64(time.Second)
	
	if offsetQuality < 0 {
		offsetQuality = 0
	}
	if delayQuality < 0 {
		delayQuality = 0
	}
	
	return (offsetQuality + delayQuality) / 2.0
}

// canOfferSeat проверяет, можем ли мы предложить слот
func (h *ptpsquaredHandler) canOfferSeat(domain int) bool {
	// Проверяем, поддерживаем ли мы этот домен
	domainSupported := false
	for _, d := range h.domains {
		if d == domain {
			domainSupported = true
			break
		}
	}
	
	if !domainSupported {
		return false
	}
	
	// Проверяем, есть ли свободные слоты
	activeOffers := 0
	for _, offer := range h.seatOffers {
		if offer.Status == "pending" || offer.Status == "accepted" {
			activeOffers++
		}
	}
	
	return activeOffers < h.seatsToOffer
}

// HandlePeerFound реализация mdns.Notifee
func (h *ptpsquaredHandler) HandlePeerFound(pi peer.AddrInfo) {
	h.logger.WithField("peer_id", pi.ID.String()).Info("Peer found via mDNS")
	
	// Подключаемся к пиру
	if err := h.host.Connect(h.ctx, pi); err != nil {
		h.logger.WithError(err).Error("Failed to connect to peer")
		return
	}
}

// HandlePeerFound реализация mdns.Notifee (для совместимости)
func (h *ptpsquaredHandler) HandlePeerFound2(pi peer.AddrInfo) {
	h.HandlePeerFound(pi)
}