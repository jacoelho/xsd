package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) expectedFromDFAState(model *runtime.DFAModel, state uint32) []string {
	if model == nil || int(state) >= len(model.States) {
		return nil
	}
	rec := model.States[state]
	trans, err := sliceDFATransitions(model, rec)
	if err != nil {
		return nil
	}
	wild, err := sliceDFAWildcards(model, rec)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(trans)+len(wild))
	for _, tr := range trans {
		name := s.elementName(tr.Elem)
		if name == "" {
			name = s.symbolName(tr.Sym)
		}
		names = append(names, name)
	}
	for range wild {
		names = append(names, "*")
	}
	return normalizeExpectedElements(names)
}
