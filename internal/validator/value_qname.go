package validator

import (
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeQName(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	canonicalBuf := []byte(nil)
	if s != nil {
		canonicalBuf = s.valueScratch[:0]
	}
	canon, err := value.CanonicalQName(normalized, resolver, canonicalBuf)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if s != nil {
		s.valueScratch = canon
	}

	if meta.Kind == runtime.VNotation && !s.notationDeclared(canon) {
		return nil, xsderrors.Invalid("notation not declared")
	}

	if needKey {
		tag := byte(0)
		if meta.Kind == runtime.VNotation {
			tag = 1
		}
		keyBuf := []byte(nil)
		if s != nil {
			keyBuf = s.keyTmp[:0]
		}
		key := runtime.QNameKeyCanonical(keyBuf, tag, canon)
		if len(key) == 0 {
			return nil, xsderrors.Invalid("invalid QName key")
		}
		if s != nil {
			s.keyTmp = key
			s.setKey(metrics, runtime.VKQName, key, false)
		}
	}
	return canon, nil
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
