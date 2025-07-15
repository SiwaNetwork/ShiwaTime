package ptp

import (
    "context"
    "encoding/binary"
    "net"
    "syscall"
    "time"

    "unsafe"

    "golang.org/x/sys/unix"

    "shiwa/internal/source"
)

// Session listens on given interface and domain, acts as Ordinary Clock (slave).

type Session struct {
    iface string
    domain uint8
}

func NewSession(iface string, domain uint8) *Session {
    return &Session{iface: iface, domain: domain}
}

func (s *Session) Start(ctx context.Context) (<-chan source.OffsetMeasurement, error) {
    // open UDP socket
    addr := &net.UDPAddr{IP: net.IPv4(224, 0, 1, 129), Port: 319}
    conn, err := net.ListenUDP("udp4", addr)
    if err != nil {
        return nil, err
    }
    _ = s.iface // TODO join multicast on specific iface

    ch := make(chan source.OffsetMeasurement)
    go s.run(ctx, conn, ch)
    return ch, nil
}

func (s *Session) run(ctx context.Context, conn *net.UDPConn, out chan<- source.OffsetMeasurement) {
    defer close(out)
    var seq uint16
    for {
        select {
        case <-ctx.Done():
            conn.Close()
            return
        default:
        }
        buf := make([]byte, 1500)
        n, _, _, _, err := recvmsgWithTstamp(conn, buf)
        if err != nil {
            continue
        }
        if n < headerSize {
            continue
        }
        var hdr Header
        if err := hdr.UnmarshalBinary(buf[:headerSize]); err != nil {
            continue
        }
        switch hdr.MessageType() {
        case MsgSync:
            seq = hdr.SequenceID
            // record t2 timestamp in recvmsgWithTstamp
        case MsgFollowUp:
            if hdr.SequenceID != seq {
                continue
            }
            // extract originTimestamp from FollowUp (bytes 34..)
            seconds := uint64(buf[34])<<40 | uint64(buf[35])<<32 | uint64(buf[36])<<24 | uint64(buf[37])<<16 | uint64(buf[38])<<8 | uint64(buf[39])
            nanosec := binary.BigEndian.Uint32(buf[40:44])
            masterTime := time.Unix(int64(seconds), int64(nanosec))
            t2 := lastTimestamp
            offset := t2.Sub(masterTime)
            out <- source.OffsetMeasurement{Offset: offset, Delay: 0, SourceName: "ptp", Timestamp: time.Now()}
        }
    }
}

// helpers for timestamping

func connFd(u *net.UDPConn) uintptr {
    f, _ := u.File()
    return f.Fd()
}

var lastTimestamp time.Time

func recvmsgWithTstamp(conn *net.UDPConn, buf []byte) (int, []byte, *unix.Sockaddr, *unix.Timeval, error) {
    // use syscall.Recvmmsg? simplify: ReadFromUDP does not expose timestamp; use syscalls.
    fd := int(connFd(conn))
    var control [512]byte

    n, _, _, _, errno := syscall.Recvmsg(fd, buf, control[:], 0)
    if errno != nil {
        return 0, nil, nil, nil, errno
    }

    // parse control messages for SCM_TIMESTAMPING
    cms, _ := syscall.ParseSocketControlMessage(control[:])
    for _, cm := range cms {
        if cm.Header.Level == unix.SOL_SOCKET && cm.Header.Type == unix.SO_TIMESTAMPING {
            ts := (*[3]unix.Timespec)(unsafe.Pointer(&cm.Data[0]))
            // use software ts[2]
            lastTimestamp = time.Unix(int64(ts[2].Sec), int64(ts[2].Nsec))
        }
    }
    return n, nil, nil, nil, nil
}