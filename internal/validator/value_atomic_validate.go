package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
)

func (s *Session) validateAtomicNoCanonical(meta runtime.ValidatorMeta, normalized []byte) error {
	stringKind := runtime.StringAny
	if meta.Kind == runtime.VString {
		kind, ok := s.stringKind(meta)
		if !ok {
			return diag.Invalid("string validator out of range")
		}
		stringKind = kind
	}
	integerKind := runtime.IntegerAny
	if meta.Kind == runtime.VInteger {
		kind, ok := s.integerKind(meta)
		if !ok {
			return diag.Invalid("integer validator out of range")
		}
		integerKind = kind
	}
	if err := valruntime.ValidateAtomic(meta.Kind, stringKind, integerKind, normalized); err != nil {
		return diag.Invalid(err.Error())
	}
	return nil
}
