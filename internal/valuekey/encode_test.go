package valuekey

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/types"
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
	keyTZ := TemporalKeyBytes(nil, 5, withTZ, true)
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
	keyNoTZ := TemporalKeyBytes(nil, 2, noTZ, false)
	if len(keyNoTZ) != 10 {
		t.Fatalf("no-tz key len = %d, want 10", len(keyNoTZ))
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
	keyTimeTZ := TemporalKeyBytes(nil, 2, withTZTime, true)
	if len(keyTimeTZ) != 10 {
		t.Fatalf("time tz key len = %d, want 10", len(keyTimeTZ))
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
