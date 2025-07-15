package protocols

import "time"



// PPS-specific types
type PPSSignalType int

const (
	PPSSignalRising PPSSignalType = iota
	PPSSignalFalling
	PPSSignalBoth
)

func (t PPSSignalType) String() string {
	switch t {
	case PPSSignalRising:
		return "RISING"
	case PPSSignalFalling:
		return "FALLING"
	case PPSSignalBoth:
		return "BOTH"
	default:
		return "UNKNOWN"
	}
}

// PPSEvent представляет PPS событие
type PPSEvent struct {
	Timestamp   time.Time
	SignalType  PPSSignalType
	SequenceNum uint64
}

// PHC-specific types
type PHCInfo struct {
	Index       int
	Name        string
	MaxAdj      int64
	NChannels   int
	PPSAvail    bool
	CrossTsAvail bool
}

// PHCTimestamp представляет аппаратную метку времени
type PHCTimestamp struct {
	Seconds     int64
	Nanoseconds int32
	Raw         bool
}