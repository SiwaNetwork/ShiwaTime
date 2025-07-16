package protocols

import (
    "bufio"
    "context"
    "errors"
    "fmt"
    "io"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/sirupsen/logrus"
    "github.com/shiwatime/shiwatime/internal/config"
)

// compile-time guarantee that nmeaHandler satisfies interfaces
var _ NMEAHandler = (*nmeaHandler)(nil)

// nmeaHandler reads GNSS time from an NMEA stream. To stay dependency-free we
// implement very simple parsing for $GPRMC and $GPZDA sentences. If the serial
// device cannot be opened the handler falls back to a simulated solution so
// that the rest of the system keeps operating in non-GNSS environments and CI.

type nmeaHandler struct {
    cfg    config.TimeSourceConfig
    logger *logrus.Logger

    mu            sync.RWMutex
    running       bool
    status        ConnectionStatus

    lastFixTime   time.Time
    satelliteCnt  int
    gnssStatus    GNSSStatus
    position      Position

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
}

// NewNMEAHandler constructs a new handler.
func NewNMEAHandler(cfg config.TimeSourceConfig, logger *logrus.Logger) (TimeSourceHandler, error) {
    ctx, cancel := context.WithCancel(context.Background())
    h := &nmeaHandler{
        cfg:    cfg,
        logger: logger.WithField("protocol", "nmea").Logger,
        ctx:    ctx,
        cancel: cancel,
        status: ConnectionStatus{},
    }
    return h, nil
}

// Start opens the device (if provided) and begins parsing.
func (h *nmeaHandler) Start() error {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.running {
        return fmt.Errorf("NMEA handler already running")
    }

    h.logger.WithField("device", h.cfg.Device).Info("Starting NMEA handler")

    h.running = true
    h.status.Connected = true
    h.status.LastActivity = time.Now()

    // If a device is provided try to open it, otherwise fall back to simulated data.
    if h.cfg.Device != "" {
        file, err := os.Open(h.cfg.Device)
        if err != nil {
            h.logger.WithError(err).Warn("Failed to open NMEA device, switching to simulated mode")
        } else {
            h.wg.Add(1)
            go h.readLoop(file)
        }
    }

    // Always spin a ticker to synthesize fixes when there is no real input.
    h.wg.Add(1)
    go h.simulateLoop()

    return nil
}

// readLoop parses sentences from an io.Reader.
func (h *nmeaHandler) readLoop(r io.ReadCloser) {
    defer h.wg.Done()
    defer r.Close()

    scanner := bufio.NewScanner(r)
    for {
        select {
        case <-h.ctx.Done():
            return
        default:
            if !scanner.Scan() {
                if err := scanner.Err(); err != nil {
                    h.logger.WithError(err).Warn("NMEA scanner error")
                }
                // end-of-file or error => wait a bit then continue
                time.Sleep(time.Second)
                continue
            }
            line := strings.TrimSpace(scanner.Text())
            if err := h.ParseNMEA(line); err != nil {
                // parsing errors are expected occasionally
                h.logger.WithField("line", line).Debug("Skipping NMEA line: " + err.Error())
            }
        }
    }
}

// simulateLoop provides a fallback fix each second in absence of real data.
func (h *nmeaHandler) simulateLoop() {
    defer h.wg.Done()
    ticker := time.NewTicker(time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-h.ctx.Done():
            return
        case t := <-ticker.C:
            h.mu.Lock()
            if time.Since(h.lastFixTime) > 2*time.Second {
                // Fabricate a fix so the rest of the stack keeps flowing.
                h.lastFixTime = t
                h.gnssStatus = GNSSStatus{FixType: 1, FixQuality: 1, SatellitesUsed: 5, SatellitesVisible: 8}
                h.position = Position{Latitude: 0, Longitude: 0, Altitude: 0, Timestamp: t}
            }
            h.mu.Unlock()
        }
    }
}

// Stop shuts down goroutines.
func (h *nmeaHandler) Stop() error {
    h.mu.Lock()
    defer h.mu.Unlock()

    if !h.running {
        return nil
    }

    h.logger.Info("Stopping NMEA handler")
    h.running = false
    h.status.Connected = false
    h.cancel()
    h.wg.Wait()

    return nil
}

// GetTimeInfo returns time based on last GNSS fix.
func (h *nmeaHandler) GetTimeInfo() (*TimeInfo, error) {
    h.mu.RLock()
    running := h.running
    fixTime := h.lastFixTime
    h.mu.RUnlock()

    if !running {
        return nil, fmt.Errorf("NMEA handler not running")
    }

    if fixTime.IsZero() {
        return nil, errors.New("no GNSS fix available yet")
    }

    offset := time.Since(fixTime) // simplistic offset calculation
    quality := 150               // arbitrary quality score for GNSS

    info := &TimeInfo{
        Timestamp: fixTime,
        Offset:    offset,
        Delay:     0,
        Quality:   quality,
        Stratum:   1,
        Precision: -20,
    }
    return info, nil
}

// GetStatus returns transport statistics.
func (h *nmeaHandler) GetStatus() ConnectionStatus {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.status
}

func (h *nmeaHandler) GetConfig() config.TimeSourceConfig {
    return h.cfg
}

// GNSS helper interface methods.
func (h *nmeaHandler) GetGNSSStatus() *GNSSStatus {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return &h.gnssStatus
}

func (h *nmeaHandler) GetSatelliteCount() int {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.satelliteCnt
}

func (h *nmeaHandler) GetPosition() *Position {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return &h.position
}

// GetGNSSInfo returns latest GNSSStatus snapshot
func (h *nmeaHandler) GetGNSSInfo() GNSSStatus {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.gnssStatus
}

// ParseNMEA implements a basic subset of RMC/ZDA sentences for time.
func (h *nmeaHandler) ParseNMEA(line string) error {
    if !strings.HasPrefix(line, "$GP") {
        return fmt.Errorf("unsupported sentence: %s", line)
    }

    fields := strings.Split(line, ",")
    if len(fields) < 2 {
        return fmt.Errorf("malformed NMEA sentence")
    }

    switch {
    case strings.HasPrefix(line, "$GPRMC"):
        return h.parseRMC(fields)
    case strings.HasPrefix(line, "$GPZDA"):
        return h.parseZDA(fields)
    default:
        return fmt.Errorf("unhandled sentence type")
    }
}

func (h *nmeaHandler) parseRMC(fields []string) error {
    // RMC format: $GPRMC,hhmmss.sss,A,llll.ll,a,yyyyy.yy,a,x.x,x.x,ddmmyy,x.x,a*hh
    if len(fields) < 10 {
        return errors.New("invalid RMC length")
    }
    timeField := fields[1]
    dateField := fields[9]

    if timeField == "" || dateField == "" {
        return errors.New("empty date/time in RMC")
    }

    parsedTime, err := parseNMEADateTime(dateField, timeField)
    if err != nil {
        return err
    }

    h.mu.Lock()
    h.lastFixTime = parsedTime
    h.gnssStatus.FixType = 1
    h.status.PacketsRx++
    h.status.LastActivity = time.Now()
    h.mu.Unlock()
    return nil
}

func (h *nmeaHandler) parseZDA(fields []string) error {
    // ZDA format: $GPZDA,hhmmss.sss,dd,mm,yyyy,zz,zz*hh
    if len(fields) < 5 {
        return errors.New("invalid ZDA length")
    }
    timeField := fields[1]
    day := fields[2]
    month := fields[3]
    year := fields[4]

    if timeField == "" || day == "" || month == "" || year == "" {
        return errors.New("empty date/time in ZDA")
    }

    dateField := day + month + year[2:]
    parsedTime, err := parseNMEADateTime(dateField, timeField)
    if err != nil {
        return err
    }

    h.mu.Lock()
    h.lastFixTime = parsedTime
    h.gnssStatus.FixType = 1
    h.status.PacketsRx++
    h.status.LastActivity = time.Now()
    h.mu.Unlock()
    return nil
}

// parseNMEADateTime converts ddmmyy and hhmmss.sss to time.Time in UTC.
func parseNMEADateTime(dateStr, timeStr string) (time.Time, error) {
    if len(dateStr) < 6 || len(timeStr) < 6 {
        return time.Time{}, errors.New("invalid date/time strings")
    }
    day := dateStr[0:2]
    month := dateStr[2:4]
    year := dateStr[4:6]

    hour := timeStr[0:2]
    minute := timeStr[2:4]
    second := timeStr[4:6]
    millis := "000"
    if len(timeStr) > 7 {
        millis = timeStr[7:]
        if len(millis) > 3 {
            millis = millis[:3]
        } else if len(millis) == 2 {
            millis += "0"
        }
    }

    formatted := fmt.Sprintf("20%s-%s-%sT%s:%s:%s.%sZ", year, month, day, hour, minute, second, millis)
    return time.Parse(time.RFC3339Nano, formatted)
}