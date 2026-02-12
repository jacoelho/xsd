package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

func sliceDFATransitions(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFATransition, error) {
	off := rec.TransOff
	end := off + rec.TransLen
	if int(off) > len(model.Transitions) || int(end) > len(model.Transitions) {
		return nil, fmt.Errorf("dfa transitions out of range")
	}
	return model.Transitions[off:end], nil
}

func sliceDFAWildcards(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFAWildcardEdge, error) {
	off := rec.WildOff
	end := off + rec.WildLen
	if int(off) > len(model.Wildcards) || int(end) > len(model.Wildcards) {
		return nil, fmt.Errorf("dfa wildcard edges out of range")
	}
	return model.Wildcards[off:end], nil
}
