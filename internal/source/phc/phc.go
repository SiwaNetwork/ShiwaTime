package phc

import (
    "context"
    "fmt"
    "os"
    "time"

    "golang.org/x/sys/unix"

    "shiwa/internal/source"
)

// PHCSource reads offset between system clock and a PHC device (e.g. /dev/ptp0).
type PHCSource struct {
    devicePath string
    monitorOnly bool
}

func New(devicePath string, monitorOnly bool) *PHCSource {
    return &PHCSource{devicePath: devicePath, monitorOnly: monitorOnly}
}

func (p *PHCSource) Name() string { return fmt.Sprintf("phc:%s", p.devicePath) }

// fdToClockID replicates FD_TO_CLOCKID macro from linux/time.h.
func fdToClockID(fd int) int32 {
    return int32((^fd << 3) | 3)
}

func (p *PHCSource) Start(ctx context.Context) (<-chan source.OffsetMeasurement, error) {
    file, err := os.Open(p.devicePath)
    if err != nil {
        return nil, err
    }
    fd := int(file.Fd())
    clockID := fdToClockID(fd)

    ch := make(chan source.OffsetMeasurement)
    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        defer close(ch)
        defer file.Close()
        for {
            select {
            case <-ctx.Done():
                return
            default:
            }
            var ts unix.Timespec
            if err := unix.ClockGettime(clockID, &ts); err != nil {
                // skip on error
            } else {
                phcTime := time.Unix(int64(ts.Sec), int64(ts.Nsec))
                sysTime := time.Now()
                offset := sysTime.Sub(phcTime)
                ch <- source.OffsetMeasurement{
                    Offset:     offset,
                    Delay:      0,
                    SourceName: p.Name(),
                    Timestamp:  time.Now(),
                }
            }
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
            }
        }
    }()
    return ch, nil
}