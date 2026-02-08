package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) checkUnionPatterns(facets []runtime.FacetInstr, normalized []byte) (bool, error) {
	if len(facets) == 0 {
		return false, nil
	}
	seen := false
	for _, instr := range facets {
		if instr.Op != runtime.FPattern {
			continue
		}
		seen = true
		if int(instr.Arg0) >= len(s.rt.Patterns) {
			return seen, valueErrorf(valueErrInvalid, "pattern %d out of range", instr.Arg0)
		}
		pat := s.rt.Patterns[instr.Arg0]
		if pat.Re != nil && !pat.Re.Match(normalized) {
			return seen, valueErrorf(valueErrFacet, "pattern violation")
		}
	}
	return seen, nil
}

func (s *Session) enumSetsContainAll(enumIDs []runtime.EnumID, keyKind runtime.ValueKind, keyBytes []byte) bool {
	for _, enumID := range enumIDs {
		if !runtime.EnumContains(&s.rt.Enums, enumID, keyKind, keyBytes) {
			return false
		}
	}
	return true
}
