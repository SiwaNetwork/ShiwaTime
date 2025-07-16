//go:build linux
// +build linux

package timecard

// SysV SHM writer for chrony / ntpd "type 28" shared memory refclock.
// Implementation is minimal: only version 0 (mode=0) layout is written.
// Reference: chrony documentation and xntpd shm reference.

import (
    "fmt"
    "syscall"
    "time"
    "unsafe"

    "golang.org/x/sys/unix"
)

// shmTime mirrors struct shmTime from ntpd/chrony (mode 0, 96 bytes)
type shmTime struct {
    Mode int32
    Count int32
    ClockSec int32
    ClockUSec int32
    ReceiveSec int32
    ReceiveUSec int32
    Leap int32
    Precision int32
    Nsamp int32
    Valid int32
    Pad [10]uint32
}

const (
    shmKeyBase = 0x4e545030 // "NTP0"
)

type ShmWriter struct {
    id   int
    data *shmTime
}

func OpenShm(segment int) (*ShmWriter, error) {
    key := shmKeyBase + segment
    id, _, errno := syscall.Syscall(syscall.SYS_SHMGET, uintptr(key), uintptr(unsafe.Sizeof(shmTime{})), uintptr(unix.IPC_CREAT|0600))
    if errno != 0 {
        return nil, fmt.Errorf("shmget: %v", errno)
    }
    addr, _, errno := syscall.Syscall(syscall.SYS_SHMAT, id, 0, 0)
    if errno != 0 {
        return nil, fmt.Errorf("shmat: %v", errno)
    }
    return &ShmWriter{id: int(id), data: (*shmTime)(unsafe.Pointer(addr))}, nil
}

func (w *ShmWriter) Write(t time.Time) {
    if w == nil || w.data == nil {
        return
    }
    usec := t.Nanosecond() / 1000
    w.data.Mode = 0
    w.data.Count++
    w.data.Valid = 0
    w.data.ClockSec = int32(t.Unix())
    w.data.ClockUSec = int32(usec)
    w.data.ReceiveSec = w.data.ClockSec
    w.data.ReceiveUSec = w.data.ClockUSec
    w.data.Leap = 0
    w.data.Precision = -20
    w.data.Nsamp = 1
    w.data.Valid = 1
    w.data.Count++
}

func (w *ShmWriter) Close() {
    if w == nil || w.data == nil {
        return
    }
    addr := uintptr(unsafe.Pointer(w.data))
    _, _, errno := syscall.Syscall(syscall.SYS_SHMDT, addr, 0, 0)
    if errno != 0 {
        // Ignore error on close
    }
    // do not remove segment, let chrony reuse across restarts
}