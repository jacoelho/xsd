package valuekey

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

// StringKeyBytes writes a tagged string key into dst, reusing capacity; dst is overwritten from the start.
func StringKeyBytes(dst []byte, tag byte, data []byte) []byte {
	dst = append(dst[:0], tag)
	dst = append(dst, data...)
	return dst
}

// StringKeyString returns a tagged string key for a Go string.
func StringKeyString(tag byte, data string) []byte {
	out := make([]byte, 1+len(data))
	out[0] = tag
	copy(out[1:], data)
	return out
}

// BinaryKeyBytes writes a tagged binary key into dst, reusing capacity; dst is overwritten from the start.
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

// QNameKeyCanonical writes a tagged QName key into dst, reusing capacity; dst is overwritten from the start.
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
func TemporalKeyBytes(dst []byte, subkind byte, t time.Time, tzKind value.TimezoneKind, leapSecond bool) []byte {
	tzFlag := timezoneFlag(tzKind)
	if tzKind == value.TZKnown {
		t = t.UTC()
	}
	if subkind == 2 {
		seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()
		dst = ensureLen(dst[:0], 11)
		dst[0] = subkind
		dst[1] = tzFlag
		binary.BigEndian.PutUint32(dst[2:], uint32(seconds))
		binary.BigEndian.PutUint32(dst[6:], uint32(t.Nanosecond()))
		dst[10] = leapSecondFlag(leapSecond)
		return dst
	}
	year, month, day := t.Date()
	hour, minute, sec := t.Clock()
	nanos := t.Nanosecond()
	keyLen := 20
	if subkind == 0 {
		keyLen = 21
	}
	dst = ensureLen(dst[:0], keyLen)
	dst[0] = subkind
	dst[1] = tzFlag
	binary.BigEndian.PutUint32(dst[2:], uint32(int32(year)))
	binary.BigEndian.PutUint16(dst[6:], uint16(month))
	binary.BigEndian.PutUint16(dst[8:], uint16(day))
	binary.BigEndian.PutUint16(dst[10:], uint16(hour))
	binary.BigEndian.PutUint16(dst[12:], uint16(minute))
	binary.BigEndian.PutUint16(dst[14:], uint16(sec))
	binary.BigEndian.PutUint32(dst[16:], uint32(nanos))
	if subkind == 0 {
		dst[20] = leapSecondFlag(leapSecond)
	}
	return dst
}

// TemporalSubkind maps a temporal kind to its compact key subkind.
func TemporalSubkind(kind temporal.Kind) (byte, bool) {
	switch kind {
	case temporal.KindDateTime:
		return 0, true
	case temporal.KindDate:
		return 1, true
	case temporal.KindTime:
		return 2, true
	case temporal.KindGYearMonth:
		return 3, true
	case temporal.KindGYear:
		return 4, true
	case temporal.KindGMonthDay:
		return 5, true
	case temporal.KindGDay:
		return 6, true
	case temporal.KindGMonth:
		return 7, true
	default:
		return 0, false
	}
}

// TemporalKeyFromValue appends a canonical temporal key for v to dst.
func TemporalKeyFromValue(dst []byte, v temporal.Value) ([]byte, error) {
	subkind, ok := TemporalSubkind(v.Kind)
	if !ok {
		return nil, fmt.Errorf("unsupported temporal kind %d", v.Kind)
	}
	return TemporalKeyBytes(dst, subkind, v.Time, temporal.ValueTimezoneKind(v.TimezoneKind), v.LeapSecond), nil
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
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Minutes)), num.FromInt64(60)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Hours)), num.FromInt64(3600)))
	total = num.AddDecInt(total, num.Mul(num.FromInt64(int64(dur.Days)), num.FromInt64(86400)))
	return total
}

// AppendUvarint appends v as a varint-encoded uint64.
func AppendUvarint(dst []byte, v uint64) []byte {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], v)
	return append(dst, buf[:n]...)
}

// StartListKey resets dst and appends the list item count prefix.
func StartListKey(dst []byte, count int) []byte {
	return AppendUvarint(dst[:0], uint64(count))
}

// AppendListEntry appends one typed list item: kind (1 byte), len (uvarint), key bytes.
func AppendListEntry(dst []byte, kind byte, key []byte) []byte {
	dst = append(dst, kind)
	dst = AppendUvarint(dst, uint64(len(key)))
	return append(dst, key...)
}

func ensureLen(dst []byte, n int) []byte {
	if cap(dst) < n {
		return make([]byte, n)
	}
	return dst[:n]
}

func timezoneFlag(kind value.TimezoneKind) byte {
	switch kind {
	case value.TZKnown:
		return 1
	default:
		return 0
	}
}

func leapSecondFlag(leapSecond bool) byte {
	if leapSecond {
		return 1
	}
	return 0
}
