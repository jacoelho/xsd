package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrs(ct *runtime.ComplexType, present []bool, attrs []StartAttr, resolver value.NSResolver, storeAttrs bool) ([]StartAttr, bool, error) {
	classified, err := s.classifyAttrs(attrs, false)
	if err != nil {
		return nil, false, err
	}
	return s.validateComplexAttrsClassified(ct, present, attrs, classified.classes, resolver, storeAttrs)
}

func (s *Session) validateComplexAttrsClassified(ct *runtime.ComplexType, present []bool, attrs []StartAttr, classes []attrClass, resolver value.NSResolver, storeAttrs bool) ([]StartAttr, bool, error) {
	var validated []StartAttr
	if storeAttrs {
		validated = s.attrValidatedBuf[:0]
		if cap(validated) < len(attrs) {
			validated = make([]StartAttr, 0, len(attrs))
		}
	}
	seenID := false

	for i, attr := range attrs {
		var class attrClass
		if i < len(classes) {
			class = classes[i]
		} else {
			class, _ = s.classifyAttr(&attr)
		}

		if class == attrClassXsiUnknown {
			return nil, seenID, newValidationError(xsderrors.ErrAttributeNotDeclared, "unknown xsi attribute")
		}
		if class == attrClassXsiKnown {
			validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
			continue
		}

		var (
			handled bool
			err     error
		)
		validated, handled, err = s.tryValidateComplexDeclaredAttr(ct, present, validated, attr, resolver, storeAttrs, &seenID)
		if err != nil {
			return nil, seenID, err
		}
		if handled {
			continue
		}

		if class == attrClassXML {
			validated = s.appendRawValidatedAttr(validated, attr, storeAttrs)
			continue
		}

		validated, err = s.validateComplexWildcardAttr(ct, validated, attr, resolver, storeAttrs, &seenID)
		if err != nil {
			return nil, seenID, err
		}
	}

	return validated, seenID, nil
}
