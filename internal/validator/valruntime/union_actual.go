package valruntime

import "github.com/jacoelho/xsd/internal/runtime"

// ActualUnionValidator returns the selected union member validator from caller-owned
// result state when present.
func ActualUnionValidator(state *Result) runtime.ValidatorID {
	if state == nil {
		return 0
	}
	_, actual := state.Actual()
	return actual
}

// ResolveActualUnionValidator returns the selected union member validator from
// caller-owned result state, falling back to lookup when the state is unset.
func ResolveActualUnionValidator(state *Result, lookup func() (runtime.ValidatorID, error)) runtime.ValidatorID {
	if actual := ActualUnionValidator(state); actual != 0 {
		return actual
	}
	if lookup == nil {
		return 0
	}
	actual, err := lookup()
	if err != nil {
		return 0
	}
	return actual
}
