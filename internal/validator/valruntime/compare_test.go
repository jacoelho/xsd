package valruntime

import (
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

func TestCheckDecimalSatisfied(t *testing.T) {
	t.Parallel()

	err := Check(runtime.FMinInclusive, runtime.VDecimal, []byte("3"), []byte("2"), Loader{
		Decimal: func(b []byte) (num.Dec, error) {
			val, parseErr := num.ParseDec(b)
			if parseErr != nil {
				return num.Dec{}, diag.Invalid("invalid decimal")
			}
			return val, nil
		},
	})
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
}

func TestCheckFloatNaNViolatesFacet(t *testing.T) {
	t.Parallel()

	err := Check(runtime.FMinInclusive, runtime.VDouble, []byte("NaN"), []byte("1"), Loader{
		Float64: func([]byte) (float64, num.FloatClass, error) {
			return 0, num.FloatNaN, nil
		},
	})
	if err == nil {
		t.Fatal("Check() error = nil, want facet violation")
	}
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrFacetViolation {
		t.Fatalf("Check() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrFacetViolation)
	}
}

func TestCheckTemporalInvalidBound(t *testing.T) {
	t.Parallel()

	err := Check(runtime.FMinInclusive, runtime.VDate, []byte("2024-01-02"), []byte("bad"), Loader{})
	if err == nil {
		t.Fatal("Check() error = nil, want datatype invalid")
	}
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("Check() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestCheckUnsupportedKind(t *testing.T) {
	t.Parallel()

	err := Check(runtime.FMinInclusive, runtime.VString, []byte("a"), []byte("b"), Loader{})
	if err == nil {
		t.Fatal("Check() error = nil, want datatype invalid")
	}
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("Check() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}
