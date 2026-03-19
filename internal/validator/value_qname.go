package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeQName(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, needKey bool, metrics *valruntime.State) ([]byte, error) {
	result, bufs, err := valruntime.QName(meta.Kind, normalized, resolver, needKey, s.notationDeclared, s.canonicalBuffers())
	s.restoreCanonicalBuffers(bufs)
	if err != nil {
		return nil, err
	}
	return s.finishCanonicalResult(metrics, result), nil
}

func (s *Session) notationDeclared(canon []byte) bool {
	if s == nil || s.rt == nil || len(s.rt.Notations) == 0 {
		return false
	}
	ns, local, err := valruntime.SplitQName(canon)
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
