package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
)

const smallAttrDupThreshold = 8

func (s *Session) attrNamesEqual(a, b *StartAttr) bool {
	if a.Sym != 0 && b.Sym != 0 {
		return a.Sym == b.Sym
	}
	return bytes.Equal(attrNSBytes(s.rt, a), attrNSBytes(s.rt, b)) && bytes.Equal(a.Local, b.Local)
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
