package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

// ResolveStartResult resolves one start-element match and xsi overrides to a runtime result.
func ResolveStartResult(
	rt *runtime.Schema,
	match StartMatch,
	sym runtime.SymbolID,
	nsID runtime.NamespaceID,
	nsBytes []byte,
	classified Classification,
	resolver value.NSResolver,
) (StartResult, error) {
	decl, err := ResolveStartMatch(rt, match, sym, nsID, nsBytes)
	if err != nil {
		return StartResult{}, err
	}
	if decl == 0 {
		return StartResult{Skip: true}, nil
	}
	if int(decl) >= len(rt.Elements) {
		return StartResult{}, fmt.Errorf("element %d out of range", decl)
	}

	elem := rt.Elements[decl]
	if elem.Flags&runtime.ElemAbstract != 0 {
		return StartResult{}, xsderrors.New(xsderrors.ErrElementAbstract, "element is abstract")
	}

	actualType := elem.Type
	if len(classified.XSIType) > 0 {
		resolved, err := ResolveStartXSIType(rt, classified.XSIType, resolver)
		if err != nil {
			return StartResult{}, xsderrors.New(xsderrors.ErrValidateXsiTypeUnresolved, err.Error())
		}
		if err := CheckStartTypeDerivation(rt, resolved, actualType, elem.Block); err != nil {
			return StartResult{}, xsderrors.New(xsderrors.ErrValidateXsiTypeDerivationBlocked, err.Error())
		}
		actualType = resolved
	}

	nilled := false
	if len(classified.XSINil) > 0 {
		flag, err := value.ParseBoolean(classified.XSINil)
		if err != nil {
			return StartResult{}, xsderrors.New(xsderrors.ErrDatatypeInvalid, fmt.Sprintf("invalid xsi:nil: %v", err))
		}
		if flag {
			if elem.Flags&runtime.ElemNillable == 0 {
				return StartResult{}, xsderrors.New(xsderrors.ErrValidateXsiNilNotNillable, "element is not nillable")
			}
			if elem.Fixed.Present {
				return StartResult{}, xsderrors.New(xsderrors.ErrValidateNilledHasFixed, "element has fixed value and cannot be nilled")
			}
			nilled = true
		}
	}

	if int(actualType) < len(rt.Types) && rt.Types[actualType].Flags&runtime.TypeAbstract != 0 {
		return StartResult{}, xsderrors.New(xsderrors.ErrElementTypeAbstract, "type is abstract")
	}

	return StartResult{Elem: decl, Type: actualType, Nilled: nilled}, nil
}
