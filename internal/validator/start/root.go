package start

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
)

// RootDecision describes the selected root-element start match or skip outcome.
type RootDecision struct {
	Match model.Match
	Skip  bool
}

// ResolveRoot applies the runtime root policy for one incoming root element.
func ResolveRoot(rt *runtime.Schema, sym runtime.SymbolID, nsID runtime.NamespaceID) (RootDecision, error) {
	if rt == nil {
		return RootDecision{}, diag.New(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}

	actual := model.ActualElementName(rt, sym, nsID)
	expected := model.ExpectedGlobalElements(rt)

	switch rt.RootPolicy {
	case runtime.RootAny:
		if sym == 0 {
			return RootDecision{Skip: true}, nil
		}
		elemID, ok := model.LookupGlobalElement(rt, sym)
		if !ok {
			return RootDecision{Skip: true}, nil
		}
		return RootDecision{Match: model.Match{Kind: model.MatchElem, Elem: elemID}}, nil
	case runtime.RootStrict:
		if sym == 0 {
			return RootDecision{}, diag.NewWithDetails(
				xsderrors.ErrValidateRootNotDeclared,
				"root element not declared",
				actual,
				expected,
			)
		}
		elemID, ok := model.LookupGlobalElement(rt, sym)
		if !ok {
			return RootDecision{}, diag.NewWithDetails(
				xsderrors.ErrValidateRootNotDeclared,
				"root element not declared",
				actual,
				expected,
			)
		}
		return RootDecision{Match: model.Match{Kind: model.MatchElem, Elem: elemID}}, nil
	default:
		return RootDecision{}, diag.NewWithDetails(
			xsderrors.ErrValidateRootNotDeclared,
			"root element not declared",
			actual,
			expected,
		)
	}
}
