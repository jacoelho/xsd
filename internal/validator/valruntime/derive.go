package valruntime

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/value"
)

// Derive converts one canonical primitive lexical value into its runtime key.
// The caller owns dst and may reuse it across calls.
func Derive(kind runtime.ValidatorKind, canonical, dst []byte) (runtime.ValueKind, []byte, error) {
	switch kind {
	case runtime.VAnyURI:
		return runtime.VKString, runtime.StringKeyBytes(dst[:0], 1, canonical), nil
	case runtime.VQName, runtime.VNotation:
		tag := byte(0)
		if kind == runtime.VNotation {
			tag = 1
		}
		key := runtime.QNameKeyCanonical(dst[:0], tag, canonical)
		if len(key) == 0 {
			return runtime.VKInvalid, nil, diag.Invalid("invalid QName key")
		}
		return runtime.VKQName, key, nil
	case runtime.VHexBinary:
		decoded, err := value.ParseHexBinary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, diag.Invalid(err.Error())
		}
		return runtime.VKBinary, runtime.BinaryKeyBytes(dst[:0], 0, decoded), nil
	case runtime.VBase64Binary:
		decoded, err := value.ParseBase64Binary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, diag.Invalid(err.Error())
		}
		return runtime.VKBinary, runtime.BinaryKeyBytes(dst[:0], 1, decoded), nil
	default:
		keyKind, keyBytes, err := runtime.KeyForValidatorKind(kind, canonical)
		if err != nil {
			return runtime.VKInvalid, nil, diag.Invalid(err.Error())
		}
		dst = append(dst[:0], keyBytes...)
		return keyKind, dst, nil
	}
}
