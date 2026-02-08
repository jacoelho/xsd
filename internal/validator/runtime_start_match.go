package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) resolveMatch(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (runtime.ElemID, error) {
	switch match.Kind {
	case MatchNone:
		return 0, newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
	case MatchElem:
		if match.Elem == 0 {
			return 0, newValidationError(xsderrors.ErrElementNotDeclared, "element not declared")
		}
		return match.Elem, nil
	case MatchWildcard:
		if match.Wildcard == 0 {
			return 0, newValidationError(xsderrors.ErrWildcardNotDeclared, "wildcard match invalid")
		}
		if !s.rt.WildcardAccepts(match.Wildcard, nsBytes, nsID) {
			return 0, newValidationError(xsderrors.ErrUnexpectedElement, "wildcard rejected namespace")
		}
		rule := s.rt.Wildcards[match.Wildcard]
		switch rule.PC {
		case runtime.PCSkip:
			return 0, nil
		case runtime.PCLax, runtime.PCStrict:
			if sym == 0 {
				if rule.PC == runtime.PCStrict {
					return 0, newValidationError(xsderrors.ErrValidateWildcardElemStrictUnresolved, "wildcard strict unresolved element")
				}
				return 0, nil
			}
			elem, ok := s.globalElementBySymbol(sym)
			if !ok {
				if rule.PC == runtime.PCStrict {
					return 0, newValidationError(xsderrors.ErrValidateWildcardElemStrictUnresolved, "wildcard strict unresolved element")
				}
				return 0, nil
			}
			return elem, nil
		default:
			return 0, fmt.Errorf("unknown wildcard processContents %d", rule.PC)
		}
	default:
		return 0, fmt.Errorf("unknown match kind %d", match.Kind)
	}
}

func (s *Session) globalElementBySymbol(sym runtime.SymbolID) (runtime.ElemID, bool) {
	if sym == 0 {
		return 0, false
	}
	if s.rt == nil || int(sym) >= len(s.rt.GlobalElements) {
		return 0, false
	}
	id := s.rt.GlobalElements[sym]
	return id, id != 0
}
