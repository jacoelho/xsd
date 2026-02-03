package valuekey

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/types"
)

// StringKeyBytes appends a tagged string key to dst.
func StringKeyBytes(dst []byte, tag byte, value []byte) []byte {
	dst = append(dst[:0], tag)
	dst = append(dst, value...)
	return dst
}

// StringKeyString returns a tagged string key for a Go string.
func StringKeyString(tag byte, value string) []byte {
	out := make([]byte, 1+len(value))
	out[0] = tag
	copy(out[1:], value)
	return out
}

// BinaryKeyBytes appends a tagged binary key to dst.
func BinaryKeyBytes(dst []byte, tag byte, data []byte) []byte {
	dst = append(dst[:0], tag)
	dst = append(dst, data...)
	return dst
}

// QNameKeyStrings returns a tagged QName key from namespace/local strings.
func QNameKeyStrings(tag byte, ns, local string) []byte {
	out := make([]byte, 0, 1+binary.MaxVarintLen64*2+len(ns)+len(local))
	out = append(out, tag)
	out = AppendUvarint(out, uint64(len(ns)))
	out = append(out, ns...)
	out = AppendUvarint(out, uint64(len(local)))
	out = append(out, local...)
	return out
}

// QNameKeyCanonical appends a tagged QName key from canonical bytes (ns\0local).
func QNameKeyCanonical(dst []byte, tag byte, canonical []byte) []byte {
	sep := bytes.IndexByte(canonical, 0)
	if sep < 0 {
		return nil
	}
	ns := canonical[:sep]
	local := canonical[sep+1:]
	dst = append(dst[:0], tag)
	dst = AppendUvarint(dst, uint64(len(ns)))
	dst = append(dst, ns...)
	dst = AppendUvarint(dst, uint64(len(local)))
	dst = append(dst, local...)
	return dst
}

const (
	canonicalNaN32 = 0x7fc00000
	canonicalNaN64 = 0x7ff8000000000000
)

// Float32Key appends the canonical float32 key encoding to dst.
func Float32Key(dst []byte, floatVal float32, class num.FloatClass) []byte {
	var bits uint32
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN32
	default:
		if floatVal == 0 {
			bits = 0
		} else {
			bits = math.Float32bits(floatVal)
		}
	}
	dst = ensureLen(dst[:0], 4)
	binary.BigEndian.PutUint32(dst, bits)
	return dst
}

// Float64Key appends the canonical float64 key encoding to dst.
func Float64Key(dst []byte, floatVal float64, class num.FloatClass) []byte {
	var bits uint64
	switch class {
	case num.FloatNaN:
		bits = canonicalNaN64
	default:
		if floatVal == 0 {
			bits = 0
		} else {
			bits = math.Float64bits(floatVal)
		}
	}
	dst = ensureLen(dst[:0], 8)
	binary.BigEndian.PutUint64(dst, bits)
	return dst
}

// TemporalKeyBytes appends a canonical temporal key encoding to dst.
func TemporalKeyBytes(dst []byte, subkind byte, t time.Time, hasTZ bool) []byte {
	if subkind == 2 {
		if hasTZ {
			t = t.UTC()
		}
		seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()
		dst = ensureLen(dst[:0], 10)
		dst[0] = subkind
		if hasTZ {
			dst[1] = 1
		} else {
			dst[1] = 0
		}
		binary.BigEndian.PutUint32(dst[2:], uint32(seconds))
		binary.BigEndian.PutUint32(dst[6:], uint32(t.Nanosecond()))
		return dst
	}
	if hasTZ {
		if subkind == 0 {
			utc := t.UTC()
			dst = ensureLen(dst[:0], 14)
			dst[0] = subkind
			dst[1] = 1
			binary.BigEndian.PutUint64(dst[2:], uint64(utc.Unix()))
			binary.BigEndian.PutUint32(dst[10:], uint32(utc.Nanosecond()))
			return dst
		}
		utc := t.UTC()
		year, month, day := utc.Date()
		hour, minute, sec := 0, 0, 0
		switch subkind {
		case 3: // gYearMonth
			day = 0
		case 4: // gYear
			month = 0
			day = 0
		case 5: // gMonthDay
			year = 0
		case 6: // gDay
			year = 0
			month = 0
		case 7: // gMonth
			year = 0
			day = 0
		}
		dst = ensureLen(dst[:0], 20)
		dst[0] = subkind
		dst[1] = 1
		binary.BigEndian.PutUint32(dst[2:], uint32(int32(year)))
		binary.BigEndian.PutUint16(dst[6:], uint16(month))
		binary.BigEndian.PutUint16(dst[8:], uint16(day))
		binary.BigEndian.PutUint16(dst[10:], uint16(hour))
		binary.BigEndian.PutUint16(dst[12:], uint16(minute))
		binary.BigEndian.PutUint16(dst[14:], uint16(sec))
		binary.BigEndian.PutUint32(dst[16:], 0)
		return dst
	}
	year, month, day := t.Date()
	hour, minute, sec := t.Clock()
	dst = ensureLen(dst[:0], 20)
	dst[0] = subkind
	dst[1] = 0
	binary.BigEndian.PutUint32(dst[2:], uint32(int32(year)))
	binary.BigEndian.PutUint16(dst[6:], uint16(month))
	binary.BigEndian.PutUint16(dst[8:], uint16(day))
	binary.BigEndian.PutUint16(dst[10:], uint16(hour))
	binary.BigEndian.PutUint16(dst[12:], uint16(minute))
	binary.BigEndian.PutUint16(dst[14:], uint16(sec))
	binary.BigEndian.PutUint32(dst[16:], uint32(t.Nanosecond()))
	return dst
}

// DurationKeyBytes appends a canonical duration key encoding to dst.
func DurationKeyBytes(dst []byte, dur types.XSDDuration) []byte {
	months := durationMonthsTotal(dur)
	seconds := durationSecondsTotal(dur)
	sign := byte(1)
	if dur.Negative {
		sign = 2
	}
	if months.Sign == 0 && seconds.Sign == 0 {
		sign = 0
	}
	dst = append(dst[:0], sign)
	dst = num.EncodeIntKey(dst, months)
	dst = num.EncodeDecKey(dst, seconds)
	return dst
}

func durationMonthsTotal(dur types.XSDDuration) num.Int {
	years := num.FromInt64(int64(dur.Years))
	months := num.FromInt64(int64(dur.Months))
	if years.Sign == 0 {
		return months
	}
	return num.Add(num.Mul(years, num.FromInt64(12)), months)
}

func durationSecondsTotal(dur types.XSDDuration) num.Dec {
	total := dur.Seconds
	total = addDecIntBig(total, num.Mul(num.FromInt64(int64(dur.Minutes)), num.FromInt64(60)))
	total = addDecIntBig(total, num.Mul(num.FromInt64(int64(dur.Hours)), num.FromInt64(3600)))
	total = addDecIntBig(total, num.Mul(num.FromInt64(int64(dur.Days)), num.FromInt64(86400)))
	return total
}

func addDecIntBig(dec num.Dec, delta num.Int) num.Dec {
	if delta.Sign == 0 {
		return dec
	}
	scale := dec.Scale
	scaled := num.DecToScaledInt(dec, scale)
	if scale != 0 {
		delta = num.Mul(delta, pow10Int(scale))
	}
	sum := num.Add(scaled, delta)
	return num.DecFromScaledInt(sum, scale)
}

func pow10Int(scale uint32) num.Int {
	if scale == 0 {
		return num.FromInt64(1)
	}
	digits := make([]byte, int(scale)+1)
	digits[0] = '1'
	for i := 1; i < len(digits); i++ {
		digits[i] = '0'
	}
	out, _ := num.ParseInt(digits)
	return out
}

// AppendUvarint appends v as a varint-encoded uint64.
func AppendUvarint(dst []byte, v uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	return append(dst, buf[:n]...)
}

func ensureLen(dst []byte, n int) []byte {
	if cap(dst) < n {
		return make([]byte, n)
	}
	return dst[:n]
}
