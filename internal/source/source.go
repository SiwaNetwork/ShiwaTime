package source

import (
    "context"
    "time"
)

// OffsetMeasurement represents single measurement from time source.
type OffsetMeasurement struct {
    Offset       time.Duration // positive if system clock ahead of source
    Delay        time.Duration // network delay or other measurement error estimate
    SourceName   string
    Timestamp    time.Time // when measurement was taken
}

// Source defines behaviour of time source such as NTP or PTP.
type Source interface {
    // Start measurement loop. Should send OffsetMeasurement on returned channel until ctx done.
    Start(ctx context.Context) (<-chan OffsetMeasurement, error)
    // Name returns unique description.
    Name() string
}