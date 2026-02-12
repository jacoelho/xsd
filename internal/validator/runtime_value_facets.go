package validator

import (
	facetengine "github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) hasLengthFacet(meta runtime.ValidatorMeta) bool {
	if s == nil || s.rt == nil || meta.Facets.Len == 0 {
		return false
	}
	ok, err := facetengine.RuntimeProgramHasOp(meta, s.rt.Facets, runtime.FLength, runtime.FMinLength, runtime.FMaxLength)
	return err == nil && ok
}

func (s *Session) validateRuntimeFacets(meta runtime.ValidatorMeta, normalized, canon []byte, metrics *valueMetrics) error {
	return facetengine.ValidateRuntimeProgram(
		facetengine.RuntimeProgram{
			Meta:       meta,
			Facets:     s.rt.Facets,
			Patterns:   s.rt.Patterns,
			Enums:      s.rt.Enums,
			Values:     s.rt.Values,
			Normalized: normalized,
			Canonical:  canon,
		},
		facetengine.RuntimeCallbacks{
			SkipPattern: func() bool {
				return metrics != nil && metrics.patternChecked
			},
			SkipEnum: func() bool {
				return metrics != nil && metrics.enumChecked
			},
			CachedEnumKey: func() (runtime.ValueKind, []byte, bool) {
				if metrics == nil || !metrics.keySet {
					return runtime.VKInvalid, nil, false
				}
				return metrics.keyKind, metrics.keyBytes, true
			},
			DeriveEnumKey: func(canonical []byte) (runtime.ValueKind, []byte, error) {
				return s.deriveKeyFromCanonical(meta.Kind, canonical)
			},
			StoreEnumKey: func(kind runtime.ValueKind, key []byte) {
				if metrics == nil {
					return
				}
				metrics.keyKind = kind
				metrics.keyBytes = key
				metrics.keySet = true
			},
			CheckRange: func(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte) error {
				switch kind {
				case runtime.VFloat, runtime.VDouble:
					return s.checkFloatRange(kind, op, canonical, bound, metrics)
				default:
					cmp, err := s.compareValue(kind, canonical, bound, metrics)
					if err != nil {
						return err
					}
					return compareRange(op, cmp)
				}
			},
			ValueLength: func(kind runtime.ValidatorKind, normalized []byte) (int, error) {
				return s.valueLength(runtime.ValidatorMeta{Kind: kind}, normalized, metrics)
			},
			ShouldSkipLength: func(kind runtime.ValidatorKind) bool {
				return kind == runtime.VQName || kind == runtime.VNotation
			},
			DigitCounts: func(kind runtime.ValidatorKind, canonical []byte) (int, int, error) {
				return digitCounts(kind, canonical, metrics)
			},
			Invalidf: func(format string, args ...any) error {
				return valueErrorf(valueErrInvalid, format, args...)
			},
			FacetViolation: func(name string) error {
				return valueErrorf(valueErrFacet, "%s violation", name)
			},
		},
	)
}
