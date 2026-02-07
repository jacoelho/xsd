package validator

import (
	"bytes"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) attrNamesEqual(a, b *StartAttr) bool {
	if a.Sym != 0 && b.Sym != 0 {
		return a.Sym == b.Sym
	}
	return bytes.Equal(attrNSBytes(s.rt, a), attrNSBytes(s.rt, b)) && bytes.Equal(a.Local, b.Local)
}

func (s *Session) checkDuplicateAttrs(attrs []StartAttr) error {
	if s == nil || s.rt == nil || len(attrs) < 2 {
		return nil
	}
	// smallAttrDupThreshold switches from quadratic scan to hashing.
	const smallAttrDupThreshold = 8
	if len(attrs) <= smallAttrDupThreshold {
		for i := range attrs {
			for j := i + 1; j < len(attrs); j++ {
				if s.attrNamesEqual(&attrs[i], &attrs[j]) {
					return newValidationError(xsderrors.ErrXMLParse, "duplicate attribute")
				}
			}
		}
		return nil
	}
	size := runtime.NextPow2(len(attrs) * 2)
	table := s.attrSeenTable
	if cap(table) < size {
		table = make([]attrSeenEntry, size)
	} else {
		table = table[:size]
		// table is cleared on each use; reuse is safe across calls.
		clear(table)
	}
	mask := uint64(size - 1)

	for i := range attrs {
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
				s.attrSeenTable = table
				return newValidationError(xsderrors.ErrXMLParse, "duplicate attribute")
			}
			slot = (slot + 1) & int(mask)
		}
	}
	s.attrSeenTable = table
	return nil
}

func attrNSBytes(rt *runtime.Schema, attr *StartAttr) []byte {
	if rt != nil && attr.NS != 0 {
		return rt.Namespaces.Bytes(attr.NS)
	}
	return attr.NSBytes
}

func (s *Session) ensureAttrNameStable(attr *StartAttr) {
	if s == nil || attr == nil || attr.NameCached {
		return
	}
	if len(attr.Local) > 0 {
		off := len(s.nameLocal)
		s.nameLocal = append(s.nameLocal, attr.Local...)
		attr.Local = s.nameLocal[off : off+len(attr.Local)]
	}
	if len(attr.NSBytes) > 0 {
		off := len(s.nameNS)
		s.nameNS = append(s.nameNS, attr.NSBytes...)
		attr.NSBytes = s.nameNS[off : off+len(attr.NSBytes)]
	}
	attr.NameCached = true
}

func (s *Session) isXsiAttribute(attr *StartAttr) bool {
	if s == nil || s.rt == nil || attr == nil {
		return false
	}
	predef := s.rt.Predef
	switch attr.Sym {
	case predef.XsiType, predef.XsiNil, predef.XsiSchemaLocation, predef.XsiNoNamespaceSchemaLocation:
		return true
	}
	if !s.isXsiNamespaceAttr(attr) {
		return false
	}
	return isXsiLocalName(attr.Local)
}

func (s *Session) isUnknownXsiAttribute(attr *StartAttr) bool {
	if s == nil || s.rt == nil || attr == nil {
		return false
	}
	return s.isXsiNamespaceAttr(attr) && !s.isXsiAttribute(attr)
}

func isXsiLocalName(local []byte) bool {
	switch {
	case bytes.Equal(local, xsiLocalType):
		return true
	case bytes.Equal(local, xsiLocalNil):
		return true
	case bytes.Equal(local, xsiLocalSchemaLocation):
		return true
	case bytes.Equal(local, xsiLocalNoNamespaceSchemaLocation):
		return true
	default:
		return false
	}
}

func (s *Session) isXsiNamespaceAttr(attr *StartAttr) bool {
	if attr.NS != 0 {
		return attr.NS == s.rt.PredefNS.Xsi
	}
	target := s.rt.Namespaces.Bytes(s.rt.PredefNS.Xsi)
	if len(target) == 0 {
		return false
	}
	return bytes.Equal(target, attr.NSBytes)
}

func (s *Session) isXMLAttribute(attr *StartAttr) bool {
	if s == nil || s.rt == nil || attr == nil {
		return false
	}
	if attr.NS != 0 {
		return attr.NS == s.rt.PredefNS.Xml
	}
	target := s.rt.Namespaces.Bytes(s.rt.PredefNS.Xml)
	if len(target) == 0 {
		return false
	}
	return bytes.Equal(target, attr.NSBytes)
}

func (s *Session) isIDValidator(id runtime.ValidatorID) bool {
	if s == nil || s.rt == nil || id == 0 {
		return false
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return false
	}
	meta := s.rt.Validators.Meta[id]
	if meta.Kind != runtime.VString {
		return false
	}
	kind, ok := s.stringKind(meta)
	if !ok {
		return false
	}
	return kind == runtime.StringID
}
