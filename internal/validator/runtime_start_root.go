package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

type rootStartDecision struct {
	match StartMatch
	skip  bool
}

func (s *Session) resolveRootStartMatch(sym runtime.SymbolID, nsID runtime.NamespaceID) (rootStartDecision, error) {
	switch s.rt.RootPolicy {
	case runtime.RootAny:
		if sym == 0 {
			return rootStartDecision{skip: true}, nil
		}
		elemID, ok := s.globalElementBySymbol(sym)
		if !ok {
			return rootStartDecision{skip: true}, nil
		}
		return rootStartDecision{match: StartMatch{Kind: MatchElem, Elem: elemID}}, nil
	case runtime.RootStrict:
		if sym == 0 {
			return rootStartDecision{}, newValidationErrorWithDetails(
				xsderrors.ErrValidateRootNotDeclared,
				"root element not declared",
				s.actualElementName(sym, nsID),
				s.expectedGlobalElements(),
			)
		}
		elemID, ok := s.globalElementBySymbol(sym)
		if !ok {
			return rootStartDecision{}, newValidationErrorWithDetails(
				xsderrors.ErrValidateRootNotDeclared,
				"root element not declared",
				s.actualElementName(sym, nsID),
				s.expectedGlobalElements(),
			)
		}
		return rootStartDecision{match: StartMatch{Kind: MatchElem, Elem: elemID}}, nil
	default:
		return rootStartDecision{}, newValidationErrorWithDetails(
			xsderrors.ErrValidateRootNotDeclared,
			"root element not declared",
			s.actualElementName(sym, nsID),
			s.expectedGlobalElements(),
		)
	}
}
