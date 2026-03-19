package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) keyForCanonicalValue(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
	meta, err := s.validatorMeta(id)
	if err != nil {
		return runtime.VKInvalid, nil, err
	}
	switch meta.Kind {
	case runtime.VList:
		keyKind, listKey, err := valruntime.DeriveCanonicalListKey(meta, s.rt.Validators, canonical, s.keyTmp[:0], func(itemValidator runtime.ValidatorID, itemValue []byte) (runtime.ValueKind, []byte, error) {
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
		actualValidator := valruntime.ResolveActualUnionValidator(nil, func() (runtime.ValidatorID, error) {
			return s.lookupActualUnionValidator(id, canonical, resolver)
		})
		if actualValidator != 0 {
			return s.keyForCanonicalValue(actualValidator, canonical, resolver, 0)
		}
		return runtime.VKInvalid, nil, diag.Invalid("union value does not match any member type")
	default:
		keyKind, keyBytes, err := valruntime.Derive(meta.Kind, canonical, s.keyTmp[:0])
		if err != nil {
			return runtime.VKInvalid, nil, err
		}
		s.keyTmp = keyBytes
		return keyKind, keyBytes, nil
	}
}
