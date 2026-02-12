package validator

import xsderrors "github.com/jacoelho/xsd/errors"

func (s *Session) classifyAndCaptureAttr(out *attrClassification, attr *StartAttr, idx int) error {
	if out == nil || attr == nil || idx < 0 || idx >= len(out.classes) {
		return nil
	}

	class, role := s.classifyAttr(attr)
	out.classes[idx] = class
	switch role {
	case xsiAttrType:
		if len(out.xsiType) > 0 {
			return newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:type attribute")
		}
		out.xsiType = attr.Value
	case xsiAttrNil:
		if len(out.xsiNil) > 0 {
			return newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:nil attribute")
		}
		out.xsiNil = attr.Value
	}
	return nil
}

func (s *Session) classifyAttr(attr *StartAttr) (attrClass, xsiAttrRole) {
	role := s.xsiAttrRole(attr)
	if role != xsiAttrNone {
		return attrClassXsiKnown, role
	}
	if s.isXsiNamespaceAttr(attr) {
		return attrClassXsiUnknown, xsiAttrNone
	}
	if s.isXMLAttribute(attr) {
		return attrClassXML, xsiAttrNone
	}
	return attrClassOther, xsiAttrNone
}
