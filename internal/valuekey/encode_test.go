package valuekey

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
)

func TestStringKeyBytes(t *testing.T) {
	got := StringKeyBytes(nil, 1, []byte("abc"))
	want := []byte{1, 'a', 'b', 'c'}
	if !bytes.Equal(got, want) {
		t.Fatalf("string key = %v, want %v", got, want)
	}
}

func TestStringKeyString(t *testing.T) {
	got := StringKeyString(2, "xyz")
	want := []byte{2, 'x', 'y', 'z'}
	if !bytes.Equal(got, want) {
		t.Fatalf("string key = %v, want %v", got, want)
	}
}

func TestQNameKeyBytes(t *testing.T) {
	key := QNameKeyStrings(3, "urn:ex", "local")
	tag, ns, local := decodeQNameKey(t, key)
	if tag != 3 {
		t.Fatalf("tag = %d, want 3", tag)
	}
	if ns != "urn:ex" {
		t.Fatalf("ns = %q, want %q", ns, "urn:ex")
	}
	if local != "local" {
		t.Fatalf("local = %q, want %q", local, "local")
	}
	canonical := []byte("urn:ex\x00local")
	alt := QNameKeyCanonical(nil, 3, canonical)
	if !bytes.Equal(key, alt) {
		t.Fatalf("canonical key mismatch: %v vs %v", key, alt)
	}
}

func TestFloatKeyBytes(t *testing.T) {
	key32 := Float32Key(nil, float32(math.NaN()), num.FloatNaN)
	if got := binary.BigEndian.Uint32(key32); got != canonicalNaN32 {
		t.Fatalf("float32 NaN bits = %#x, want %#x", got, canonicalNaN32)
	}
	negZero32 := Float32Key(nil, float32(math.Copysign(0, -1)), num.FloatFinite)
	if got := binary.BigEndian.Uint32(negZero32); got != 0 {
		t.Fatalf("float32 -0 bits = %#x, want 0", got)
	}

	key64 := Float64Key(nil, math.NaN(), num.FloatNaN)
	if got := binary.BigEndian.Uint64(key64); got != canonicalNaN64 {
		t.Fatalf("float64 NaN bits = %#x, want %#x", got, canonicalNaN64)
	}
	negZero64 := Float64Key(nil, math.Copysign(0, -1), num.FloatFinite)
	if got := binary.BigEndian.Uint64(negZero64); got != 0 {
		t.Fatalf("float64 -0 bits = %#x, want 0", got)
	}
}

func TestTemporalKeyBytes(t *testing.T) {
	withTZ := time.Date(2020, 1, 2, 3, 4, 5, 60000000, time.FixedZone("X", 2*3600))
	keyTZ := TemporalKeyBytes(nil, 5, withTZ, value.TZKnown, false)
	if len(keyTZ) != 20 {
		t.Fatalf("tz key len = %d, want 20", len(keyTZ))
	}
	if keyTZ[0] != 5 || keyTZ[1] != 1 {
		t.Fatalf("tz key header = %v, want [5 1]", keyTZ[:2])
	}
	utc := withTZ.UTC()
	year := int32(binary.BigEndian.Uint32(keyTZ[2:6]))
	month := binary.BigEndian.Uint16(keyTZ[6:8])
	day := binary.BigEndian.Uint16(keyTZ[8:10])
	hour := binary.BigEndian.Uint16(keyTZ[10:12])
	minute := binary.BigEndian.Uint16(keyTZ[12:14])
	sec := binary.BigEndian.Uint16(keyTZ[14:16])
	nanos := binary.BigEndian.Uint32(keyTZ[16:20])
	if year != 0 || month != uint16(utc.Month()) || day != uint16(utc.Day()) || hour != 0 || minute != 0 || sec != 0 || nanos != 0 {
		t.Fatalf("tz payload mismatch: %d-%02d-%02d %02d:%02d:%02d.%d vs gMonthDay %02d-%02dZ", year, month, day, hour, minute, sec, nanos, utc.Month(), utc.Day())
	}

	noTZ := time.Date(2021, 7, 9, 11, 12, 13, 14000000, time.UTC)
	keyNoTZ := TemporalKeyBytes(nil, 2, noTZ, value.TZNone, false)
	if len(keyNoTZ) != 11 {
		t.Fatalf("no-tz key len = %d, want 11", len(keyNoTZ))
	}
	if keyNoTZ[0] != 2 || keyNoTZ[1] != 0 {
		t.Fatalf("no-tz key header = %v, want [2 0]", keyNoTZ[:2])
	}
	seconds := binary.BigEndian.Uint32(keyNoTZ[2:6])
	nanos = binary.BigEndian.Uint32(keyNoTZ[6:10])
	wantSeconds := uint32(noTZ.Hour()*3600 + noTZ.Minute()*60 + noTZ.Second())
	if seconds != wantSeconds || nanos != uint32(noTZ.Nanosecond()) {
		t.Fatalf("no-tz payload mismatch")
	}

	withTZTime := time.Date(2020, 1, 2, 0, 30, 0, 0, time.FixedZone("X", 2*3600))
	keyTimeTZ := TemporalKeyBytes(nil, 2, withTZTime, value.TZKnown, false)
	if len(keyTimeTZ) != 11 {
		t.Fatalf("time tz key len = %d, want 11", len(keyTimeTZ))
	}
	if keyTimeTZ[0] != 2 || keyTimeTZ[1] != 1 {
		t.Fatalf("time tz key header = %v, want [2 1]", keyTimeTZ[:2])
	}
	seconds = binary.BigEndian.Uint32(keyTimeTZ[2:6])
	nanos = binary.BigEndian.Uint32(keyTimeTZ[6:10])
	utcSeconds := uint32(withTZTime.UTC().Hour()*3600 + withTZTime.UTC().Minute()*60 + withTZTime.UTC().Second())
	if seconds != utcSeconds || nanos != uint32(withTZTime.UTC().Nanosecond()) {
		t.Fatalf("time tz payload mismatch")
	}
}

func TestTemporalKeyBytesDateTimePreEpochOrdering(t *testing.T) {
	preEpoch, err := value.ParseDateTime([]byte("1969-12-31T23:59:59Z"))
	if err != nil {
		t.Fatalf("parse pre-epoch: %v", err)
	}
	epoch, err := value.ParseDateTime([]byte("1970-01-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("parse epoch: %v", err)
	}
	preKey := TemporalKeyBytes(nil, 0, preEpoch, value.TZKnown, false)
	epochKey := TemporalKeyBytes(nil, 0, epoch, value.TZKnown, false)
	if bytes.Compare(preKey, epochKey) >= 0 {
		t.Fatalf("expected pre-epoch key < epoch key")
	}
}

func TestTemporalKeyBytes_LeapSecondDistinct(t *testing.T) {
	leapTime, err := value.ParseTime([]byte("23:59:60Z"))
	if err != nil {
		t.Fatalf("parse leap time: %v", err)
	}
	midnightTime, err := value.ParseTime([]byte("00:00:00Z"))
	if err != nil {
		t.Fatalf("parse midnight time: %v", err)
	}
	leapTimeKey := TemporalKeyBytes(nil, 2, leapTime, value.TZKnown, true)
	midnightTimeKey := TemporalKeyBytes(nil, 2, midnightTime, value.TZKnown, false)
	if bytes.Equal(leapTimeKey, midnightTimeKey) {
		t.Fatalf("expected time leap-second key to differ from plain midnight key")
	}

	leapDateTime, err := value.ParseDateTime([]byte("1999-12-31T23:59:60Z"))
	if err != nil {
		t.Fatalf("parse leap dateTime: %v", err)
	}
	nextSecond, err := value.ParseDateTime([]byte("2000-01-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("parse next-second dateTime: %v", err)
	}
	leapDateTimeKey := TemporalKeyBytes(nil, 0, leapDateTime, value.TZKnown, true)
	nextSecondKey := TemporalKeyBytes(nil, 0, nextSecond, value.TZKnown, false)
	if bytes.Equal(leapDateTimeKey, nextSecondKey) {
		t.Fatalf("expected dateTime leap-second key to differ from next-second key")
	}
}

func TestDurationKeyBytes(t *testing.T) {
	zero, err := num.ParseDec([]byte("0"))
	if err != nil {
		t.Fatalf("parse zero: %v", err)
	}

	dur := types.XSDDuration{}
	key := DurationKeyBytes(nil, dur)
	want := []byte{0}
	want = num.EncodeIntKey(want, num.IntZero)
	want = num.EncodeDecKey(want, zero)
	if !bytes.Equal(key, want) {
		t.Fatalf("zero duration key = %v, want %v", key, want)
	}

	dur = types.XSDDuration{Years: 1, Months: 2}
	key = DurationKeyBytes(nil, dur)
	want = []byte{1}
	want = num.EncodeIntKey(want, num.FromInt64(14))
	want = num.EncodeDecKey(want, zero)
	if !bytes.Equal(key, want) {
		t.Fatalf("months duration key = %v, want %v", key, want)
	}

	secDec, err := num.ParseDec([]byte("1.5"))
	if err != nil {
		t.Fatalf("parse seconds: %v", err)
	}
	dur = types.XSDDuration{Seconds: secDec}
	key = DurationKeyBytes(nil, dur)
	want = []byte{1}
	want = num.EncodeIntKey(want, num.IntZero)
	want = num.EncodeDecKey(want, secDec)
	if !bytes.Equal(key, want) {
		t.Fatalf("seconds duration key = %v, want %v", key, want)
	}

	dur = types.XSDDuration{Negative: true}
	key = DurationKeyBytes(nil, dur)
	if len(key) == 0 || key[0] != 0 {
		t.Fatalf("negative zero sign = %v, want 0", key)
	}
}

func TestDurationKeyBytesPrecision(t *testing.T) {
	leftSec, err := num.ParseDec([]byte("0.123456789123456789"))
	if err != nil {
		t.Fatalf("parse left seconds: %v", err)
	}
	rightSec, err := num.ParseDec([]byte("0.123456789123456788"))
	if err != nil {
		t.Fatalf("parse right seconds: %v", err)
	}
	left := types.XSDDuration{Seconds: leftSec}
	right := types.XSDDuration{Seconds: rightSec}
	keyLeft := DurationKeyBytes(nil, left)
	keyRight := DurationKeyBytes(nil, right)
	if bytes.Equal(keyLeft, keyRight) {
		t.Fatalf("duration keys should differ for high-precision seconds")
	}
}

func TestDurationKeyBytesAvoidsOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	tested := false

	years := maxInt/12 + 1
	if int64(years) > math.MaxInt64/12 {
		tested = true
		dur := types.XSDDuration{Years: years}
		got := DurationKeyBytes(nil, dur)
		legacy := durationKeyBytesLegacy(nil, dur)
		if bytes.Equal(got, legacy) {
			t.Fatalf("expected overflow-safe months key to differ from legacy")
		}
	}

	days := maxInt/86400 + 1
	if int64(days) > math.MaxInt64/86400 {
		tested = true
		dur := types.XSDDuration{Days: days}
		got := DurationKeyBytes(nil, dur)
		legacy := durationKeyBytesLegacy(nil, dur)
		if bytes.Equal(got, legacy) {
			t.Fatalf("expected overflow-safe seconds key to differ from legacy")
		}
	}

	if !tested {
		t.Skip("int size too small to overflow int64 in legacy duration key path")
	}
}

func durationKeyBytesLegacy(dst []byte, dur types.XSDDuration) []byte {
	monthsTotal := int64(dur.Years)*12 + int64(dur.Months)
	months, _ := num.ParseInt([]byte(strconv.FormatInt(monthsTotal, 10)))
	seconds := legacyDurationSecondsTotal(dur)
	sign := byte(1)
	if dur.Negative {
		sign = 2
	}
	if monthsTotal == 0 && seconds.Sign == 0 {
		sign = 0
	}
	dst = append(dst[:0], sign)
	dst = num.EncodeIntKey(dst, months)
	dst = num.EncodeDecKey(dst, seconds)
	return dst
}

func legacyDurationSecondsTotal(dur types.XSDDuration) num.Dec {
	total := dur.Seconds
	total = legacyAddDecInt(total, int64(dur.Minutes)*60)
	total = legacyAddDecInt(total, int64(dur.Hours)*3600)
	total = legacyAddDecInt(total, int64(dur.Days)*86400)
	return total
}

func legacyAddDecInt(dec num.Dec, delta int64) num.Dec {
	if delta == 0 {
		return dec
	}
	scale := dec.Scale
	scaled := num.DecToScaledInt(dec, scale)
	deltaScaled := num.DecToScaledInt(num.FromInt64(delta).AsDec(), scale)
	sum := num.Add(scaled, deltaScaled)
	return num.DecFromScaledInt(sum, scale)
}

func decodeQNameKey(t *testing.T, key []byte) (byte, string, string) {
	t.Helper()
	if len(key) == 0 {
		t.Fatalf("empty key")
	}
	tag := key[0]
	rest := key[1:]
	nsLen, n := binary.Uvarint(rest)
	if n <= 0 {
		t.Fatalf("invalid ns length")
	}
	rest = rest[n:]
	if int(nsLen) > len(rest) {
		t.Fatalf("ns length out of range")
	}
	ns := string(rest[:nsLen])
	rest = rest[nsLen:]
	localLen, n := binary.Uvarint(rest)
	if n <= 0 {
		t.Fatalf("invalid local length")
	}
	rest = rest[n:]
	if int(localLen) > len(rest) {
		t.Fatalf("local length out of range")
	}
	local := string(rest[:localLen])
	return tag, ns, local
}
