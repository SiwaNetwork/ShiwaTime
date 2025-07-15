//go:build linux
// +build linux

package clock

import (
    "time"

    "golang.org/x/sys/unix"
)

func stepSystem(offset time.Duration) error {
    now := time.Now().Add(-offset)
    tv := unix.NsecToTimeval(now.UnixNano())
    return unix.Settimeofday(&tv)
}

func slewSystem(offset time.Duration) error {
    // Using adjtimex in frequency mode is complex; we do one-shot offset adjust using ADJ_SETOFFSET
    timex := unix.Timex{
        Modes:   unix.ADJ_SETOFFSET,
        Time:    unix.NsecToTimeval(offset.Nanoseconds()),
    }
    _, err := unix.Adjtimex(&timex)
    return err
}