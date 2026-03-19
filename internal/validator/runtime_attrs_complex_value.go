package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/attrs"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateComplexAttrValue(
	validated []attrs.Start,
	attr attrs.Start,
	resolver value.NSResolver,
	storeAttrs bool,
	spec attrs.ValueSpec,
	seenID *bool,
) ([]attrs.Start, error) {
	return attrs.ValidateValue(
		validated,
		attr,
		storeAttrs,
		spec,
		seenID,
		attrs.ValidateValueCallbacks{
			Validate: func(validator runtime.ValidatorID, lexical []byte, store bool) (attrs.ValueResult, error) {
				canon, metrics, err := s.validateValueInternalWithMetrics(validator, lexical, resolver, valruntime.AttributeOptions(spec.Fixed.Present, store))
				if err != nil {
					return attrs.ValueResult{}, err
				}
				keyKind, keyBytes, _ := metrics.Result.Key()
				return attrs.ValueResult{
					Canonical: canon,
					KeyKind:   keyKind,
					KeyBytes:  keyBytes,
					HasKey:    metrics.Result.HasKey(),
				}, nil
			},
			IsIDValidator: s.isIDValidator,
			AppendCanonical: func(validated []attrs.Start, attr attrs.Start, store bool, canonical []byte, keyKind runtime.ValueKind, keyBytes []byte) []attrs.Start {
				return s.appendValidatedAttr(validated, attr, store, canonical, keyKind, keyBytes)
			},
			MatchFixed: func(spec attrs.ValueSpec, result attrs.ValueResult) (bool, error) {
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
