package validator

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
)

// RuntimeProgram contains compiled runtime facet inputs for one validator.
type RuntimeProgram struct {
	Enums      runtime.EnumTable
	Facets     []runtime.FacetInstr
	Patterns   []runtime.Pattern
	Values     runtime.ValueBlob
	Normalized []byte
	Canonical  []byte
	Meta       runtime.ValidatorMeta
}

// RuntimeCallbacks provide validator-owned operations for runtime facet checks.
type RuntimeCallbacks struct {
	SkipPattern      func() bool
	SkipEnum         func() bool
	CachedEnumKey    func() (runtime.ValueKind, []byte, bool)
	DeriveEnumKey    func(canonical []byte) (runtime.ValueKind, []byte, error)
	StoreEnumKey     func(kind runtime.ValueKind, key []byte)
	CheckRange       func(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte) error
	ValueLength      func(kind runtime.ValidatorKind, normalized []byte) (int, error)
	ShouldSkipLength func(kind runtime.ValidatorKind) bool
	DigitCounts      func(kind runtime.ValidatorKind, canonical []byte) (int, int, error)
	Invalidf         func(format string, args ...any) error
	FacetViolation   func(name string) error
}

// ValidateRuntimeProgram evaluates runtime facet instructions.
func ValidateRuntimeProgram(in RuntimeProgram, cb RuntimeCallbacks) error {
	program, err := RuntimeProgramSlice(in.Meta, in.Facets)
	if err != nil {
		return invalidRuntimeProgramf(cb, "%v", err)
	}
	eval := runtimeProgramEvaluator{in: in, cb: cb}
	for _, instr := range program {
		if err := eval.validateInstruction(instr); err != nil {
			return err
		}
	}
	return nil
}

type runtimeProgramEvaluator struct {
	cb RuntimeCallbacks
	in RuntimeProgram
}

func (e runtimeProgramEvaluator) validateInstruction(instr runtime.FacetInstr) error {
	switch instr.Op {
	case runtime.FPattern:
		return e.validatePattern(instr)
	case runtime.FEnum:
		return e.validateEnum(instr)
	case runtime.FMinInclusive, runtime.FMaxInclusive, runtime.FMinExclusive, runtime.FMaxExclusive:
		return e.validateRange(instr)
	case runtime.FLength, runtime.FMinLength, runtime.FMaxLength:
		return e.validateLength(instr)
	case runtime.FTotalDigits, runtime.FFractionDigits:
		return e.validateDigits(instr)
	}
	return e.invalidf("unknown facet op %d", instr.Op)
}

func (e runtimeProgramEvaluator) validatePattern(instr runtime.FacetInstr) error {
	if e.in.Meta.Kind == runtime.VUnion {
		return nil
	}
	if e.cb.SkipPattern != nil && e.cb.SkipPattern() {
		return nil
	}
	if int(instr.Arg0) >= len(e.in.Patterns) {
		return e.invalidf("pattern %d out of range", instr.Arg0)
	}
	pat := e.in.Patterns[instr.Arg0]
	if pat.Re != nil && !pat.Re.Match(e.in.Normalized) {
		return e.facetViolation("pattern")
	}
	return nil
}

func (e runtimeProgramEvaluator) validateEnum(instr runtime.FacetInstr) error {
	if e.cb.SkipEnum != nil && e.cb.SkipEnum() {
		return nil
	}
	kind, key, err := e.enumKey()
	if err != nil {
		return err
	}
	enumID := runtime.EnumID(instr.Arg0)
	if !runtime.EnumContains(&e.in.Enums, enumID, kind, key) {
		return e.facetViolation("enumeration")
	}
	return nil
}

func (e runtimeProgramEvaluator) validateRange(instr runtime.FacetInstr) error {
	ref := runtime.ValueRef{Off: instr.Arg0, Len: instr.Arg1, Present: true}
	bound := facetValueBytes(e.in.Values, ref)
	if bound == nil {
		return e.invalidf("range facet bound out of range")
	}
	if e.cb.CheckRange == nil {
		return e.invalidf("range callback is nil")
	}
	return e.cb.CheckRange(instr.Op, e.in.Meta.Kind, e.in.Canonical, bound)
}

func (e runtimeProgramEvaluator) validateLength(instr runtime.FacetInstr) error {
	if e.cb.ShouldSkipLength != nil && e.cb.ShouldSkipLength(e.in.Meta.Kind) {
		return nil
	}
	if e.cb.ValueLength == nil {
		return e.invalidf("length callback is nil")
	}
	length, err := e.cb.ValueLength(e.in.Meta.Kind, e.in.Normalized)
	if err != nil {
		return err
	}
	switch instr.Op {
	case runtime.FLength:
		if length != int(instr.Arg0) {
			return e.facetViolation("length")
		}
	case runtime.FMinLength:
		if length < int(instr.Arg0) {
			return e.facetViolation("minLength")
		}
	case runtime.FMaxLength:
		if length > int(instr.Arg0) {
			return e.facetViolation("maxLength")
		}
	}
	return nil
}

func (e runtimeProgramEvaluator) validateDigits(instr runtime.FacetInstr) error {
	if e.cb.DigitCounts == nil {
		return e.invalidf("digits callback is nil")
	}
	total, fraction, err := e.cb.DigitCounts(e.in.Meta.Kind, e.in.Canonical)
	if err != nil {
		return err
	}
	switch instr.Op {
	case runtime.FTotalDigits:
		if total > int(instr.Arg0) {
			return e.facetViolation("totalDigits")
		}
	case runtime.FFractionDigits:
		if fraction > int(instr.Arg0) {
			return e.facetViolation("fractionDigits")
		}
	}
	return nil
}

func (e runtimeProgramEvaluator) enumKey() (runtime.ValueKind, []byte, error) {
	if e.cb.CachedEnumKey != nil {
		kind, key, ok := e.cb.CachedEnumKey()
		if ok {
			return kind, key, nil
		}
	}
	if e.cb.DeriveEnumKey == nil {
		return runtime.VKInvalid, nil, fmt.Errorf("enum key callback is nil")
	}
	kind, key, err := e.cb.DeriveEnumKey(e.in.Canonical)
	if err != nil {
		return runtime.VKInvalid, nil, err
	}
	if e.cb.StoreEnumKey != nil {
		e.cb.StoreEnumKey(kind, key)
	}
	return kind, key, nil
}

func (e runtimeProgramEvaluator) invalidf(format string, args ...any) error {
	return invalidRuntimeProgramf(e.cb, format, args...)
}

func (e runtimeProgramEvaluator) facetViolation(name string) error {
	if e.cb.FacetViolation != nil {
		return e.cb.FacetViolation(name)
	}
	return fmt.Errorf("%s violation", name)
}

func invalidRuntimeProgramf(cb RuntimeCallbacks, format string, args ...any) error {
	if cb.Invalidf != nil {
		return cb.Invalidf(format, args...)
	}
	return fmt.Errorf(format, args...)
}

// RuntimeProgramSlice returns the facet instruction slice for a validator meta.
func RuntimeProgramSlice(meta runtime.ValidatorMeta, facets []runtime.FacetInstr) ([]runtime.FacetInstr, error) {
	if meta.Facets.Len == 0 {
		return nil, nil
	}
	start := int(meta.Facets.Off)
	end := start + int(meta.Facets.Len)
	if start < 0 || end < 0 || end > len(facets) {
		return nil, fmt.Errorf("facet program out of range")
	}
	return facets[start:end], nil
}

// RuntimeProgramHasOp reports whether a validator facet program contains any requested op.
func RuntimeProgramHasOp(meta runtime.ValidatorMeta, facets []runtime.FacetInstr, ops ...runtime.FacetOp) (bool, error) {
	program, err := RuntimeProgramSlice(meta, facets)
	if err != nil {
		return false, err
	}
	for _, instr := range program {
		if slices.Contains(ops, instr.Op) {
			return true, nil
		}
	}
	return false, nil
}

// RuntimeProgramEnumIDs returns enumeration IDs referenced by facet instructions.
func RuntimeProgramEnumIDs(program []runtime.FacetInstr) []runtime.EnumID {
	if len(program) == 0 {
		return nil
	}
	out := make([]runtime.EnumID, 0, len(program))
	for _, instr := range program {
		if instr.Op == runtime.FEnum {
			out = append(out, runtime.EnumID(instr.Arg0))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func facetValueBytes(values runtime.ValueBlob, ref runtime.ValueRef) []byte {
	if !ref.Present {
		return nil
	}
	if ref.Len == 0 {
		return []byte{}
	}
	start := int(ref.Off)
	end := start + int(ref.Len)
	if start < 0 || end < 0 || end > len(values.Blob) {
		return nil
	}
	return values.Blob[start:end]
}
