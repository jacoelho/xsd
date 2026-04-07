package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestLengthListCountsItems(t *testing.T) {
	t.Parallel()

	var cache ValueCache
	got, err := cache.Length(runtime.VList, []byte("a b  c"))
	if err != nil {
		t.Fatalf("Length() error = %v", err)
	}
	if got != 3 {
		t.Fatalf("Length() = %d, want 3", got)
	}
}

func TestSetListLengthSeedsCache(t *testing.T) {
	t.Parallel()

	var cache ValueCache
	cache.SetListLength(4)

	got, err := cache.Length(runtime.VList, []byte("ignored"))
	if err != nil {
		t.Fatalf("Length() error = %v", err)
	}
	if got != 4 {
		t.Fatalf("Length() = %d, want 4", got)
	}
}

func TestDigitCountsInvalidInteger(t *testing.T) {
	t.Parallel()

	var cache ValueCache
	_, _, err := cache.DigitCounts(runtime.VInteger, []byte("not-int"))
	if err == nil {
		t.Fatal("DigitCounts() error = nil, want datatype invalid")
	}
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("DigitCounts() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestSetDecimalSeedsDigitCounts(t *testing.T) {
	t.Parallel()

	val, perr := num.ParseDec([]byte("12.340"))
	if perr != nil {
		t.Fatalf("ParseDec() error = %v", perr)
	}

	var cache ValueCache
	cache.SetDecimal(val)

	total, fraction, err := cache.DigitCounts(runtime.VDecimal, nil)
	if err != nil {
		t.Fatalf("DigitCounts() error = %v", err)
	}
	if total != len(val.Coef) || fraction != int(val.Scale) {
		t.Fatalf("DigitCounts() = (%d, %d), want (%d, %d)", total, fraction, len(val.Coef), int(val.Scale))
	}
}
