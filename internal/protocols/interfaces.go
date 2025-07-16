package protocols

import (
	"time"
	
	"github.com/sirupsen/logrus"
	"github.com/shiwatime/shiwatime/internal/config"
)

// TimeInfo содержит информацию о времени от источника
type TimeInfo struct {
	Timestamp time.Time     // Время от источника
	Offset    time.Duration // Смещение относительно системного времени
	Delay     time.Duration // Задержка сети
	Quality   int           // Качество источника (0-255)
	Stratum   int           // Stratum для NTP
	Precision int           // Точность источника

	// GNSS/Position related (optional)
	Latitude  float64 // градусы
	Longitude float64 // градусы
	Altitude  float64 // метры
	FixType   int     // тип фикса
	SatellitesUsed int // используемые спутники
}

// TimeSourceHandler интерфейс обработчика источника времени
type TimeSourceHandler interface {
	// Start запускает обработчик
	Start() error
	
	// Stop останавливает обработчик
	Stop() error
	
	// GetTimeInfo получает информацию о времени
	GetTimeInfo() (*TimeInfo, error)
	
	// GetStatus получает статус соединения
	GetStatus() ConnectionStatus
	
	// GetConfig получает конфигурацию
	GetConfig() config.TimeSourceConfig
}

// ConnectionStatus статус соединения с источником времени
type ConnectionStatus struct {
	Connected      bool
	LastActivity   time.Time
	ErrorCount     int
	LastError      error
	PacketsRx      uint64
	PacketsTx      uint64
	BytesRx        uint64
	BytesTx        uint64
}

// PTPHandler интерфейс для PTP обработчика
type PTPHandler interface {
	TimeSourceHandler
	
	// GetClockIdentity получает clock identity
	GetClockIdentity() string
	
	// GetDomain получает PTP домен
	GetDomain() int
	
	// GetPortState получает состояние PTP порта
	GetPortState() PTPPortState
	
	// GetMasterInfo получает информацию о мастере
	GetMasterInfo() *PTPMasterInfo
	
	// SendAnnounce отправляет Announce сообщение
	SendAnnounce() error
	
	// SendSync отправляет Sync сообщение
	SendSync() error
	
	// HandleMessage обрабатывает входящее PTP сообщение
	HandleMessage(msg []byte) error
}

// PTPPortState состояние PTP порта
type PTPPortState int

const (
	PTPPortStateInitializing PTPPortState = iota
	PTPPortStateFaulty
	PTPPortStateDisabled
	PTPPortStateListening
	PTPPortStatePreMaster
	PTPPortStateMaster
	PTPPortStatePassive
	PTPPortStateUncalibrated
	PTPPortStateSlave
)

func (ps PTPPortState) String() string {
	switch ps {
	case PTPPortStateInitializing:
		return "INITIALIZING"
	case PTPPortStateFaulty:
		return "FAULTY"
	case PTPPortStateDisabled:
		return "DISABLED"
	case PTPPortStateListening:
		return "LISTENING"
	case PTPPortStatePreMaster:
		return "PRE_MASTER"
	case PTPPortStateMaster:
		return "MASTER"
	case PTPPortStatePassive:
		return "PASSIVE"
	case PTPPortStateUncalibrated:
		return "UNCALIBRATED"
	case PTPPortStateSlave:
		return "SLAVE"
	default:
		return "UNKNOWN"
	}
}

// PTPMasterInfo информация о PTP мастере
type PTPMasterInfo struct {
	ClockIdentity    string
	ClockClass       int
	ClockAccuracy    int
	OffsetScaledLogVariance int
	Priority1        int
	Priority2        int
	TimeSource       int
	StepsRemoved     int
	SourcePortIdentity string
}

// NTPHandler интерфейс для NTP обработчика  
type NTPHandler interface {
	TimeSourceHandler
	
	// GetStratum получает stratum сервера
	GetStratum() int
	
	// GetReferenceID получает reference ID
	GetReferenceID() string
	
	// GetRootDelay получает root delay
	GetRootDelay() time.Duration
	
	// GetRootDispersion получает root dispersion
	GetRootDispersion() time.Duration
	
	// SendRequest отправляет NTP запрос
	SendRequest() error
	
	// ParseResponse парсит NTP ответ
	ParseResponse(data []byte) (*TimeInfo, error)
}

// PPSHandler интерфейс для PPS обработчика
type PPSHandler interface {
	TimeSourceHandler
	
	// GetPulseCount получает количество импульсов
	GetPulseCount() uint64
	
	// GetLastPulseTime получает время последнего импульса
	GetLastPulseTime() time.Time
	
	// EnablePulseOutput включает выход PPS
	EnablePulseOutput() error
	
	// DisablePulseOutput выключает выход PPS
	DisablePulseOutput() error
}

// NMEAHandler интерфейс для NMEA обработчика
type NMEAHandler interface {
	TimeSourceHandler
	
	// GetGNSSStatus получает статус GNSS
	GetGNSSStatus() *GNSSStatus
	
	// GetSatelliteCount получает количество видимых спутников
	GetSatelliteCount() int
	
	// GetPosition получает текущую позицию
	GetPosition() *Position
	
	// ParseNMEA парсит NMEA сообщение
	ParseNMEA(line string) error
}

// GNSSStatus статус GNSS приемника
type GNSSStatus struct {
	FixType      int    // Тип фикса (0-нет, 1-GPS, 2-DGPS, 3-PPS)
	FixQuality   int    // Качество фикса
	SatellitesUsed int  // Используемые спутники
	SatellitesVisible int // Видимые спутники
	HDOP         float64 // Horizontal Dilution of Precision
	VDOP         float64 // Vertical Dilution of Precision
	PDOP         float64 // Position Dilution of Precision
}

// Position GPS позиция
type Position struct {
	Latitude  float64
	Longitude float64
	Altitude  float64
	Timestamp time.Time
}

// CreateHandler создает обработчик для указанного протокола
func CreateHandler(protocol string, config config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
	switch protocol {
	case "ptp":
		return NewPTPHandler(config, logger)
	case "ntp":
		return NewNTPHandler(config, logger)
	case "pps":
		return NewPPSHandler(config, logger)
	case "nmea":
		return NewNMEAHandler(config, logger)
	case "phc":
		return NewPHCHandler(config, logger)
	default:
		return NewMockHandler(config, logger)
	}
}

// PTPSquaredHandler интерфейс для PTP+Squared обработчика
type PTPSquaredHandler interface {
	TimeSourceHandler
	
	// GetPeerID получает ID пира
	GetPeerID() string
	
	// GetDomains получает поддерживаемые домены
	GetDomains() []int
	
	// GetSeatsToOffer получает количество предлагаемых слотов
	GetSeatsToOffer() int
	
	// GetSeatsToFill получает количество заполняемых слотов
	GetSeatsToFill() int
	
	// GetConcurrentSources получает количество одновременных источников
	GetConcurrentSources() int
	
	// GetCapabilities получает возможности узла
	GetCapabilities() []string
	
	// GetPreferenceScore получает предпочтительный балл
	GetPreferenceScore() int
	
	// GetReservations получает резервирования
	GetReservations() []string
	
	// GetConnectedPeers получает список подключенных пиров
	GetConnectedPeers() []string
	
	// GetNetworkStats получает статистику сети
	GetNetworkStats() *PTPSquaredNetworkStats
	
	// RequestSeat запрашивает слот у другого узла
	RequestSeat(peerID string, domain int) error
	
	// OfferSeat предлагает слот другому узлу
	OfferSeat(peerID string, domain int) error
	
	// HandleTimeSync обрабатывает синхронизацию времени
	HandleTimeSync(peerID string, timeInfo *TimeInfo) error
}

// PTPSquaredNetworkStats статистика PTP+Squared сети
type PTPSquaredNetworkStats struct {
	TotalPeers       int
	ActivePeers      int
	TotalSeatsOffered int
	TotalSeatsFilled  int
	AverageLatency    time.Duration
	AverageJitter     time.Duration
	NetworkQuality    float64
}