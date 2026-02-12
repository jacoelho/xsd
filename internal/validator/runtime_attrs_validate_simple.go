package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) validateSimpleTypeAttrs(attrs []StartAttr, storeAttrs bool) (AttrResult, error) {
	classified, err := s.classifyAttrs(attrs, false)
	if err != nil {
		return AttrResult{}, err
	}
	return s.validateSimpleTypeAttrsClassified(attrs, classified.classes, storeAttrs)
}

func (s *Session) validateSimpleTypeAttrsClassified(attrs []StartAttr, classes []attrClass, storeAttrs bool) (AttrResult, error) {
	for i, attr := range attrs {
		var class attrClass
		if i < len(classes) {
			class = classes[i]
		} else {
			class, _ = s.classifyAttr(&attr)
		}
		switch class {
		case attrClassXsiUnknown:
			return AttrResult{}, newValidationError(xsderrors.ErrValidateSimpleTypeAttrNotAllowed, "unknown xsi attribute")
		case attrClassXsiKnown, attrClassXML:
			continue
		default:
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
