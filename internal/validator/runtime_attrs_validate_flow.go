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
	typ, ok := s.typeByID(typeID)
	if !ok {
		return AttrResult{}, fmt.Errorf("type %d not found", typeID)
	}
	storeAttrs := s.hasIdentityConstraints()
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
				return ValidateSimple(
					s.rt,
					input,
					classes,
					store,
					validated,
					func(validated []Start, attr Start, storeAttrs bool) []Start {
						return StoreRaw(validated, attr, storeAttrs, s.ensureAttrNameStable, s.storeValue)
					},
				)
			},
			ValidateComplex: func(ct *runtime.ComplexType, present []bool, input []Start, classes []Class, store bool, validated []Start) ([]Start, bool, error) {
				return s.validateComplexAttrsClassified(ct, present, input, classes, resolver, store, validated)
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
	s.attrAppliedBuf = result.Applied[:0]
	return AttrResult{Attrs: result.Attrs, Applied: result.Applied}, nil
}
