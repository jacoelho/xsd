package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// StartRootDecision describes the selected root-element start match or skip outcome.
type StartRootDecision struct {
	Match StartMatch
	Skip  bool
}

// ResolveStartRoot applies the runtime root policy for one incoming root element.
func ResolveStartRoot(rt *runtime.Schema, sym runtime.SymbolID, nsID runtime.NamespaceID) (StartRootDecision, error) {
	if rt == nil {
		return StartRootDecision{}, xsderrors.New(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}

	actual := StartActualElementName(rt, sym, nsID)
	expected := ExpectedStartGlobalElements(rt)

	switch rt.RootPolicy {
	case runtime.RootAny:
		if sym == 0 {
			return StartRootDecision{Skip: true}, nil
		}
		elemID, ok := LookupStartGlobalElement(rt, sym)
		if !ok {
			return StartRootDecision{Skip: true}, nil
		}
		return StartRootDecision{Match: StartMatch{Kind: StartMatchElem, Elem: elemID}}, nil
	case runtime.RootStrict:
		if sym == 0 {
			return StartRootDecision{}, xsderrors.NewWithDetails(
				xsderrors.ErrValidateRootNotDeclared,
				"root element not declared",
				actual,
				expected,
			)
		}
		elemID, ok := LookupStartGlobalElement(rt, sym)
		if !ok {
			return StartRootDecision{}, xsderrors.NewWithDetails(
				xsderrors.ErrValidateRootNotDeclared,
				"root element not declared",
				actual,
				expected,
			)
		}
		return StartRootDecision{Match: StartMatch{Kind: StartMatchElem, Elem: elemID}}, nil
	default:
		return StartRootDecision{}, xsderrors.NewWithDetails(
			xsderrors.ErrValidateRootNotDeclared,
			"root element not declared",
			actual,
			expected,
		)
	}
}
