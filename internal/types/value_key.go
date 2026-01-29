package types

import (
	"encoding/binary"
	"math"
)

// CanonicalDurationKey returns deterministic value-space bytes for an XSD duration.
func CanonicalDurationKey(value XSDDuration, dst []byte) []byte {
	months := value.Years*12 + value.Months
	seconds := float64(value.Days)*86400 +
		float64(value.Hours)*3600 +
		float64(value.Minutes)*60 +
		value.Seconds
	if value.Negative {
		months = -months
		seconds = -seconds
	}
	if months == 0 && seconds == 0 {
		seconds = 0
	}
	out := dst[:0]
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutVarint(buf[:], int64(months))
	out = append(out, buf[:n]...)
	var fbuf [8]byte
	binary.BigEndian.PutUint64(fbuf[:], math.Float64bits(seconds))
	out = append(out, fbuf[:]...)
	return out
}
