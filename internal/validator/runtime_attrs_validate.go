package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) ValidateAttributes(typeID runtime.TypeID, attrs []StartAttr, resolver value.NSResolver) (AttrResult, error) {
	if s == nil || s.rt == nil {
		return AttrResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	typ, ok := s.typeByID(typeID)
	if !ok {
		return AttrResult{}, fmt.Errorf("type %d not found", typeID)
	}
	storeAttrs := s.hasIdentityConstraints()

	if typ.Kind != runtime.TypeComplex {
		return s.validateSimpleTypeAttrs(attrs, storeAttrs)
	}

	ct := &s.rt.ComplexTypes[typ.Complex.ID]
	uses := s.attrUses(ct.Attrs)
	present := s.prepareAttrPresent(len(uses))

	if err := s.checkDuplicateAttrs(attrs); err != nil {
		return AttrResult{}, err
	}

	validated, seenID, err := s.validateComplexAttrs(ct, present, attrs, resolver, storeAttrs)
	if err != nil {
		return AttrResult{}, err
	}

	applied, err := s.applyDefaultAttrs(uses, present, storeAttrs, seenID)
	if err != nil {
		return AttrResult{}, err
	}

	result := AttrResult{Attrs: validated, Applied: applied}
	if storeAttrs {
		s.attrValidatedBuf = validated[:0]
	}
	s.attrAppliedBuf = applied[:0]
	return result, nil
}

func (s *Session) validateSimpleTypeAttrs(attrs []StartAttr, storeAttrs bool) (AttrResult, error) {
	for _, attr := range attrs {
		if s.isUnknownXsiAttribute(&attr) {
			return AttrResult{}, newValidationError(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "unknown xsi attribute")
		}
		if !s.isXsiAttribute(&attr) && !s.isXMLAttribute(&attr) {
			return AttrResult{}, newValidationError(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "attribute not allowed on simple type")
		}
	}
	if !storeAttrs {
		return AttrResult{}, nil
	}
	result := AttrResult{Attrs: make([]StartAttr, 0, len(attrs))}
	for _, attr := range attrs {
		s.ensureAttrNameStable(&attr)
		attr.Value = s.storeValue(attr.Value)
		attr.KeyKind = runtime.VKInvalid
		attr.KeyBytes = nil
		result.Attrs = append(result.Attrs, attr)
	}
	return result, nil
}

func (s *Session) prepareAttrPresent(size int) []bool {
	present := s.attrPresent
	if cap(present) < size {
		present = make([]bool, size)
	} else {
		present = present[:size]
		for i := range present {
			present[i] = false
		}
	}
	s.attrPresent = present
	return present
}
