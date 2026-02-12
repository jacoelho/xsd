package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) expectedFromNFAMatchers(model *runtime.NFAModel, positions []uint64) []string {
	if model == nil || len(positions) == 0 {
		return nil
	}
	names := make([]string, 0, 4)
	forEachBit(positions, len(model.Matchers), func(pos int) {
		m := model.Matchers[pos]
		switch m.Kind {
		case runtime.PosExact:
			name := s.elementName(m.Elem)
			if name == "" {
				name = s.symbolName(m.Sym)
			}
			names = append(names, name)
		case runtime.PosWildcard:
			names = append(names, "*")
		}
	})
	return normalizeExpectedElements(names)
}

func (s *Session) expectedFromNFAStart(model *runtime.NFAModel) []string {
	if model == nil {
		return nil
	}
	start, ok := bitsetSlice(model.Bitsets, model.Start)
	if !ok {
		return nil
	}
	return s.expectedFromNFAMatchers(model, start)
}

func (s *Session) expectedFromNFAFollow(model *runtime.NFAModel, state []uint64) []string {
	if model == nil || len(state) == 0 || int(model.FollowLen) > len(model.Follow) {
		return nil
	}
	follow := make([]uint64, len(state))
	forEachBit(state, len(model.Follow), func(pos int) {
		ref := model.Follow[pos]
		set, ok := bitsetSlice(model.Bitsets, ref)
		if !ok {
			return
		}
		bitsetOr(follow, set)
	})
	return s.expectedFromNFAMatchers(model, follow)
}
