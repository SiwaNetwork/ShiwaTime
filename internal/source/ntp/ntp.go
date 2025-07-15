package ntp

import (
    "context"
    "fmt"
    "time"

    beevikntp "github.com/beevik/ntp"

    "shiwa/internal/source"
)

// NTPSource implements Source interface using NTP server.

type NTPSource struct {
    addr        string
    poll        time.Duration
    monitorOnly bool
}

func New(addr string, poll time.Duration, monitorOnly bool) *NTPSource {
    if poll == 0 {
        poll = 4 * time.Second
    }
    return &NTPSource{addr: addr, poll: poll, monitorOnly: monitorOnly}
}

func (n *NTPSource) Name() string { return fmt.Sprintf("ntp:%s", n.addr) }

func (n *NTPSource) Start(ctx context.Context) (<-chan source.OffsetMeasurement, error) {
    ch := make(chan source.OffsetMeasurement)
    go func() {
        ticker := time.NewTicker(n.poll)
        defer ticker.Stop()
        defer close(ch)
        for {
            select {
            case <-ctx.Done():
                return
            default:
            }
            resp, err := beevikntp.Query(n.addr)
            if err != nil {
                // propagate error as measurement with zero offset? skip
            } else {
                offset := resp.ClockOffset
                delay := resp.RTT / 2
                ch <- source.OffsetMeasurement{
                    Offset:     offset,
                    Delay:      delay,
                    SourceName: n.Name(),
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