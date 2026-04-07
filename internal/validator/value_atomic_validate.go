package validator

import (
	"fmt"
	"unsafe"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateAtomicNoCanonical(meta runtime.ValidatorMeta, normalized []byte) error {
	stringKind := runtime.StringAny
	if meta.Kind == runtime.VString {
		kind, ok := s.stringKind(meta)
		if !ok {
			return xsderrors.Invalid("string validator out of range")
		}
		stringKind = kind
	}
	integerKind := runtime.IntegerAny
	if meta.Kind == runtime.VInteger {
		kind, ok := s.integerKind(meta)
		if !ok {
			return xsderrors.Invalid("integer validator out of range")
		}
		integerKind = kind
	}
	if err := validateAtomicLexical(meta.Kind, stringKind, integerKind, normalized); err != nil {
		return xsderrors.Invalid(err.Error())
	}
	return nil
}

func validateAtomicLexical(kind runtime.ValidatorKind, stringKind runtime.StringKind, integerKind runtime.IntegerKind, normalized []byte) error {
	switch kind {
	case runtime.VString:
		return runtime.ValidateStringKind(stringKind, normalized)
	case runtime.VBoolean:
		_, err := value.ParseBoolean(normalized)
		return err
	case runtime.VDecimal:
		if _, err := num.ParseDec(normalized); err != nil {
			return fmt.Errorf("invalid decimal")
		}
		return nil
	case runtime.VInteger:
		intVal, err := num.ParseInt(normalized)
		if err != nil {
			return fmt.Errorf("invalid integer")
		}
		return runtime.ValidateIntegerKind(integerKind, intVal)
	case runtime.VFloat:
		if err := num.ValidateFloatLexical(normalized); err != nil {
			return fmt.Errorf("invalid float")
		}
		return nil
	case runtime.VDouble:
		if err := num.ValidateFloatLexical(normalized); err != nil {
			return fmt.Errorf("invalid double")
		}
		return nil
	case runtime.VDuration:
		_, err := value.ParseDuration(unsafe.String(unsafe.SliceData(normalized), len(normalized)))
		return err
	default:
		return fmt.Errorf("unsupported atomic kind %d", kind)
	}
}
