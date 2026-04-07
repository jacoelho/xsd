package validator

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCheckDecimalSatisfied(t *testing.T) {
	t.Parallel()

	err := checkRuntimeRange(runtime.FMinInclusive, runtime.VDecimal, []byte("3"), []byte("2"), &ValueCache{})
	if err != nil {
		t.Fatalf("checkRuntimeRange() error = %v, want nil", err)
	}
}

func TestCheckFloatNaNViolatesFacet(t *testing.T) {
	t.Parallel()

	err := checkRuntimeRange(runtime.FMinInclusive, runtime.VDouble, []byte("NaN"), []byte("1"), &ValueCache{})
	if err == nil {
		t.Fatal("checkRuntimeRange() error = nil, want facet violation")
	}
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrFacetViolation {
		t.Fatalf("checkRuntimeRange() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrFacetViolation)
	}
}

func TestCheckTemporalInvalidBound(t *testing.T) {
	t.Parallel()

	err := checkRuntimeRange(runtime.FMinInclusive, runtime.VDate, []byte("2024-01-02"), []byte("bad"), nil)
	if err == nil {
		t.Fatal("checkRuntimeRange() error = nil, want datatype invalid")
	}
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("checkRuntimeRange() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestCheckUnsupportedKind(t *testing.T) {
	t.Parallel()

	err := checkRuntimeRange(runtime.FMinInclusive, runtime.VString, []byte("a"), []byte("b"), nil)
	if err == nil {
		t.Fatal("checkRuntimeRange() error = nil, want datatype invalid")
	}
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("checkRuntimeRange() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestCheckDecimalWithoutCache(t *testing.T) {
	t.Parallel()

	err := checkRuntimeRange(runtime.FMinInclusive, runtime.VDecimal, []byte("3"), []byte("2"), nil)
	if err != nil {
		t.Fatalf("checkRuntimeRange() error = %v, want nil", err)
	}
}
