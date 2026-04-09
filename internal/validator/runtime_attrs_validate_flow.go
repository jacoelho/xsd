package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// ValidateAttributes validates attributes against the resolved runtime type.
func (s *Session) ValidateAttributes(typeID runtime.TypeID, inputAttrs []Start, resolver value.NSResolver) (AttrResult, error) {
	if s == nil || s.rt == nil {
		return AttrResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	classified, err := s.classifyAttrs(inputAttrs, true)
	if err != nil {
		return AttrResult{}, err
	}
	return s.validateAttributesClassified(typeID, inputAttrs, resolver, classified)
}

func (s *Session) validateAttributesClassified(typeID runtime.TypeID, inputAttrs []Start, resolver value.NSResolver, classified Classification) (AttrResult, error) {
	return s.validateAttributesClassifiedWithStorage(typeID, inputAttrs, resolver, classified, s.hasIdentityConstraints(), true)
}

func (s *Session) validateAttributesClassifiedWithStorage(typeID runtime.TypeID, inputAttrs []Start, resolver value.NSResolver, classified Classification, storeAttrs, storeValues bool) (AttrResult, error) {
	typ, ok := s.typeByID(typeID)
	if !ok {
		return AttrResult{}, fmt.Errorf("type %d not found", typeID)
	}
	result, err := ValidateType(
		s.rt,
		typ,
		classified,
		inputAttrs,
		storeAttrs,
		TypeCallbacks{
			PrepareValidated: s.attrState.PrepareValidated,
			PreparePresent:   s.attrState.PreparePresent,
			ValidateSimple: func(input []Start, classes []Class, store bool, validated []Start) ([]Start, error) {
				storeRaw := StoreRaw
				if !storeValues {
					storeRaw = func(validated []Start, attr Start, storeAttrs bool, _ func(*Start), _ func([]byte) []byte) []Start {
						return StoreRawIdentity(validated, attr, storeAttrs, s.ensureAttrNameStable)
					}
				}
				return ValidateSimple(
					s.rt,
					input,
					classes,
					store,
					validated,
					func(validated []Start, attr Start, storeAttrs bool) []Start {
						return storeRaw(validated, attr, storeAttrs, s.ensureAttrNameStable, s.storeValue)
					},
				)
			},
			ValidateComplex: func(ct *runtime.ComplexType, present []bool, input []Start, classes []Class, store bool, validated []Start) ([]Start, bool, error) {
				return s.validateComplexAttrsClassified(ct, present, input, classes, resolver, store, storeValues, validated)
			},
			ApplyDefaults: s.applyDefaultAttrs,
		},
	)
	if err != nil {
		return AttrResult{}, err
	}

	if storeAttrs {
		s.attrState.Validated = result.Attrs[:0]
	}
	if cap(result.Applied) > 0 {
		s.attrAppliedBuf = result.Applied[:0]
	}
	return AttrResult{Attrs: result.Attrs, Applied: result.Applied}, nil
}

func (s *Session) needsIdentityAttrs(elemID runtime.ElemID) bool {
	if s == nil || s.rt == nil {
		return false
	}
	if s.icState.Scopes.Len() > 0 {
		return true
	}
	elem, ok := s.element(elemID)
	return ok && elem.ICLen > 0
}
