package model

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

func checkedSpan(off, ln uint32, size int) (start, end int, ok bool) {
	start = int(off)
	end = start + int(ln)
	if start < 0 || end < 0 || start > end || end > size {
		return 0, 0, false
	}
	return start, end, true
}

func dfaByRef(rt *runtime.Schema, ref runtime.ModelRef) (*runtime.DFAModel, error) {
	if rt == nil || ref.ID == 0 || int(ref.ID) >= len(rt.Models.DFA) {
		return nil, fmt.Errorf("dfa model %d out of range", ref.ID)
	}
	return &rt.Models.DFA[ref.ID], nil
}

func nfaByRef(rt *runtime.Schema, ref runtime.ModelRef) (*runtime.NFAModel, error) {
	if rt == nil || ref.ID == 0 || int(ref.ID) >= len(rt.Models.NFA) {
		return nil, fmt.Errorf("nfa model %d out of range", ref.ID)
	}
	return &rt.Models.NFA[ref.ID], nil
}

func allByRef(rt *runtime.Schema, ref runtime.ModelRef) (*runtime.AllModel, error) {
	if rt == nil || ref.ID == 0 || int(ref.ID) >= len(rt.Models.All) {
		return nil, fmt.Errorf("all model %d out of range", ref.ID)
	}
	return &rt.Models.All[ref.ID], nil
}

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

func allMemberAllowsSubst(rt *runtime.Schema, member runtime.AllMember, elem runtime.ElemID) bool {
	if rt == nil || member.SubstLen == 0 {
		return false
	}
	start := int(member.SubstOff)
	end := start + int(member.SubstLen)
	if start < 0 || end < 0 || end > len(rt.Models.AllSubst) {
		return false
	}
	return slices.Contains(rt.Models.AllSubst[start:end], elem)
}

// LookupGlobalElement resolves one global element by its symbol.
func LookupGlobalElement(rt *runtime.Schema, sym runtime.SymbolID) (runtime.ElemID, bool) {
	if rt == nil || sym == 0 || int(sym) >= len(rt.GlobalElements) {
		return 0, false
	}
	id := rt.GlobalElements[sym]
	return id, id != 0
}

func element(rt *runtime.Schema, id runtime.ElemID) (runtime.Element, bool) {
	if rt == nil || id == 0 || int(id) >= len(rt.Elements) {
		return runtime.Element{}, false
	}
	return rt.Elements[id], true
}
