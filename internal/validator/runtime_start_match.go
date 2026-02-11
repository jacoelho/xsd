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
		var wildcardElem runtime.ElemID
		resolved, err := resolveWildcardSymbol(rule.PC, sym, func(symbol runtime.SymbolID) bool {
			elem, ok := s.globalElementBySymbol(symbol)
			if !ok {
				return false
			}
			wildcardElem = elem
			return true
		}, func() error {
			return newValidationError(xsderrors.ErrValidateWildcardElemStrictUnresolved, "wildcard strict unresolved element")
		})
		if err != nil {
			return 0, err
		}
		if !resolved {
			return 0, nil
		}
		return wildcardElem, nil
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
