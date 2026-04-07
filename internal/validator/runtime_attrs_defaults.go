package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) applyDefaultAttrs(uses []runtime.AttrUse, present []bool, storeAttrs, seenID bool) ([]Applied, error) {
	readValue := func(ref runtime.ValueRef) []byte { return valueBytes(s.rt.Values, ref) }
	materializeKey := func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID, stored runtime.ValueKeyRef) (runtime.ValueKind, []byte, error) {
		return materializeValueKey(
			validator,
			canonical,
			member,
			stored,
			readValue,
			func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
				return s.keyForCanonicalValue(validator, canonical, nil, member)
			},
		)
	}
	return ApplyDefaults(
		uses,
		present,
		storeAttrs,
		seenID,
		s.attrAppliedBuf[:0],
		SelectDefaultOrFixed,
		s.isIDValidator,
		readValue,
		func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) error {
			return s.trackDefaultValue(validator, canonical, nil, member)
		},
		materializeKey,
		s.storeKey,
	)
}
