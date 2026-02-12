package validator

import (
	"unsafe"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateAtomicNoCanonical(meta runtime.ValidatorMeta, normalized []byte) error {
	switch meta.Kind {
	case runtime.VString:
		kind, ok := s.stringKind(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "string validator out of range")
		}
		if err := runtime.ValidateStringKind(kind, normalized); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	case runtime.VBoolean:
		if _, err := value.ParseBoolean(normalized); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	case runtime.VDecimal:
		if _, perr := num.ParseDec(normalized); perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
	case runtime.VInteger:
		kind, ok := s.integerKind(meta)
		if !ok {
			return valueErrorf(valueErrInvalid, "integer validator out of range")
		}
		intVal, perr := num.ParseInt(normalized)
		if perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		if err := runtime.ValidateIntegerKind(kind, intVal); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	case runtime.VFloat:
		if perr := num.ValidateFloatLexical(normalized); perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid float")
		}
	case runtime.VDouble:
		if perr := num.ValidateFloatLexical(normalized); perr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid double")
		}
	case runtime.VDuration:
		if _, err := durationlex.Parse(unsafe.String(unsafe.SliceData(normalized), len(normalized))); err != nil {
			return valueErrorMsg(valueErrInvalid, err.Error())
		}
	default:
		return valueErrorf(valueErrInvalid, "unsupported atomic kind %d", meta.Kind)
	}
	return nil
}
