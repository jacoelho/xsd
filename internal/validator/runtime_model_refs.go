package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) allMemberAllowsSubst(member runtime.AllMember, elem runtime.ElemID) bool {
	if s == nil || s.rt == nil || member.SubstLen == 0 {
		return false
	}
	start := int(member.SubstOff)
	end := start + int(member.SubstLen)
	if start < 0 || end < 0 || end > len(s.rt.Models.AllSubst) {
		return false
	}
	return slices.Contains(s.rt.Models.AllSubst[start:end], elem)
}

func (s *Session) dfaByRef(ref runtime.ModelRef) (*runtime.DFAModel, error) {
	if ref.ID == 0 || int(ref.ID) >= len(s.rt.Models.DFA) {
		return nil, fmt.Errorf("dfa model %d out of range", ref.ID)
	}
	return &s.rt.Models.DFA[ref.ID], nil
}

func (s *Session) nfaByRef(ref runtime.ModelRef) (*runtime.NFAModel, error) {
	if ref.ID == 0 || int(ref.ID) >= len(s.rt.Models.NFA) {
		return nil, fmt.Errorf("nfa model %d out of range", ref.ID)
	}
	return &s.rt.Models.NFA[ref.ID], nil
}

func (s *Session) allByRef(ref runtime.ModelRef) (*runtime.AllModel, error) {
	if ref.ID == 0 || int(ref.ID) >= len(s.rt.Models.All) {
		return nil, fmt.Errorf("all model %d out of range", ref.ID)
	}
	return &s.rt.Models.All[ref.ID], nil
}
