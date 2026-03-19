package start

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/model"
	"github.com/jacoelho/xsd/internal/value"
)

// ResolveResult resolves one start-element match and xsi overrides to a runtime result.
func ResolveResult(
	rt *runtime.Schema,
	match model.Match,
	sym runtime.SymbolID,
	nsID runtime.NamespaceID,
	nsBytes []byte,
	classified attrs.Classification,
	resolver value.NSResolver,
) (Result, error) {
	decl, err := ResolveMatch(rt, match, sym, nsID, nsBytes)
	if err != nil {
		return Result{}, err
	}
	if decl == 0 {
		return Result{Skip: true}, nil
	}
	if int(decl) >= len(rt.Elements) {
		return Result{}, fmt.Errorf("element %d out of range", decl)
	}

	elem := rt.Elements[decl]
	if elem.Flags&runtime.ElemAbstract != 0 {
		return Result{}, diag.New(xsderrors.ErrElementAbstract, "element is abstract")
	}

	actualType := elem.Type
	if len(classified.XSIType) > 0 {
		resolved, err := ResolveXSIType(rt, classified.XSIType, resolver)
		if err != nil {
			return Result{}, diag.New(xsderrors.ErrValidateXsiTypeUnresolved, err.Error())
		}
		if err := CheckTypeDerivation(rt, resolved, actualType, elem.Block); err != nil {
			return Result{}, diag.New(xsderrors.ErrValidateXsiTypeDerivationBlocked, err.Error())
		}
		actualType = resolved
	}

	nilled := false
	if len(classified.XSINil) > 0 {
		flag, err := value.ParseBoolean(classified.XSINil)
		if err != nil {
			return Result{}, diag.New(xsderrors.ErrDatatypeInvalid, fmt.Sprintf("invalid xsi:nil: %v", err))
		}
		if flag {
			if elem.Flags&runtime.ElemNillable == 0 {
				return Result{}, diag.New(xsderrors.ErrValidateXsiNilNotNillable, "element is not nillable")
			}
			if elem.Fixed.Present {
				return Result{}, diag.New(xsderrors.ErrValidateNilledHasFixed, "element has fixed value and cannot be nilled")
			}
			nilled = true
		}
	}

	if int(actualType) < len(rt.Types) && rt.Types[actualType].Flags&runtime.TypeAbstract != 0 {
		return Result{}, diag.New(xsderrors.ErrElementTypeAbstract, "type is abstract")
	}

	return Result{Elem: decl, Type: actualType, Nilled: nilled}, nil
}
