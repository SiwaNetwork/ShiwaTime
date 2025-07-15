//go:build !libbeat
// +build !libbeat

package metrics

import (
    "context"
    "errors"
)

// BeatPublisher stub returns error when libbeat not enabled.

type BeatPublisher struct{}

func NewBeatPublisher(index string) (*BeatPublisher, error) {
    return nil, errors.New("libbeat build tag not enabled")
}

func (p *BeatPublisher) PublishAsync(ctx context.Context, evt Event) {
    // no-op
}