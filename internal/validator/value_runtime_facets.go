package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
)

// validateRuntimeFacets evaluates one validator's runtime facet program and returns the
// updated caller-owned key scratch buffer.
func validateRuntimeFacets(
	meta runtime.ValidatorMeta,
	facetCode []runtime.FacetInstr,
	patterns []runtime.Pattern,
	enums runtime.EnumTable,
	values runtime.ValueBlob,
	normalized, canonical []byte,
	metrics *ValueMetrics,
	keyBuf []byte,
) ([]byte, error) {
	state := metrics.result()
	cache := metrics.cache()
	err := semantics.ValidateRuntimeProgram(
		semantics.RuntimeProgram{
			Meta:       meta,
			Facets:     facetCode,
			Patterns:   patterns,
			Enums:      enums,
			Values:     values,
			Normalized: normalized,
			Canonical:  canonical,
		},
		semantics.RuntimeCallbacks{
			SkipPattern: func() bool {
				return state.PatternChecked()
			},
			SkipEnum: func() bool {
				return state.EnumChecked()
			},
			CachedEnumKey: func() (runtime.ValueKind, []byte, bool) {
				return state.Key()
			},
			DeriveEnumKey: func(canonical []byte) (runtime.ValueKind, []byte, error) {
				keyKind, derived, err := derive(meta.Kind, canonical, keyBuf[:0])
				if err != nil {
					return runtime.VKInvalid, nil, err
				}
				keyBuf = derived
				return keyKind, derived, nil
			},
			StoreEnumKey: func(kind runtime.ValueKind, key []byte) {
				state.SetKey(kind, key)
			},
			CheckRange: func(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte) error {
				return checkRuntimeRange(op, kind, canonical, bound, cache)
			},
			ValueLength: func(kind runtime.ValidatorKind, normalized []byte) (int, error) {
				return cache.Length(kind, normalized)
			},
			ShouldSkipLength: func(kind runtime.ValidatorKind) bool {
				return kind == runtime.VQName || kind == runtime.VNotation
			},
			DigitCounts: func(kind runtime.ValidatorKind, canonical []byte) (int, int, error) {
				return cache.DigitCounts(kind, canonical)
			},
			Invalidf: xsderrors.Invalidf,
			FacetViolation: func(name string) error {
				return xsderrors.Facetf("%s violation", name)
			},
		},
	)
	return keyBuf, err
}
