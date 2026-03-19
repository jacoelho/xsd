package valruntime

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestActualUnionValidator(t *testing.T) {
	var state Result
	state.SetActual(7, 11)

	if got := ActualUnionValidator(&state); got != 11 {
		t.Fatalf("ActualUnionValidator() = %d, want %d", got, 11)
	}
	if got := ActualUnionValidator(nil); got != 0 {
		t.Fatalf("ActualUnionValidator(nil) = %d, want 0", got)
	}
}

func TestResolveActualUnionValidatorUsesCachedState(t *testing.T) {
	var state Result
	state.SetActual(0, 19)

	called := false
	got := ResolveActualUnionValidator(&state, func() (runtime.ValidatorID, error) {
		called = true
		return 23, nil
	})
	if got != 19 {
		t.Fatalf("ResolveActualUnionValidator() = %d, want %d", got, 19)
	}
	if called {
		t.Fatal("ResolveActualUnionValidator() called lookup for cached state")
	}
}

func TestResolveActualUnionValidatorFallsBackToLookup(t *testing.T) {
	called := false
	got := ResolveActualUnionValidator(nil, func() (runtime.ValidatorID, error) {
		called = true
		return 29, nil
	})
	if got != 29 {
		t.Fatalf("ResolveActualUnionValidator() = %d, want %d", got, 29)
	}
	if !called {
		t.Fatal("ResolveActualUnionValidator() did not call lookup")
	}
}

func TestResolveActualUnionValidatorReturnsZeroOnLookupError(t *testing.T) {
	got := ResolveActualUnionValidator(nil, func() (runtime.ValidatorID, error) {
		return 0, errors.New("boom")
	})
	if got != 0 {
		t.Fatalf("ResolveActualUnionValidator() = %d, want 0", got)
	}
}
