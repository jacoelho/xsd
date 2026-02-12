package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) expectedAllMemberNames(member runtime.AllMember) []string {
	names := make([]string, 0, 4)
	if head := s.elementName(member.Elem); head != "" {
		names = append(names, head)
	}
	if s == nil || s.rt == nil || !member.AllowsSubst || member.SubstLen == 0 {
		return names
	}
	start := int(member.SubstOff)
	end := start + int(member.SubstLen)
	if start < 0 || end < 0 || end > len(s.rt.Models.AllSubst) {
		return names
	}
	for _, elem := range s.rt.Models.AllSubst[start:end] {
		names = append(names, s.elementName(elem))
	}
	return names
}

func (s *Session) expectedFromAllRemaining(model *runtime.AllModel, state []uint64, onlyRequired bool) []string {
	if model == nil {
		return nil
	}
	names := make([]string, 0, len(model.Members))
	for i, member := range model.Members {
		if allHas(state, i) {
			continue
		}
		if onlyRequired && member.Optional {
			continue
		}
		names = append(names, s.expectedAllMemberNames(member)...)
	}
	return normalizeExpectedElements(names)
}
