package num

import (
	"bytes"
	"math"
	"testing"
	"testing/quick"
)

func TestQuickIntRoundTrip(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000}
	err := quick.Check(func(v int64) bool {
		if v == math.MinInt64 {
			return true
		}
		got, err := ParseInt(FromInt64(v).RenderCanonical(nil))
		if err != nil {
			return false
		}
		want := FromInt64(v)
		return got.Sign == want.Sign && bytes.Equal(got.Digits, want.Digits)
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuickIntCompare(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000}
	err := quick.Check(func(a, b int64) bool {
		if a == math.MinInt64 || b == math.MinInt64 {
			return true
		}
		got := FromInt64(a).Compare(FromInt64(b))
		want := cmpInt64(a, b)
		return got == want
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuickIntAddMul(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000}
	err := quick.Check(func(a, b int32) bool {
		wantAdd := int64(a) + int64(b)
		gotAdd := Add(FromInt64(int64(a)), FromInt64(int64(b)))
		if gotAdd.Compare(FromInt64(wantAdd)) != 0 {
			return false
		}
		wantMul := int64(a) * int64(b)
		gotMul := Mul(FromInt64(int64(a)), FromInt64(int64(b)))
		return gotMul.Compare(FromInt64(wantMul)) == 0
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func TestQuickDecRoundTrip(t *testing.T) {
	cfg := &quick.Config{MaxCount: 1000}
	err := quick.Check(func(v int64, scale uint8) bool {
		if v == math.MinInt64 {
			return true
		}
		sign := int8(1)
		abs := v
		if abs < 0 {
			sign = -1
			abs = -abs
		}
		coef := FromInt64(abs).Digits
		if v == 0 {
			sign = 0
			coef = zeroDigits
		}
		dec := Dec{Sign: sign, Coef: coef, Scale: uint32(scale % 9)}
		canonical := dec.RenderCanonical(nil)
		parsed, err := ParseDec(canonical)
		if err != nil {
			return false
		}
		return dec.Compare(parsed) == 0
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
}

func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
