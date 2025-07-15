//go:build libbeat
// +build libbeat

// Beat Publisher with libbeat implementation

package metrics

import (
    "context"
    "time"

    "github.com/elastic/beats/v7/libbeat/beat"
    "github.com/elastic/beats/v7/libbeat/common"
    "github.com/elastic/beats/v7/libbeat/publisher/pipeline"
)

// BeatPublisher wraps libbeat pipeline for publishing events.
type BeatPublisher struct {
    client beat.Client
}

func NewBeatPublisher(index string) (*BeatPublisher, error) {
    // Minimal beat instance config
    settings := beat.Beat{Info: beat.Info{Beat: "shiwa", IndexPrefix: index, Version: "0.1"}}

    // Create pipeline with default config (outputs taken from beats.yml or env)
    cfg := common.NewConfig()

    pipeline, err := pipeline.Load(settings, cfg, beat.BeatMetrics{})
    if err != nil {
        return nil, err
    }

    client, err := pipeline.Connect()
    if err != nil {
        return nil, err
    }

    return &BeatPublisher{client: client}, nil
}

func (p *BeatPublisher) Publish(ctx context.Context, evt Event) error {
    e := beat.Event{
        Timestamp: evt.Timestamp,
        Fields:    common.MapStr(evt.Fields),
    }
    return p.client.Publish(e)
}

func (p *BeatPublisher) PublishAsync(ctx context.Context, evt Event) {
    e := beat.Event{Timestamp: evt.Timestamp, Fields: common.MapStr(evt.Fields)}
    p.client.Publish(e)
}

func (p *BeatPublisher) Close() {
    if p.client != nil {
        p.client.Close()
    }
}