package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestResolveUnionMatchedUpdatesState(t *testing.T) {
	var state ValueState

	canonical, err := resolveUnion(
		unionOutcome{
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
		t.Fatalf("resolveUnion() error = %v", err)
	}
	if got := string(canonical); got != "12" {
		t.Fatalf("resolveUnion() canonical = %q, want %q", got, "12")
	}
	keyKind, keyBytes, ok := state.Key()
	if !ok || keyKind != runtime.VKDecimal || string(keyBytes) != "key" {
		t.Fatalf("resolveUnion() key = (%v, %q, %v), want (%v, %q, true)", keyKind, keyBytes, ok, runtime.VKDecimal, "key")
	}
	actualType, actualValidator := state.Actual()
	if actualType != 7 || actualValidator != 9 {
		t.Fatalf("resolveUnion() actual = (%d, %d), want (7, 9)", actualType, actualValidator)
	}
	if !state.PatternChecked() || !state.EnumChecked() {
		t.Fatal("resolveUnion() should mark pattern and enum checks")
	}
}

func TestResolveUnionEnumerationViolationMarksPatternCheck(t *testing.T) {
	var state ValueState

	canonical, err := resolveUnion(
		unionOutcome{
			SawValid:        true,
			PatternChecked:  true,
			ActualValidator: 3,
		},
		&state,
	)
	if err == nil || err.Error() != "enumeration violation" {
		t.Fatalf("resolveUnion() error = %v, want enumeration violation", err)
	}
	if canonical != nil {
		t.Fatalf("resolveUnion() canonical = %v, want nil", canonical)
	}
	if !state.PatternChecked() {
		t.Fatal("resolveUnion() should preserve pattern-checked state on failure")
	}
	if state.EnumChecked() {
		t.Fatal("resolveUnion() enum checked = true, want false")
	}
}

func TestResolveUnionReturnsFirstErrAndPatternState(t *testing.T) {
	var state ValueState

	wantErr := invalid("invalid integer")
	canonical, err := resolveUnion(
		unionOutcome{
			FirstErr:       wantErr,
			PatternChecked: true,
		},
		&state,
	)
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("resolveUnion() error = %v, want %v", err, wantErr)
	}
	if canonical != nil {
		t.Fatalf("resolveUnion() canonical = %v, want nil", canonical)
	}
	if !state.PatternChecked() {
		t.Fatal("resolveUnion() should preserve pattern-checked state before returning first error")
	}
}
