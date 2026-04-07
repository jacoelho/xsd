package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) keyForCanonicalValue(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
	meta, err := s.validatorMeta(id)
	if err != nil {
		return runtime.VKInvalid, nil, err
	}
	switch meta.Kind {
	case runtime.VList:
		keyKind, listKey, err := deriveCanonicalListKey(meta, s.rt.Validators, canonical, s.keyTmp[:0], func(itemValidator runtime.ValidatorID, itemValue []byte) (runtime.ValueKind, []byte, error) {
			return s.keyForCanonicalValue(itemValidator, itemValue, resolver, 0)
		})
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		s.keyTmp = listKey
		return keyKind, listKey, nil
	case runtime.VUnion:
		if member != 0 {
			return s.keyForCanonicalValue(member, canonical, resolver, 0)
		}
		actualValidator, err := s.lookupActualUnionValidator(id, canonical, resolver)
		if err == nil && actualValidator != 0 {
			return s.keyForCanonicalValue(actualValidator, canonical, resolver, 0)
		}
		return runtime.VKInvalid, nil, xsderrors.Invalid("union value does not match any member type")
	default:
		keyKind, keyBytes, err := deriveCanonicalPrimitiveKey(meta.Kind, canonical, s.keyTmp[:0])
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		s.keyTmp = keyBytes
		return keyKind, keyBytes, nil
	}
}

func deriveCanonicalPrimitiveKey(kind runtime.ValidatorKind, canonical, dst []byte) (runtime.ValueKind, []byte, error) {
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
			return runtime.VKInvalid, nil, xsderrors.Invalid("invalid QName key")
		}
		return runtime.VKQName, key, nil
	case runtime.VHexBinary:
		decoded, err := value.ParseHexBinary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, xsderrors.Invalid(err.Error())
		}
		return runtime.VKBinary, runtime.BinaryKeyBytes(dst[:0], 0, decoded), nil
	case runtime.VBase64Binary:
		decoded, err := value.ParseBase64Binary(canonical)
		if err != nil {
			return runtime.VKInvalid, nil, xsderrors.Invalid(err.Error())
		}
		return runtime.VKBinary, runtime.BinaryKeyBytes(dst[:0], 1, decoded), nil
	default:
		keyKind, keyBytes, err := runtime.KeyForValidatorKind(kind, canonical)
		if err != nil {
			return runtime.VKInvalid, nil, xsderrors.Invalid(err.Error())
		}
		dst = append(dst[:0], keyBytes...)
		return keyKind, dst, nil
	}
}
