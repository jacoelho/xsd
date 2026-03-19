package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
)

func (s *Session) appendRawValidatedAttr(validated []attrs.Start, attr attrs.Start, storeAttrs bool) []attrs.Start {
	return attrs.StoreRaw(validated, attr, storeAttrs, s.ensureAttrNameStable, s.storeValue)
}

func (s *Session) appendValidatedAttr(validated []attrs.Start, attr attrs.Start, storeAttrs bool, canonical []byte, keyKind runtime.ValueKind, keyBytes []byte) []attrs.Start {
	return attrs.StoreCanonical(validated, attr, storeAttrs, s.ensureAttrNameStable, canonical, keyKind, keyBytes)
}
