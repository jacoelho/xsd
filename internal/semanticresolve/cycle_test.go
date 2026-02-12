package semanticresolve

import (
	"errors"
	"fmt"
	"testing"

	model "github.com/jacoelho/xsd/internal/types"
)

func TestCycleDetectorEnterReturnsTypedCycleError(t *testing.T) {
	detector := NewCycleDetector[model.QName]()
	key := model.QName{Namespace: "urn:test", Local: "A"}

	if err := detector.Enter(key); err != nil {
		t.Fatalf("first enter should succeed, got %v", err)
	}

	err := detector.Enter(key)
	if err == nil {
		t.Fatalf("second enter should report cycle")
	}
	if !IsCycleError(err) {
		t.Fatalf("expected cycle error, got %v", err)
	}

	var cycleErr CycleError[model.QName]
	if !errors.As(err, &cycleErr) {
		t.Fatalf("expected typed cycle error, got %T", err)
	}
	if cycleErr.Key != key {
		t.Fatalf("cycle key = %s, want %s", cycleErr.Key, key)
	}
}

func TestIsCycleErrorDetectsWrappedCycleErrors(t *testing.T) {
	key := model.QName{Namespace: "urn:test", Local: "B"}
	err := fmt.Errorf("outer: %w", CycleError[model.QName]{Key: key})
	if !IsCycleError(err) {
		t.Fatalf("expected wrapped cycle error to be detected")
	}
}

func TestIsCycleErrorRejectsNonCycleErrors(t *testing.T) {
	if IsCycleError(errors.New("not a cycle")) {
		t.Fatalf("expected non-cycle error to be rejected")
	}
}
