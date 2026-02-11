package valuesemantics

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestComparableForPrimitiveNameDecimalIntegerDerived(t *testing.T) {
	got, err := ComparableForPrimitiveName("decimal", "42", true)
	if err != nil {
		t.Fatalf("ComparableForPrimitiveName() error = %v", err)
	}
	if _, ok := got.(model.ComparableInt); !ok {
		t.Fatalf("value type = %T, want model.ComparableInt", got)
	}
}

func TestComparableForPrimitiveNameTemporal(t *testing.T) {
	got, err := ComparableForPrimitiveName("date", "2026-01-02Z", false)
	if err != nil {
		t.Fatalf("ComparableForPrimitiveName() error = %v", err)
	}
	if _, ok := got.(model.ComparableTime); !ok {
		t.Fatalf("value type = %T, want model.ComparableTime", got)
	}
}
