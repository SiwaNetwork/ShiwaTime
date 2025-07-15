package ptp

// ClockQuality represents PTP ClockQuality struct.
type ClockQuality struct {
    Class    uint8
    Accuracy uint8
    Variance uint16
}

type AnnounceDataSet struct {
    Priority1 uint8
    ClockQuality ClockQuality
    Priority2 uint8
    GrandmasterIdentity [8]byte
}

// betterThan compares two announce datasets according to IEEE1588 BMCA section 9.3.5.
func (a AnnounceDataSet) betterThan(b AnnounceDataSet) bool {
    if a.Priority1 != b.Priority1 {
        return a.Priority1 < b.Priority1
    }
    if a.ClockQuality.Class != b.ClockQuality.Class {
        return a.ClockQuality.Class < b.ClockQuality.Class
    }
    if a.ClockQuality.Accuracy != b.ClockQuality.Accuracy {
        return a.ClockQuality.Accuracy < b.ClockQuality.Accuracy
    }
    if a.ClockQuality.Variance != b.ClockQuality.Variance {
        return a.ClockQuality.Variance < b.ClockQuality.Variance
    }
    if a.Priority2 != b.Priority2 {
        return a.Priority2 < b.Priority2
    }
    // finally compare identities lexicographically (lower is better)
    return string(a.GrandmasterIdentity[:]) < string(b.GrandmasterIdentity[:])
}