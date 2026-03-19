package model

import (
	"testing"
)

func TestComparableForPrimitiveNameDecimalIntegerDerived(t *testing.T) {
	got, err := ComparableForPrimitiveName("decimal", "42", true)
	if err != nil {
		t.Fatalf("ComparableForPrimitiveName() error = %v", err)
	}
	if _, ok := got.(ComparableInt); !ok {
		t.Fatalf("value type = %T, want ComparableInt", got)
	}
}

func TestComparableForPrimitiveNameTemporal(t *testing.T) {
	got, err := ComparableForPrimitiveName("date", "2026-01-02Z", false)
	if err != nil {
		t.Fatalf("ComparableForPrimitiveName() error = %v", err)
	}
	if _, ok := got.(ComparableTime); !ok {
		t.Fatalf("value type = %T, want ComparableTime", got)
	}
}
