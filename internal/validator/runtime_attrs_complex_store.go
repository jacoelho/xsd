package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) appendRawValidatedAttr(validated []StartAttr, attr StartAttr, storeAttrs bool) []StartAttr {
	if !storeAttrs {
		return validated
	}
	s.ensureAttrNameStable(&attr)
	attr.Value = s.storeValue(attr.Value)
	attr.KeyKind = runtime.VKInvalid
	attr.KeyBytes = nil
	return append(validated, attr)
}

func (s *Session) appendValidatedAttr(validated []StartAttr, attr StartAttr, storeAttrs bool, canonical []byte, keyKind runtime.ValueKind, keyBytes []byte) []StartAttr {
	if !storeAttrs {
		return validated
	}
	s.ensureAttrNameStable(&attr)
	attr.Value = canonical
	attr.KeyKind = keyKind
	attr.KeyBytes = keyBytes
	return append(validated, attr)
}
