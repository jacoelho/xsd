package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) startElementClassified(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, resolver value.NSResolver, classified attrClassification) (StartResult, error) {
	decl, err := s.resolveMatch(match, sym, nsID, nsBytes)
	if err != nil {
		return StartResult{}, err
	}
	if decl == 0 {
		return StartResult{Skip: true}, nil
	}
	elem, ok := s.element(decl)
	if !ok {
		return StartResult{}, fmt.Errorf("element %d out of range", decl)
	}
	if elem.Flags&runtime.ElemAbstract != 0 {
		return StartResult{}, newValidationError(xsderrors.ErrElementAbstract, "element is abstract")
	}

	actualType := elem.Type
	xsiType := classified.xsiType
	xsiNil := classified.xsiNil
	if len(xsiType) > 0 {
		resolved, err := s.resolveXsiType(xsiType, resolver)
		if err != nil {
			return StartResult{}, newValidationError(xsderrors.ErrValidateXsiTypeUnresolved, err.Error())
		}
		if err := s.checkTypeDerivation(resolved, actualType, elem.Block); err != nil {
			return StartResult{}, newValidationError(xsderrors.ErrValidateXsiTypeDerivationBlocked, err.Error())
		}
		actualType = resolved
	}

	nilled := false
	if len(xsiNil) > 0 {
		flag, err := value.ParseBoolean(xsiNil)
		if err != nil {
			return StartResult{}, newValidationError(xsderrors.ErrDatatypeInvalid, fmt.Sprintf("invalid xsi:nil: %v", err))
		}
		if flag {
			if elem.Flags&runtime.ElemNillable == 0 {
				return StartResult{}, newValidationError(xsderrors.ErrValidateXsiNilNotNillable, "element is not nillable")
			}
			if elem.Fixed.Present {
				return StartResult{}, newValidationError(xsderrors.ErrValidateNilledHasFixed, "element has fixed value and cannot be nilled")
			}
			nilled = true
		}
	}

	if typ, ok := s.typeByID(actualType); ok {
		if typ.Flags&runtime.TypeAbstract != 0 {
			return StartResult{}, newValidationError(xsderrors.ErrElementTypeAbstract, "type is abstract")
		}
	}

	return StartResult{Elem: decl, Type: actualType, Nilled: nilled}, nil
}
