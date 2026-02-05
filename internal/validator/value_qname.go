package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) canonicalizeQName(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, needKey bool, metrics *valueMetrics) ([]byte, error) {
	canon, err := value.CanonicalQName(normalized, resolver, nil)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if meta.Kind == runtime.VNotation && !s.notationDeclared(canon) {
		return nil, valueErrorMsg(valueErrInvalid, "notation not declared")
	}
	canonStored := canon
	if needKey {
		tag := byte(0)
		if meta.Kind == runtime.VNotation {
			tag = 1
		}
		key := valuekey.QNameKeyCanonical(s.keyTmp[:0], tag, canonStored)
		if len(key) == 0 {
			return nil, valueErrorf(valueErrInvalid, "invalid QName key")
		}
		s.keyTmp = key
		s.setKey(metrics, runtime.VKQName, key, false)
	}
	return canonStored, nil
}

func (s *Session) notationDeclared(canon []byte) bool {
	if s == nil || s.rt == nil || len(s.rt.Notations) == 0 {
		return false
	}
	ns, local, err := splitCanonicalQName(canon)
	if err != nil {
		return false
	}
	nsID := s.namespaceID(ns)
	if nsID == 0 {
		return false
	}
	sym := s.rt.Symbols.Lookup(nsID, local)
	if sym == 0 {
		return false
	}
	_, ok := slices.BinarySearch(s.rt.Notations, sym)
	return ok
}
