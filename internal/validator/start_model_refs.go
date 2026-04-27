package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

func checkedModelSpan(off, ln uint32, size int) (start, end int, ok bool) {
	start = int(off)
	end = start + int(ln)
	if start < 0 || end < 0 || start > end || end > size {
		return 0, 0, false
	}
	return start, end, true
}

func dfaByRef(rt *runtime.Schema, ref runtime.ModelRef) (*runtime.DFAModel, error) {
	model, ok := rt.DFAModelByRef(ref)
	if !ok {
		return nil, fmt.Errorf("dfa model %d out of range", ref.ID)
	}
	return model, nil
}

func nfaByRef(rt *runtime.Schema, ref runtime.ModelRef) (*runtime.NFAModel, error) {
	model, ok := rt.NFAModelByRef(ref)
	if !ok {
		return nil, fmt.Errorf("nfa model %d out of range", ref.ID)
	}
	return model, nil
}

func allByRef(rt *runtime.Schema, ref runtime.ModelRef) (*runtime.AllModel, error) {
	model, ok := rt.AllModelByRef(ref)
	if !ok {
		return nil, fmt.Errorf("all model %d out of range", ref.ID)
	}
	return model, nil
}

func sliceDFATransitions(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFATransition, error) {
	start, end, ok := checkedModelSpan(rec.TransOff, rec.TransLen, len(model.Transitions))
	if !ok {
		return nil, fmt.Errorf("dfa transitions out of range")
	}
	return model.Transitions[start:end], nil
}

func sliceDFAWildcards(model *runtime.DFAModel, rec runtime.DFAState) ([]runtime.DFAWildcardEdge, error) {
	start, end, ok := checkedModelSpan(rec.WildOff, rec.WildLen, len(model.Wildcards))
	if !ok {
		return nil, fmt.Errorf("dfa wildcard edges out of range")
	}
	return model.Wildcards[start:end], nil
}

func allMemberAllowsSubst(rt *runtime.Schema, member runtime.AllMember, elem runtime.ElemID) bool {
	if rt == nil || member.SubstLen == 0 {
		return false
	}
	return slices.Contains(rt.AllSubstitutions(member.SubstOff, member.SubstLen), elem)
}

// LookupStartGlobalElement resolves one global element by its symbol.
func LookupStartGlobalElement(rt *runtime.Schema, sym runtime.SymbolID) (runtime.ElemID, bool) {
	return rt.GlobalElement(sym)
}

func element(rt *runtime.Schema, id runtime.ElemID) (runtime.Element, bool) {
	return rt.Element(id)
}
