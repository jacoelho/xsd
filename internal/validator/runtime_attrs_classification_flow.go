package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
)

func (s *Session) classifyAttrs(attrs []StartAttr, checkDuplicates bool) (attrClassification, error) {
	if s == nil || s.rt == nil || len(attrs) == 0 {
		return attrClassification{}, nil
	}

	out := attrClassification{classes: s.prepareAttrClasses(len(attrs))}
	dup := s.prepareAttrDupState(len(attrs), checkDuplicates)

	for i := range attrs {
		if checkDuplicates && s.hasDuplicateAttrAt(attrs, i, &dup) && out.duplicateErr == nil {
			out.duplicateErr = newValidationError(xsderrors.ErrXMLParse, "duplicate attribute")
		}

		if err := s.classifyAndCaptureAttr(&out, &attrs[i], i); err != nil {
			return attrClassification{}, err
		}
	}

	s.finalizeAttrDupState(dup)
	return out, nil
}
