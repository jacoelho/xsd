package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
)

func (s *Session) ensureAttrNameStable(attr *attrs.Start) {
	if s == nil || attr == nil || attr.NameCached {
		return
	}
	attr.Local, attr.NSBytes, attr.NameCached = s.Names.Stabilize(attr.Local, attr.NSBytes, attr.NameCached)
}

func (s *Session) isIDValidator(id runtime.ValidatorID) bool {
	meta, ok, err := s.validatorMetaIfPresent(id)
	if err != nil || !ok {
		return false
	}
	if meta.Kind != runtime.VString {
		return false
	}
	kind, ok := s.stringKind(meta)
	if !ok {
		return false
	}
	return kind == runtime.StringID
}
