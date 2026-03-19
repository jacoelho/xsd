package valruntime

import (
	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

// Tables groups the compiled runtime tables used by facet evaluation.
type Tables struct {
	Facets   []runtime.FacetInstr
	Patterns []runtime.Pattern
	Enums    runtime.EnumTable
	Values   runtime.ValueBlob
}

// Validate evaluates one validator's runtime facet program and returns the
// updated caller-owned key scratch buffer.
func Validate(meta runtime.ValidatorMeta, tables Tables, normalized, canonical []byte, state *State, keyBuf []byte) ([]byte, error) {
	cache := state.MeasureCache()
	result := state.ResultState()
	load := Loader{
		Decimal: func(canonical []byte) (num.Dec, error) {
			return cache.Decimal(canonical)
		},
		Integer: func(canonical []byte) (num.Int, error) {
			return cache.Integer(canonical)
		},
		Float32: func(canonical []byte) (float32, num.FloatClass, error) {
			return cache.Float32(canonical)
		},
		Float64: func(canonical []byte) (float64, num.FloatClass, error) {
			return cache.Float64(canonical)
		},
	}

	err := facets.ValidateRuntimeProgram(
		facets.RuntimeProgram{
			Meta:       meta,
			Facets:     tables.Facets,
			Patterns:   tables.Patterns,
			Enums:      tables.Enums,
			Values:     tables.Values,
			Normalized: normalized,
			Canonical:  canonical,
		},
		facets.RuntimeCallbacks{
			SkipPattern: func() bool {
				return result.PatternChecked()
			},
			SkipEnum: func() bool {
				return result.EnumChecked()
			},
			CachedEnumKey: func() (runtime.ValueKind, []byte, bool) {
				return result.Key()
			},
			DeriveEnumKey: func(canonical []byte) (runtime.ValueKind, []byte, error) {
				keyKind, derived, err := Derive(meta.Kind, canonical, keyBuf[:0])
				if err != nil {
					return runtime.VKInvalid, nil, err
				}
				keyBuf = derived
				return keyKind, derived, nil
			},
			StoreEnumKey: func(kind runtime.ValueKind, key []byte) {
				result.SetKey(kind, key)
			},
			CheckRange: func(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte) error {
				return Check(op, kind, canonical, bound, load)
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
			Invalidf: diag.Invalidf,
			FacetViolation: func(name string) error {
				return diag.Facetf("%s violation", name)
			},
		},
	)
	return keyBuf, err
}
