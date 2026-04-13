package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
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
	validator, err := newRuntimeFacetValidator(RuntimeProgram{
		Meta:       meta,
		Facets:     facetCode,
		Patterns:   patterns,
		Enums:      enums,
		Values:     values,
		Normalized: normalized,
		Canonical:  canonical,
	}, metrics.result(), metrics.cache(), keyBuf)
	if err != nil {
		return keyBuf, err
	}
	if err := validator.run(); err != nil {
		return validator.keyBuf, err
	}
	return validator.keyBuf, nil
}

type runtimeFacetValidator struct {
	program []runtime.FacetInstr
	in      RuntimeProgram
	state   *ValueState
	cache   *ValueCache
	keyBuf  []byte
}

func newRuntimeFacetValidator(in RuntimeProgram, state *ValueState, cache *ValueCache, keyBuf []byte) (*runtimeFacetValidator, error) {
	program, err := RuntimeProgramSlice(in.Meta, in.Facets)
	if err != nil {
		return nil, xsderrors.Invalidf("%v", err)
	}
	return &runtimeFacetValidator{
		program: program,
		in:      in,
		state:   state,
		cache:   cache,
		keyBuf:  keyBuf,
	}, nil
}

func (v *runtimeFacetValidator) run() error {
	for _, instr := range v.program {
		if err := v.validateInstruction(instr); err != nil {
			return err
		}
	}
	return nil
}

func (v *runtimeFacetValidator) validateInstruction(instr runtime.FacetInstr) error {
	switch instr.Op {
	case runtime.FPattern:
		return v.validatePattern(instr)
	case runtime.FEnum:
		return v.validateEnum(instr)
	case runtime.FMinInclusive, runtime.FMaxInclusive, runtime.FMinExclusive, runtime.FMaxExclusive:
		return v.validateRange(instr)
	case runtime.FLength, runtime.FMinLength, runtime.FMaxLength:
		return v.validateLength(instr)
	case runtime.FTotalDigits, runtime.FFractionDigits:
		return v.validateDigits(instr)
	default:
		return xsderrors.Invalidf("unknown facet op %d", instr.Op)
	}
}

func (v *runtimeFacetValidator) validatePattern(instr runtime.FacetInstr) error {
	if v.in.Meta.Kind == runtime.VUnion || v.state.PatternChecked() {
		return nil
	}
	if int(instr.Arg0) >= len(v.in.Patterns) {
		return xsderrors.Invalidf("pattern %d out of range", instr.Arg0)
	}
	pat := v.in.Patterns[instr.Arg0]
	if pat.Re != nil && !pat.Re.Match(v.in.Normalized) {
		return xsderrors.Facetf("pattern violation")
	}
	return nil
}

func (v *runtimeFacetValidator) validateEnum(instr runtime.FacetInstr) error {
	if v.state.EnumChecked() {
		return nil
	}
	kind, key, err := v.enumKey()
	if err != nil {
		return err
	}
	if !runtime.EnumContains(&v.in.Enums, runtime.EnumID(instr.Arg0), kind, key) {
		return xsderrors.Facetf("enumeration violation")
	}
	return nil
}

func (v *runtimeFacetValidator) validateRange(instr runtime.FacetInstr) error {
	ref := runtime.ValueRef{Off: instr.Arg0, Len: instr.Arg1, Present: true}
	bound := facetValueBytes(v.in.Values, ref)
	if bound == nil {
		return xsderrors.Invalid("range facet bound out of range")
	}
	return checkRuntimeRange(instr.Op, v.in.Meta.Kind, v.in.Canonical, bound, v.cache)
}

func (v *runtimeFacetValidator) validateLength(instr runtime.FacetInstr) error {
	if v.in.Meta.Kind == runtime.VQName || v.in.Meta.Kind == runtime.VNotation {
		return nil
	}
	length, err := v.cache.Length(v.in.Meta.Kind, v.in.Normalized)
	if err != nil {
		return err
	}
	switch instr.Op {
	case runtime.FLength:
		if length != int(instr.Arg0) {
			return xsderrors.Facetf("length violation")
		}
	case runtime.FMinLength:
		if length < int(instr.Arg0) {
			return xsderrors.Facetf("minLength violation")
		}
	case runtime.FMaxLength:
		if length > int(instr.Arg0) {
			return xsderrors.Facetf("maxLength violation")
		}
	}
	return nil
}

func (v *runtimeFacetValidator) validateDigits(instr runtime.FacetInstr) error {
	total, fraction, err := v.cache.DigitCounts(v.in.Meta.Kind, v.in.Canonical)
	if err != nil {
		return err
	}
	switch instr.Op {
	case runtime.FTotalDigits:
		if total > int(instr.Arg0) {
			return xsderrors.Facetf("totalDigits violation")
		}
	case runtime.FFractionDigits:
		if fraction > int(instr.Arg0) {
			return xsderrors.Facetf("fractionDigits violation")
		}
	}
	return nil
}

func (v *runtimeFacetValidator) enumKey() (runtime.ValueKind, []byte, error) {
	if kind, key, ok := v.state.Key(); ok {
		return kind, key, nil
	}
	kind, key, err := derive(v.in.Meta.Kind, v.in.Canonical, v.keyBuf[:0])
	if err != nil {
		return runtime.VKInvalid, nil, err
	}
	v.keyBuf = key
	v.state.SetKey(kind, key)
	return kind, key, nil
}
