package ptp

import (
    "encoding/binary"
    "errors"
)

// MessageType represents the PTP message type field (4 bits).
const (
    MsgSync              = 0x0
    MsgDelayReq          = 0x1
    MsgFollowUp          = 0x8
    MsgDelayResp         = 0x9
    MsgAnnounce          = 0xb
    MsgSignal            = 0xc
    MsgManagement        = 0xd
)

// Header lengths.
const (
    headerSize = 34 // bytes for common header
)

// PtPHeader is common header of PTPv2 message (IEEE1588-2008).
type Header struct {
    TransportSpecific uint8 // 4 bits transportSpecific | 4 bits messageType
    Version           uint8 // 4 bits version
    MessageLength     uint16
    DomainNumber      uint8
    Flags             uint16 // flagsField
    CorrectionField   uint64
    SourcePortIdentity PortIdentity
    SequenceID        uint16
    ControlField      uint8
    LogMessageInterval int8
}

type PortIdentity struct {
    ClockIdentity [8]byte
    PortNumber    uint16
}

func (h *Header) MessageType() uint8 {
    return h.TransportSpecific & 0x0f
}

func (h *Header) MarshalBinary() ([]byte, error) {
    buf := make([]byte, headerSize)
    buf[0] = h.TransportSpecific
    buf[1] = h.Version
    binary.BigEndian.PutUint16(buf[2:], h.MessageLength)
    buf[4] = h.DomainNumber
    buf[5] = 0          // reserved
    binary.BigEndian.PutUint16(buf[6:], h.Flags)
    binary.BigEndian.PutUint64(buf[8:], h.CorrectionField)
    // reserved 4 bytes 16..19
    copy(buf[20:28], h.SourcePortIdentity.ClockIdentity[:])
    binary.BigEndian.PutUint16(buf[28:], h.SourcePortIdentity.PortNumber)
    binary.BigEndian.PutUint16(buf[30:], h.SequenceID)
    buf[32] = h.ControlField
    buf[33] = byte(h.LogMessageInterval)
    return buf, nil
}

func (h *Header) UnmarshalBinary(b []byte) error {
    if len(b) < headerSize {
        return errors.New("buffer too small")
    }
    h.TransportSpecific = b[0]
    h.Version = b[1]
    h.MessageLength = binary.BigEndian.Uint16(b[2:])
    h.DomainNumber = b[4]
    h.Flags = binary.BigEndian.Uint16(b[6:])
    h.CorrectionField = binary.BigEndian.Uint64(b[8:])
    copy(h.SourcePortIdentity.ClockIdentity[:], b[20:28])
    h.SourcePortIdentity.PortNumber = binary.BigEndian.Uint16(b[28:])
    h.SequenceID = binary.BigEndian.Uint16(b[30:])
    h.ControlField = b[32]
    h.LogMessageInterval = int8(b[33])
    return nil
}

// Timestamp is 48-bit seconds + 32-bit nanoseconds structure.
type Timestamp struct {
    Seconds uint64 // only lower 48 bits used
    Nanoseconds uint32
}

// MarshalTimestamp writes Timestamp into 10-byte buffer.
func (t Timestamp) Marshal(buf []byte) {
    // assume len(buf)>=10
    buf[0] = byte(t.Seconds >> 40)
    buf[1] = byte(t.Seconds >> 32)
    buf[2] = byte(t.Seconds >> 24)
    buf[3] = byte(t.Seconds >> 16)
    buf[4] = byte(t.Seconds >> 8)
    buf[5] = byte(t.Seconds)
    binary.BigEndian.PutUint32(buf[6:], t.Nanoseconds)
}

func (t *Timestamp) Unmarshal(buf []byte) {
    t.Seconds = uint64(buf[0])<<40 | uint64(buf[1])<<32 | uint64(buf[2])<<24 | uint64(buf[3])<<16 | uint64(buf[4])<<8 | uint64(buf[5])
    t.Nanoseconds = binary.BigEndian.Uint32(buf[6:])
}

// Announce message specific fields (after common header).
type AnnounceMessage struct {
    OriginTimestamp Timestamp
    CurrentUTCOffset int16
    GrandmasterPriority1 uint8
    GrandmasterClockQuality ClockQuality
    GrandmasterPriority2 uint8
    GrandmasterIdentity [8]byte
    StepsRemoved uint16
    TimeSource uint8
}

// Signaling message consists of header + targetPortIdentity (10 bytes) + TLVs.

type SignalingMessage struct {
    Target PortIdentity
    TLVs []TLV
}

// Management message:
type ManagementMessage struct {
    Target PortIdentity
    StartingTLV TLV // simplified: one TLV
}

// TLV generic
const tlvHeaderSize = 4

type TLV struct {
    Type  uint16
    Value []byte
}

func (t *TLV) Marshal() []byte {
    buf := make([]byte, tlvHeaderSize+len(t.Value))
    binary.BigEndian.PutUint16(buf[0:], t.Type)
    binary.BigEndian.PutUint16(buf[2:], uint16(len(t.Value)))
    copy(buf[4:], t.Value)
    return buf
}