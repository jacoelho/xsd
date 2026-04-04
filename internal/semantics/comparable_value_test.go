package semantics

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestComparableForPrimitiveNameDecimalIntegerDerived(t *testing.T) {
	got, err := comparableForPrimitiveName("decimal", "42", true)
	if err != nil {
		t.Fatalf("comparableForPrimitiveName() error = %v", err)
	}
	if _, ok := got.(model.ComparableInt); !ok {
		t.Fatalf("value type = %T, want ComparableInt", got)
	}
}

func TestComparableForPrimitiveNameTemporal(t *testing.T) {
	got, err := comparableForPrimitiveName("date", "2026-01-02Z", false)
	if err != nil {
		t.Fatalf("comparableForPrimitiveName() error = %v", err)
	}
	if _, ok := got.(model.ComparableTime); !ok {
		t.Fatalf("value type = %T, want ComparableTime", got)
	}
}
