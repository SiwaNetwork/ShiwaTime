package main

import (
    "context"
    "flag"
    "log"
    "os"
    "os/signal"
    "time"

    "shiwa/internal/clock"
    "shiwa/internal/config"
    "shiwa/internal/metrics"
    "shiwa/internal/source"
    "shiwa/internal/source/ntp"
)

func main() {
    cfgPath := flag.String("config", "timebeat.yml", "Path to configuration YAML")
    flag.Parse()

    cfg, err := config.Load(*cfgPath)
    if err != nil {
        log.Fatalf("failed to load config: %v", err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sig := make(chan os.Signal, 1)
    signal.Notify(sig, os.Interrupt)
    go func() {
        <-sig
        cancel()
    }()

    // Metrics client
    var metricsClient *metrics.Client
    if len(cfg.OutputElastic.Hosts) > 0 {
        metricsClient, err = metrics.New(cfg.OutputElastic.Hosts, "shiwa-timebeat")
        if err != nil {
            log.Printf("metrics disabled: %v", err)
        }
    }

    // Build sources list (currently only NTP supported)
    var sources []source.Source
    for _, s := range cfg.Timebeat.ClockSync.Primary {
        if s.Disable {
            continue
        }
        if s.Protocol == "ntp" {
            sources = append(sources, ntp.New(s.IP, s.PollInterval.Duration, s.MonitorOnly))
        }
        // TODO: ptp, pps, etc.
    }

    if len(sources) == 0 {
        log.Fatalf("no active time sources configured")
    }

    // Start sources and aggregate offsets
    measurementCh := make(chan source.OffsetMeasurement)
    for _, src := range sources {
        ch, err := src.Start(ctx)
        if err != nil {
            log.Fatalf("failed to start source %s: %v", src.Name(), err)
        }
        go func(c <-chan source.OffsetMeasurement) {
            for m := range c {
                measurementCh <- m
            }
        }(ch)
    }

    stepLimit := cfg.Timebeat.ClockSync.StepLimit.Duration
    if stepLimit == 0 {
        stepLimit = 15 * time.Minute
    }

    for {
        select {
        case <-ctx.Done():
            return
        case m := <-measurementCh:
            log.Printf("measurement from %s offset=%s delay=%s", m.SourceName, m.Offset, m.Delay)
            if !cfg.Timebeat.ClockSync.AdjustClock {
                continue
            }
            if err := clock.ApplyOffset(m.Offset, stepLimit); err != nil {
                log.Printf("clock adjust error: %v", err)
            }
            if metricsClient != nil {
                evt := metrics.Event{
                    Timestamp: time.Now(),
                    Fields: map[string]interface{}{
                        "source":   m.SourceName,
                        "offset":   m.Offset.Seconds(),
                        "delay":    m.Delay.Seconds(),
                    },
                }
                metricsClient.PublishAsync(ctx, evt)
            }
        }
    }
}