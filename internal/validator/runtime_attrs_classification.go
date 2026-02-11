package validator

import (
	"bytes"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

type attrClass uint8

const (
	attrClassOther attrClass = iota
	attrClassXsiKnown
	attrClassXsiUnknown
	attrClassXML
)

type xsiAttrRole uint8

const (
	xsiAttrNone xsiAttrRole = iota
	xsiAttrType
	xsiAttrNil
	xsiAttrSchemaLocation
	xsiAttrNoNamespaceSchemaLocation
)

type attrClassification struct {
	classes      []attrClass
	xsiType      []byte
	xsiNil       []byte
	duplicateErr error
}

func (s *Session) classifyAttrs(attrs []StartAttr, checkDuplicates bool) (attrClassification, error) {
	if s == nil || s.rt == nil || len(attrs) == 0 {
		return attrClassification{}, nil
	}

	classes := s.attrClassBuf
	if cap(classes) < len(attrs) {
		classes = make([]attrClass, len(attrs))
	} else {
		classes = classes[:len(attrs)]
	}
	s.attrClassBuf = classes

	out := attrClassification{classes: classes}

	var table []attrSeenEntry
	mask := uint64(0)
	useHashTable := checkDuplicates && len(attrs) > smallAttrDupThreshold
	if useHashTable {
		size := runtime.NextPow2(len(attrs) * 2)
		table = s.attrSeenTable
		if cap(table) < size {
			table = make([]attrSeenEntry, size)
		} else {
			table = table[:size]
			clear(table)
		}
		mask = uint64(size - 1)
	}

	for i := range attrs {
		if checkDuplicates {
			if useHashTable {
				nsBytes := attrNSBytes(s.rt, &attrs[i])
				hash := attrNameHash(nsBytes, attrs[i].Local)
				slot := int(hash & mask)
				for {
					entry := table[slot]
					if entry.hash == 0 {
						table[slot] = attrSeenEntry{hash: hash, idx: uint32(i)}
						break
					}
					if entry.hash == hash && s.attrNamesEqual(&attrs[int(entry.idx)], &attrs[i]) {
						if out.duplicateErr == nil {
							out.duplicateErr = newValidationError(xsderrors.ErrXMLParse, "duplicate attribute")
						}
						break
					}
					slot = (slot + 1) & int(mask)
				}
			} else {
				for j := 0; j < i; j++ {
					if s.attrNamesEqual(&attrs[j], &attrs[i]) {
						if out.duplicateErr == nil {
							out.duplicateErr = newValidationError(xsderrors.ErrXMLParse, "duplicate attribute")
						}
						break
					}
				}
			}
		}

		class, role := s.classifyAttr(&attrs[i])
		classes[i] = class
		switch role {
		case xsiAttrType:
			if len(out.xsiType) > 0 {
				return attrClassification{}, newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:type attribute")
			}
			out.xsiType = attrs[i].Value
		case xsiAttrNil:
			if len(out.xsiNil) > 0 {
				return attrClassification{}, newValidationError(xsderrors.ErrDatatypeInvalid, "duplicate xsi:nil attribute")
			}
			out.xsiNil = attrs[i].Value
		}
	}

	if useHashTable {
		s.attrSeenTable = table
	}
	return out, nil
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

