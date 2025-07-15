package clock

import (
    "log"
    "time"
)

// Step sets system clock by difference directly.
func Step(offset time.Duration) error {
    return stepSystem(offset)
}

// Slew gradually adjusts clock via small adjustment.
func Slew(offset time.Duration) error {
    return slewSystem(offset)
}

// ApplyOffset chooses between step and slew depending on magnitude vs boundary.
func ApplyOffset(offset, stepLimit time.Duration) error {
    abs := offset
    if abs < 0 {
        abs = -abs
    }
    if abs >= stepLimit {
        log.Printf("[clock] stepping clock by %s", offset)
        return Step(offset)
    }
    log.Printf("[clock] slewing clock by %s", offset)
    return Slew(offset)
}