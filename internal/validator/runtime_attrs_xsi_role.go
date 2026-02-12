package validator

import (
	"bytes"
)

func (s *Session) xsiAttrRole(attr *StartAttr) xsiAttrRole {
	if s == nil || s.rt == nil || attr == nil {
		return xsiAttrNone
	}
	predef := s.rt.Predef
	switch attr.Sym {
	case predef.XsiType:
		return xsiAttrType
	case predef.XsiNil:
		return xsiAttrNil
	case predef.XsiSchemaLocation:
		return xsiAttrSchemaLocation
	case predef.XsiNoNamespaceSchemaLocation:
		return xsiAttrNoNamespaceSchemaLocation
	}
	if !s.isXsiNamespaceAttr(attr) {
		return xsiAttrNone
	}
	switch {
	case bytes.Equal(attr.Local, xsiLocalType):
		return xsiAttrType
	case bytes.Equal(attr.Local, xsiLocalNil):
		return xsiAttrNil
	case bytes.Equal(attr.Local, xsiLocalSchemaLocation):
		return xsiAttrSchemaLocation
	case bytes.Equal(attr.Local, xsiLocalNoNamespaceSchemaLocation):
		return xsiAttrNoNamespaceSchemaLocation
	default:
		return xsiAttrNone
	}
}
