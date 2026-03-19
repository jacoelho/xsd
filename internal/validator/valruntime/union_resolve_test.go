package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestResolveUnionMatchedUpdatesState(t *testing.T) {
	var state Result

	canonical, err := ResolveUnion(
		UnionOutcome{
			Canonical:       []byte("12"),
			KeyBytes:        []byte("key"),
			KeyKind:         runtime.VKDecimal,
			KeySet:          true,
			Matched:         true,
			PatternChecked:  true,
			EnumChecked:     true,
			ActualTypeID:    7,
			ActualValidator: 9,
		},
		&state,
	)
	if err != nil {
		t.Fatalf("ResolveUnion() error = %v", err)
	}
	if got := string(canonical); got != "12" {
		t.Fatalf("ResolveUnion() canonical = %q, want %q", got, "12")
	}
	keyKind, keyBytes, ok := state.Key()
	if !ok || keyKind != runtime.VKDecimal || string(keyBytes) != "key" {
		t.Fatalf("ResolveUnion() key = (%v, %q, %v), want (%v, %q, true)", keyKind, keyBytes, ok, runtime.VKDecimal, "key")
	}
	actualType, actualValidator := state.Actual()
	if actualType != 7 || actualValidator != 9 {
		t.Fatalf("ResolveUnion() actual = (%d, %d), want (7, 9)", actualType, actualValidator)
	}
	if !state.PatternChecked() || !state.EnumChecked() {
		t.Fatal("ResolveUnion() should mark pattern and enum checks")
	}
}

func TestResolveUnionEnumerationViolationMarksPatternCheck(t *testing.T) {
	var state Result

	canonical, err := ResolveUnion(
		UnionOutcome{
			SawValid:        true,
			PatternChecked:  true,
			ActualValidator: 3,
		},
		&state,
	)
	if err == nil || err.Error() != "enumeration violation" {
		t.Fatalf("ResolveUnion() error = %v, want enumeration violation", err)
	}
	if canonical != nil {
		t.Fatalf("ResolveUnion() canonical = %v, want nil", canonical)
	}
	if !state.PatternChecked() {
		t.Fatal("ResolveUnion() should preserve pattern-checked state on failure")
	}
	if state.EnumChecked() {
		t.Fatal("ResolveUnion() enum checked = true, want false")
	}
}

func TestResolveUnionReturnsFirstErrAndPatternState(t *testing.T) {
	var state Result

	wantErr := invalid("invalid integer")
	canonical, err := ResolveUnion(
		UnionOutcome{
			FirstErr:       wantErr,
			PatternChecked: true,
		},
		&state,
	)
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("ResolveUnion() error = %v, want %v", err, wantErr)
	}
	if canonical != nil {
		t.Fatalf("ResolveUnion() canonical = %v, want nil", canonical)
	}
	if !state.PatternChecked() {
		t.Fatal("ResolveUnion() should preserve pattern-checked state before returning first error")
	}
}
