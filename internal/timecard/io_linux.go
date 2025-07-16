//go:build linux
// +build linux

// Package timecard provides low-level helpers for the OCP Time Appliance Project
// (time-card) PCIe device.  This file implements a minimal mmap-based driver
// that maps BAR0 (resource0) into user space so higher-level code can read / write
// control and status registers.
//
// The implementation purposefully keeps the surface small:
//   • OpenPCI(addr) – maps <sysfs>/resource0 and returns a Driver.
//   • Driver interface exposes 32-bit aligned Read / Write helpers and Close.
//
// NOTE:  In production environments it is preferable to use a dedicated UIO
//        driver or character device with proper privileges, but mmap of
//        resource0 works on most systems when run as root or with CAP_SYS_RAWIO.
package timecard

import (
    "encoding/binary"
    "fmt"
    "os"
    "sync"
    "syscall"
)

// Driver abstracts raw register access.  Offsets are interpreted as BAR0 byte
// offsets.  All operations are little-endian to match the time-card register
// layout.
//
// The implementation intentionally keeps only 32-bit accesses – higher-level
// code can assemble wider fields if needed.
//
// Offsets MUST be 4-byte aligned; otherwise ReadU32 panics.
//
// Close must be called to unmap the memory.
//
// The interface lives in this file so platform-specific back-ends can be
// swapped without touching importing code.
type Driver interface {
    ReadU32(off uint32) uint32
    WriteU32(off uint32, val uint32)
    Close() error
}

// OpenPCI maps BAR0 of the device given by <addr> in PCI domain format
// (e.g. "0000:65:00.0").  Internally it mmaps
//   /sys/bus/pci/devices/<addr>/resource0
// The returned Driver is safe for concurrent access.
func OpenPCI(addr string) (Driver, error) {
    path := fmt.Sprintf("/sys/bus/pci/devices/%s/resource0", addr)
    f, err := os.OpenFile(path, os.O_RDWR, 0)
    if err != nil {
        return nil, fmt.Errorf("timecard: open %s: %w", path, err)
    }

    // Get size via fstat – resource0 usually maps full BAR length.
    fi, err := f.Stat()
    if err != nil {
        f.Close()
        return nil, fmt.Errorf("timecard: stat resource0: %w", err)
    }
    size := fi.Size()
    if size == 0 {
        f.Close()
        return nil, fmt.Errorf("timecard: resource0 size 0")
    }

    data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
    if err != nil {
        f.Close()
        return nil, fmt.Errorf("timecard: mmap: %w", err)
    }

    drv := &pciDriver{
        file: f,
        mem:  data,
        size: uint32(size),
    }
    return drv, nil
}

// pciDriver implements Driver using mmap'd resource0.
// All accesses are guarded by a mutex to keep them atomic w.r.t other goroutines.
type pciDriver struct {
    file *os.File
    mem  []byte
    size uint32
    mu   sync.Mutex
}

func (d *pciDriver) ReadU32(off uint32) uint32 {
    if off%4 != 0 {
        panic("timecard: ReadU32 offset not aligned")
    }
    if off+4 > d.size {
        panic("timecard: ReadU32 out of range")
    }
    d.mu.Lock()
    val := binary.LittleEndian.Uint32(d.mem[off : off+4])
    d.mu.Unlock()
    return val
}

func (d *pciDriver) WriteU32(off uint32, val uint32) {
    if off%4 != 0 {
        panic("timecard: WriteU32 offset not aligned")
    }
    if off+4 > d.size {
        panic("timecard: WriteU32 out of range")
    }
    d.mu.Lock()
    binary.LittleEndian.PutUint32(d.mem[off:off+4], val)
    d.mu.Unlock()
}

func (d *pciDriver) Close() error {
    d.mu.Lock()
    defer d.mu.Unlock()
    if d.mem != nil {
        syscall.Munmap(d.mem)
        d.mem = nil
    }
    if d.file != nil {
        _ = d.file.Close()
        d.file = nil
    }
    return nil
}