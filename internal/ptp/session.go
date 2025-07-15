package ptp

import (
    "context"
    "encoding/binary"
    "math/rand"
    "net"
    "syscall"
    "time"
    "unsafe"

    "golang.org/x/sys/unix"

    "shiwa/internal/source"
)

// ClockState represents simplified OC states.
const (
    StateInit = iota
    StateMaster
    StateSlave
)

type Session struct {
    iface string
    domain uint8

    state int

    connEvent *net.UDPConn // port 319 event
    connGen   *net.UDPConn // port 320 general

    seq uint16

    // last master dataset
    masterDS AnnounceDataSet
    masterAddr *net.UDPAddr

    pathDelay time.Duration
}

func NewSession(iface string, domain uint8) *Session {
    return &Session{iface: iface, domain: domain, state: StateInit}
}

func (s *Session) Start(ctx context.Context) (<-chan source.OffsetMeasurement, error) {
    ifi, err := net.InterfaceByName(s.iface)
    if err != nil {
        return nil, err
    }

    // event messages (sync, delayreq) 319
    evAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 319}
    connEv, err := net.ListenUDP("udp4", evAddr)
    if err != nil {
        return nil, err
    }
    // general messages 320
    genAddr := &net.UDPAddr{IP: net.IPv4zero, Port: 320}
    connGen, err := net.ListenUDP("udp4", genAddr)
    if err != nil {
        return nil, err
    }

    // join multicast 224.0.1.129 on both sockets
    maddr := net.IPv4(224, 0, 1, 129).To4()
    joinIPv4Multicast(int(connFd(connEv)), maddr, ifi.Index)
    joinIPv4Multicast(int(connFd(connGen)), maddr, ifi.Index)

    enableHWTimestamps(int(connFd(connEv)))
    enableHWTimestamps(int(connFd(connGen)))

    s.connEvent = connEv
    s.connGen = connGen
    s.seq = uint16(rand.Uint32())

    out := make(chan source.OffsetMeasurement)
    go s.run(ctx, out)
    return out, nil
}

func joinIPv4Multicast(fd int, maddr net.IP, ifindex int) {
    var mreq unix.IPMreqn
    copy(mreq.Multiaddr[:], maddr.To4())
    mreq.Ifindex = int32(ifindex)
    _ = unix.SetsockoptIPMreqn(fd, unix.IPPROTO_IP, unix.IP_ADD_MEMBERSHIP, &mreq)
}

// enableHWTimestamps tries to enable hardware timestamping; falls back to software.
func enableHWTimestamps(fd int) {
    flags := unix.SOF_TIMESTAMPING_RX_HARDWARE | unix.SOF_TIMESTAMPING_TX_HARDWARE | unix.SOF_TIMESTAMPING_RAW_HARDWARE | unix.SOF_TIMESTAMPING_SOFTWARE | unix.SOF_TIMESTAMPING_TX_SOFTWARE | unix.SOF_TIMESTAMPING_RX_SOFTWARE
    unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_TIMESTAMPING, flags)
}

// connFd converts UDPConn to int FD.
func connFd(c *net.UDPConn) int {
    f, _ := c.File()
    return int(f.Fd())
}

func (s *Session) run(ctx context.Context, out chan<- source.OffsetMeasurement) {
    defer close(out)
    go s.delayRequestLoop(ctx)

    buf := make([]byte, 1500)
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }
        n, addr, ts, err := recvPTP(s.connEvent, buf)
        if err != nil || n < headerSize {
            continue
        }
        var hdr Header
        _ = hdr.UnmarshalBinary(buf[:headerSize])
        switch hdr.MessageType() {
        case MsgSync:
            s.seq = hdr.SequenceID
            // store t2
            t2 := ts
            // wait for followUp
            n2, _, _, _ := recvPTP(s.connGen, buf)
            if n2 < headerSize {
                continue
            }
            var fh Header
            _ = fh.UnmarshalBinary(buf[:headerSize])
            if fh.MessageType() != MsgFollowUp || fh.SequenceID != s.seq {
                continue
            }
            originSec := uint64(buf[34])<<40 | uint64(buf[35])<<32 | uint64(buf[36])<<24 | uint64(buf[37])<<16 | uint64(buf[38])<<8 | uint64(buf[39])
            originNano := binary.BigEndian.Uint32(buf[40:44])
            t1 := time.Unix(int64(originSec), int64(originNano))
            offset := t2.Sub(t1) - s.pathDelay/2
            out <- source.OffsetMeasurement{Offset: offset, Delay: s.pathDelay, SourceName: "ptp", Timestamp: time.Now()}
            s.masterAddr = addr
        case MsgDelayResp:
            // t4 (this message receive ts), contains t3 in body
            t4 := ts
            // requestReceiptTimestamp starts at byte 34
            rsec := uint64(buf[34])<<40 | uint64(buf[35])<<32 | uint64(buf[36])<<24 | uint64(buf[37])<<16 | uint64(buf[38])<<8 | uint64(buf[39])
            rnano := binary.BigEndian.Uint32(buf[40:44])
            t3 := time.Unix(int64(rsec), int64(rnano))
            s.pathDelay = (t4.Sub(t3) - s.pathDelay) // simple update
        case MsgAnnounce:
            // parse announce dataset and run BMCA
            var ann AnnounceDataSet
            ann.Priority1 = buf[48]
            ann.ClockQuality.Class = buf[49]
            ann.ClockQuality.Accuracy = buf[50]
            ann.ClockQuality.Variance = binary.BigEndian.Uint16(buf[51:53])
            ann.Priority2 = buf[53]
            copy(ann.GrandmasterIdentity[:], buf[54:62])
            if s.state == StateInit {
                s.masterDS = ann
                s.state = StateSlave
            } else if ann.betterThan(s.masterDS) {
                s.masterDS = ann
                s.state = StateSlave
            }
        }
    }
}

func (s *Session) delayRequestLoop(ctx context.Context) {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    buf := make([]byte, headerSize+10)
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if s.state != StateSlave || s.masterAddr == nil {
                continue
            }
            // build DelayReq message
            hdr := Header{
                TransportSpecific: MsgDelayReq,
                Version:           0x2,
                MessageLength:    uint16(len(buf)),
                DomainNumber:     s.domain,
                SequenceID:       s.seq,
                ControlField:     0x01,
            }
            h, _ := hdr.MarshalBinary()
            copy(buf, h)
            // originTimestamp left zero, will be filled by tx timestamp
            s.connEvent.WriteToUDP(buf, s.masterAddr)
        }
    }
}

// recvPTP reads a datagram and extracts software RX timestamp (ns).
func recvPTP(c *net.UDPConn, buf []byte) (int, *net.UDPAddr, time.Time, error) {
    oob := make([]byte, 512)
    n, oobn, _, addr, err := c.ReadMsgUDP(buf, oob)
    if err != nil {
        return 0, nil, time.Time{}, err
    }
    // parse SCM_TIMESTAMPING
    msgs, _ := syscall.ParseSocketControlMessage(oob[:oobn])
    for _, m := range msgs {
        if m.Header.Level == unix.SOL_SOCKET && m.Header.Type == unix.SO_TIMESTAMPING {
            ts := (*[3]unix.Timespec)(unsafe.Pointer(&m.Data[0]))
            // pick RAW_HARDWARE if non-zero, else software
            t := time.Unix(int64(ts[2].Sec), int64(ts[2].Nsec))
            if ts[2].Sec == 0 && ts[2].Nsec == 0 {
                t = time.Unix(int64(ts[0].Sec), int64(ts[0].Nsec))
            }
            return n, addr, t, nil
        }
    }
    return n, addr, time.Now(), nil
}