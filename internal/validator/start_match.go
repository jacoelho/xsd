package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

// ResolveStartMatch resolves one start-element match result to a concrete element ID.
func ResolveStartMatch(rt *runtime.Schema, match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (runtime.ElemID, error) {
	if rt == nil {
		return 0, xsderrors.New(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}

	switch match.Kind {
	case StartMatchNone:
		return 0, xsderrors.New(xsderrors.ErrUnexpectedElement, "no content model match")
	case StartMatchElem:
		if match.Elem == 0 {
			return 0, xsderrors.New(xsderrors.ErrElementNotDeclared, "element not declared")
		}
		return match.Elem, nil
	case StartMatchWildcard:
		if match.Wildcard == 0 {
			return 0, xsderrors.New(xsderrors.ErrWildcardNotDeclared, "wildcard match invalid")
		}
		if !rt.WildcardAccepts(match.Wildcard, nsBytes, nsID) {
			return 0, xsderrors.New(xsderrors.ErrUnexpectedElement, "wildcard rejected namespace")
		}

		rule := rt.Wildcards[match.Wildcard]
		var wildcardElem runtime.ElemID
		resolved, err := ResolveStartSymbol(
			rule.PC,
			sym,
			func(symbol runtime.SymbolID) bool {
				elem, ok := LookupStartGlobalElement(rt, symbol)
				if !ok {
					return false
				}
				wildcardElem = elem
				return true
			},
			func() error {
				return xsderrors.New(xsderrors.ErrValidateWildcardElemStrictUnresolved, "wildcard strict unresolved element")
			},
		)
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
