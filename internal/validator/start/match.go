package start

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
	"github.com/jacoelho/xsd/internal/validator/wildcard"
)

// ResolveMatch resolves one start-element match result to a concrete element ID.
func ResolveMatch(rt *runtime.Schema, match model.Match, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte) (runtime.ElemID, error) {
	if rt == nil {
		return 0, diag.New(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}

	switch match.Kind {
	case model.MatchNone:
		return 0, diag.New(xsderrors.ErrUnexpectedElement, "no content model match")
	case model.MatchElem:
		if match.Elem == 0 {
			return 0, diag.New(xsderrors.ErrElementNotDeclared, "element not declared")
		}
		return match.Elem, nil
	case model.MatchWildcard:
		if match.Wildcard == 0 {
			return 0, diag.New(xsderrors.ErrWildcardNotDeclared, "wildcard match invalid")
		}
		if !rt.WildcardAccepts(match.Wildcard, nsBytes, nsID) {
			return 0, diag.New(xsderrors.ErrUnexpectedElement, "wildcard rejected namespace")
		}

		rule := rt.Wildcards[match.Wildcard]
		var wildcardElem runtime.ElemID
		resolved, err := wildcard.ResolveSymbol(
			rule.PC,
			sym,
			func(symbol runtime.SymbolID) bool {
				elem, ok := model.LookupGlobalElement(rt, symbol)
				if !ok {
					return false
				}
				wildcardElem = elem
				return true
			},
			func() error {
				return diag.New(xsderrors.ErrValidateWildcardElemStrictUnresolved, "wildcard strict unresolved element")
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
