package valruntime

// ResolveUnion applies one union match outcome to caller-owned result state and
// returns the final canonical value or error.
func ResolveUnion(out UnionOutcome, state *Result) ([]byte, error) {
	if out.Matched {
		applyUnionOutcome(state, out)
		return out.Canonical, nil
	}
	if out.PatternChecked {
		state.SetPatternChecked(true)
	}
	if out.FirstErr != nil {
		return nil, out.FirstErr
	}
	if out.SawValid {
		return nil, facet("enumeration violation")
	}
	return nil, invalid("union value does not match any member type")
}

func applyUnionOutcome(state *Result, out UnionOutcome) {
	if out.KeySet {
		state.SetKey(out.KeyKind, out.KeyBytes)
	}
	state.SetPatternChecked(out.PatternChecked)
	state.SetEnumChecked(out.EnumChecked)
	state.SetActual(out.ActualTypeID, out.ActualValidator)
}
