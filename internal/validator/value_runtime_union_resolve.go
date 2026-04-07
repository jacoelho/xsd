package validator

func resolveUnion(out unionOutcome, state *ValueState) ([]byte, error) {
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

func applyUnionOutcome(state *ValueState, out unionOutcome) {
	if out.KeySet {
		state.SetKey(out.KeyKind, out.KeyBytes)
	}
	state.SetPatternChecked(out.PatternChecked)
	state.SetEnumChecked(out.EnumChecked)
	state.SetActual(out.ActualTypeID, out.ActualValidator)
}
