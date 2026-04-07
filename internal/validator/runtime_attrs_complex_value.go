package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrValue(
	validated []Start,
	attr Start,
	resolver value.NSResolver,
	storeAttrs bool,
	spec ValueSpec,
	seenID *bool,
) ([]Start, error) {
	return ValidateValue(
		validated,
		attr,
		storeAttrs,
		spec,
		seenID,
		ValidateValueCallbacks{
			Validate: func(validator runtime.ValidatorID, lexical []byte, store bool) (ValueResult, error) {
				var metrics ValueMetrics
				opts := valueOptions{
					ApplyWhitespace:  true,
					TrackIDs:         true,
					RequireCanonical: spec.Fixed.Present,
					StoreValue:       store,
					NeedKey:          spec.Fixed.Present,
				}
				canon, err := s.validateValueCore(validator, lexical, resolver, opts, &metrics)
				if err != nil {
					return ValueResult{}, err
				}
				keyKind, keyBytes, _ := metrics.State.Key()
				return ValueResult{
					Canonical: canon,
					KeyKind:   keyKind,
					KeyBytes:  keyBytes,
					HasKey:    metrics.State.HasKey(),
				}, nil
			},
			IsIDValidator: s.isIDValidator,
			AppendCanonical: func(validated []Start, attr Start, store bool, canonical []byte, keyKind runtime.ValueKind, keyBytes []byte) []Start {
				return StoreCanonical(validated, attr, store, s.ensureAttrNameStable, canonical, keyKind, keyBytes)
			},
			MatchFixed: func(spec ValueSpec, result ValueResult) (bool, error) {
				return matchFixedValue(
					spec.Validator,
					spec.FixedMember,
					result.Canonical,
					result.KeyKind,
					result.KeyBytes,
					result.HasKey,
					spec.Fixed,
					spec.FixedKey,
					func(ref runtime.ValueRef) []byte { return valueBytes(s.rt.Values, ref) },
					func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
						return s.keyForCanonicalValue(validator, canonical, resolver, member)
					},
				)
			},
		},
	)
}
