package valruntime

import (
	"fmt"
	"unsafe"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// ValidateAtomic validates an atomic lexical value without producing a canonical form.
func ValidateAtomic(kind runtime.ValidatorKind, stringKind runtime.StringKind, integerKind runtime.IntegerKind, normalized []byte) error {
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

// ValidateTemporal validates a temporal lexical value without producing a canonical form.
func ValidateTemporal(kind runtime.ValidatorKind, normalized []byte) error {
	_, err := ParseTemporal(kind, normalized)
	return err
}

// ParseTemporal parses one temporal lexical value for runtime comparisons and canonicalization.
func ParseTemporal(kind runtime.ValidatorKind, lexical []byte) (value.Value, error) {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return value.Value{}, fmt.Errorf("unsupported temporal kind %d", kind)
	}
	return value.Parse(spec.Kind, lexical)
}

// ValidateAnyURI validates an xs:anyURI lexical value.
func ValidateAnyURI(normalized []byte) error {
	return value.ValidateAnyURI(normalized)
}

// ValidateHexBinary validates an xs:hexBinary lexical value.
func ValidateHexBinary(normalized []byte) error {
	_, err := value.ParseHexBinary(normalized)
	return err
}

// ValidateBase64Binary validates an xs:base64Binary lexical value.
func ValidateBase64Binary(normalized []byte) error {
	_, err := value.ParseBase64Binary(normalized)
	return err
}
