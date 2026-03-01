package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func sliceDFATransitions(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFATransition, error) {
	start, end, ok := checkedSpan(rec.TransOff, rec.TransLen, len(model.Transitions))
	if !ok {
		return nil, fmt.Errorf("dfa transitions out of range")
	}
	return model.Transitions[start:end], nil
}

func sliceDFAWildcards(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFAWildcardEdge, error) {
	start, end, ok := checkedSpan(rec.WildOff, rec.WildLen, len(model.Wildcards))
	if !ok {
		return nil, fmt.Errorf("dfa wildcard edges out of range")
	}
	return model.Wildcards[start:end], nil
}
