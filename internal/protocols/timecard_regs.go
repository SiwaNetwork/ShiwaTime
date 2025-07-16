package protocols

// Register offsets for OCP time-card BAR0 (little-endian).  The list is a
// subset sufficient for GNSS / PPS functionality.  Offsets follow the
// definitions in the official driver (see TAP Time-Card ‘DRV’ repo).
// They are kept here instead of a dedicated sub-package to avoid circular
// deps.

const (
    // ITU-T ToD counter: nanoseconds + seconds (UTC)
    tcRegTodNs   = 0x0000 // uint32, nanoseconds [0-1e9)
    tcRegTodSecL = 0x0004 // uint32, lower 32 bits of seconds since epoch
    tcRegTodSecH = 0x0008 // uint32, upper 16 bits (use 48-bit seconds)

    // PPS registers
    tcRegPpsCountL = 0x0010 // uint32 – lower 32 bits of PPS counter
    tcRegPpsCountH = 0x0014 // uint32 – upper 32 bits
    tcRegPpsLastNs = 0x0018 // uint32 – nanoseconds of last PPS edge

    // GNSS registers (u-blox NMEA mirror)
    tcRegGnssFix   = 0x0020 // uint32 – bit0: valid, bits3:1 fix type (0-5)
    tcRegGnssLat   = 0x0024 // int32  – scaled degrees*1e7
    tcRegGnssLon   = 0x0028 // int32  – scaled degrees*1e7
    tcRegGnssAlt   = 0x002C // int32  – mm
    tcRegGnssSats  = 0x0030 // uint32 – bits7:0 sats used, bits15:8 sats view
)