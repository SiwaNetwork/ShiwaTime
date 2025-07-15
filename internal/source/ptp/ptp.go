package ptp

import (
    "context"

    ptpstack "shiwa/internal/ptp"
    "shiwa/internal/source"
)

type PTPSource struct {
    sess *ptpstack.Session
    name string
}

func New(iface string, domain uint8) *PTPSource {
    return &PTPSource{
        sess: ptpstack.NewSession(iface, domain),
        name: "ptp:" + iface,
    }
}

func (p *PTPSource) Name() string { return p.name }

func (p *PTPSource) Start(ctx context.Context) (<-chan source.OffsetMeasurement, error) {
    return p.sess.Start(ctx)
}