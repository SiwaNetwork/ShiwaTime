package ntp

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/shiwatime/shiwatime/pkg/clock"
	"github.com/shiwatime/shiwatime/pkg/config"
	"github.com/shiwatime/shiwatime/pkg/types"
	"github.com/sirupsen/logrus"
)

const (
	ntpEpochOffset = 2208988800 // Difference between NTP epoch (1900) and Unix epoch (1970)
	ntpPort        = "123"
)

// NTPSource implements clock.TimeSource for NTP protocol
type NTPSource struct {
	*clock.BaseTimeSource
	config   config.ClockSource
	conn     net.Conn
	mu       sync.Mutex
	stopCh   chan struct{}
	wg       sync.WaitGroup
	logger   *logrus.Entry
	samples  chan *types.TimeSample
	pollInterval time.Duration
}

// NewNTPSource creates a new NTP time source
func NewNTPSource(cfg config.ClockSource, priority int) (*NTPSource, error) {
	pollInterval := 4 * time.Second
	if cfg.PollInterval != "" {
		d, err := time.ParseDuration(cfg.PollInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid poll interval: %w", err)
		}
		pollInterval = d
	}

	return &NTPSource{
		BaseTimeSource: clock.NewBaseTimeSource("ntp", priority),
		config:         cfg,
		stopCh:         make(chan struct{}),
		samples:        make(chan *types.TimeSample, 10),
		pollInterval:   pollInterval,
		logger: logrus.WithFields(logrus.Fields{
			"source":   "ntp",
			"server":   cfg.IP,
			"priority": priority,
		}),
	}, nil
}

// Start starts the NTP client
func (n *NTPSource) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.conn != nil {
		return fmt.Errorf("NTP source already started")
	}

	n.SetState(clock.StateInitializing)
	n.logger.Info("Starting NTP source")

	// Create UDP connection
	serverAddr, err := net.ResolveUDPAddr("udp", n.config.IP+":"+ntpPort)
	if err != nil {
		n.SetState(clock.StateError)
		return fmt.Errorf("failed to resolve server address: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		n.SetState(clock.StateError)
		return fmt.Errorf("failed to create connection: %w", err)
	}

	n.conn = conn
	n.SetState(clock.StateSyncing)

	// Start polling routine
	n.wg.Add(1)
	go n.pollRoutine(ctx)

	return nil
}

// Stop stops the NTP client
func (n *NTPSource) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.conn == nil {
		return nil
	}

	n.logger.Info("Stopping NTP source")
	close(n.stopCh)
	
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}

	n.wg.Wait()
	n.SetState(clock.StateStopped)
	
	return nil
}

// GetSample returns the latest time sample
func (n *NTPSource) GetSample() (*types.TimeSample, error) {
	select {
	case sample := <-n.samples:
		return sample, nil
	case <-time.After(100 * time.Millisecond):
		return nil, fmt.Errorf("no sample available")
	}
}

// IsAvailable returns true if the source is available
func (n *NTPSource) IsAvailable() bool {
	return n.GetStatus().State == clock.StateSynchronized
}

// pollRoutine polls the NTP server periodically
func (n *NTPSource) pollRoutine(ctx context.Context) {
	defer n.wg.Done()

	ticker := time.NewTicker(n.pollInterval)
	defer ticker.Stop()

	// Initial poll
	n.poll()

	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopCh:
			return
		case <-ticker.C:
			n.poll()
		}
	}
}

// poll performs a single NTP query
func (n *NTPSource) poll() {
	packet := &ntpPacket{
		Settings: 0x1B, // LI=0, VN=3, Mode=3 (client)
	}

	// Record transmit timestamp
	t1 := time.Now()
	packet.TxTimestamp = toNTPTime(t1)

	// Send request
	if err := binary.Write(n.conn, binary.BigEndian, packet); err != nil {
		n.logger.WithError(err).Error("Failed to send NTP request")
		n.IncrementErrorCount(err.Error())
		return
	}

	// Set read timeout
	n.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read response
	response := &ntpPacket{}
	if err := binary.Read(n.conn, binary.BigEndian, response); err != nil {
		n.logger.WithError(err).Error("Failed to read NTP response")
		n.IncrementErrorCount(err.Error())
		return
	}

	// Record receive timestamp
	t4 := time.Now()

	// Extract timestamps
	t2 := fromNTPTime(response.RxTimestamp)
	t3 := fromNTPTime(response.TxTimestamp)

	// Calculate offset and delay
	// offset = ((t2 - t1) + (t3 - t4)) / 2
	// delay = (t4 - t1) - (t3 - t2)
	offset := ((t2.Sub(t1)) + (t3.Sub(t4))) / 2
	delay := (t4.Sub(t1)) - (t3.Sub(t2))

	// Create sample
	sample := &types.TimeSample{
		LocalTime:  t1,
		SourceTime: t2.Add(offset),
		Offset:     offset,
		Delay:      delay,
		Error:      delay / 2, // Rough estimate
		Valid:      true,
		Source:     fmt.Sprintf("ntp://%s", n.config.IP),
		Quality:    calculateQuality(delay, response.Stratum),
	}

	// Update status
	n.IncrementSyncCount()
	n.SetState(clock.StateSynchronized)
	
	status := n.GetStatus()
	status.Stratum = int(response.Stratum) + 1
	status.RootDelay = fromNTPShort(response.RootDelay)
	status.RootDispersion = fromNTPShort(response.RootDispersion)

	// Send sample
	select {
	case n.samples <- sample:
	default:
		// Drop oldest sample if channel is full
		<-n.samples
		n.samples <- sample
	}

	n.logger.WithFields(logrus.Fields{
		"offset":     offset,
		"delay":      delay,
		"stratum":    response.Stratum,
		"quality":    sample.Quality,
	}).Debug("NTP poll completed")
}

// ntpPacket represents an NTP packet
type ntpPacket struct {
	Settings       uint8  // LI, VN, Mode
	Stratum        uint8
	Poll           int8
	Precision      int8
	RootDelay      uint32
	RootDispersion uint32
	RefID          uint32
	RefTimestamp   uint64
	OrigTimestamp  uint64
	RxTimestamp    uint64
	TxTimestamp    uint64
}

// toNTPTime converts time.Time to NTP timestamp
func toNTPTime(t time.Time) uint64 {
	seconds := uint64(t.Unix() + ntpEpochOffset)
	fraction := uint64(t.Nanosecond()) << 32 / 1e9
	return seconds<<32 | fraction
}

// fromNTPTime converts NTP timestamp to time.Time
func fromNTPTime(ntpTime uint64) time.Time {
	seconds := int64(ntpTime>>32) - ntpEpochOffset
	nanoseconds := int64((ntpTime & 0xFFFFFFFF) * 1e9 >> 32)
	return time.Unix(seconds, nanoseconds)
}

// fromNTPShort converts NTP short format to time.Duration
func fromNTPShort(ntpShort uint32) time.Duration {
	seconds := int64(ntpShort >> 16)
	fraction := int64(ntpShort & 0xFFFF)
	nanoseconds := seconds*1e9 + (fraction*1e9)>>16
	return time.Duration(nanoseconds)
}

// calculateQuality calculates the quality score based on delay and stratum
func calculateQuality(delay time.Duration, stratum uint8) int {
	// Start with perfect quality
	quality := 100

	// Reduce quality based on delay
	if delay > 100*time.Millisecond {
		quality -= 20
	} else if delay > 50*time.Millisecond {
		quality -= 10
	} else if delay > 10*time.Millisecond {
		quality -= 5
	}

	// Reduce quality based on stratum
	quality -= int(stratum) * 5

	// Ensure quality is within bounds
	if quality < 0 {
		quality = 0
	}

	return quality
}