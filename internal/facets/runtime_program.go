package facets

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
		return invalidf(cb, "%v", err)
	}
	for _, instr := range program {
		switch instr.Op {
		case runtime.FPattern:
			if in.Meta.Kind == runtime.VUnion {
				continue
			}
			if cb.SkipPattern != nil && cb.SkipPattern() {
				continue
			}
			if int(instr.Arg0) >= len(in.Patterns) {
				return invalidf(cb, "pattern %d out of range", instr.Arg0)
			}
			pat := in.Patterns[instr.Arg0]
			if pat.Re != nil && !pat.Re.Match(in.Normalized) {
				return facetViolation(cb, "pattern")
			}
		case runtime.FEnum:
			if cb.SkipEnum != nil && cb.SkipEnum() {
				continue
			}
			kind, key, ok := cachedEnumKey(cb)
			if !ok {
				derivedKind, derivedKey, err := deriveEnumKey(cb, in.Canonical)
				if err != nil {
					return err
				}
				kind = derivedKind
				key = derivedKey
				if cb.StoreEnumKey != nil {
					cb.StoreEnumKey(kind, key)
				}
			}
			enumID := runtime.EnumID(instr.Arg0)
			if !runtime.EnumContains(&in.Enums, enumID, kind, key) {
				return facetViolation(cb, "enumeration")
			}
		case runtime.FMinInclusive, runtime.FMaxInclusive, runtime.FMinExclusive, runtime.FMaxExclusive:
			ref := runtime.ValueRef{Off: instr.Arg0, Len: instr.Arg1, Present: true}
			bound := facetValueBytes(in.Values, ref)
			if bound == nil {
				return invalidf(cb, "range facet bound out of range")
			}
			if cb.CheckRange == nil {
				return invalidf(cb, "range callback is nil")
			}
			if err := cb.CheckRange(instr.Op, in.Meta.Kind, in.Canonical, bound); err != nil {
				return err
			}
		case runtime.FLength, runtime.FMinLength, runtime.FMaxLength:
			if cb.ShouldSkipLength != nil && cb.ShouldSkipLength(in.Meta.Kind) {
				continue
			}
			if cb.ValueLength == nil {
				return invalidf(cb, "length callback is nil")
			}
			length, err := cb.ValueLength(in.Meta.Kind, in.Normalized)
			if err != nil {
				return err
			}
			switch instr.Op {
			case runtime.FLength:
				if length != int(instr.Arg0) {
					return facetViolation(cb, "length")
				}
			case runtime.FMinLength:
				if length < int(instr.Arg0) {
					return facetViolation(cb, "minLength")
				}
			case runtime.FMaxLength:
				if length > int(instr.Arg0) {
					return facetViolation(cb, "maxLength")
				}
			}
		case runtime.FTotalDigits, runtime.FFractionDigits:
			if cb.DigitCounts == nil {
				return invalidf(cb, "digits callback is nil")
			}
			total, fraction, err := cb.DigitCounts(in.Meta.Kind, in.Canonical)
			if err != nil {
				return err
			}
			switch instr.Op {
			case runtime.FTotalDigits:
				if total > int(instr.Arg0) {
					return facetViolation(cb, "totalDigits")
				}
			case runtime.FFractionDigits:
				if fraction > int(instr.Arg0) {
					return facetViolation(cb, "fractionDigits")
				}
			}
		default:
			return invalidf(cb, "unknown facet op %d", instr.Op)
		}
	}
	return nil
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

func invalidf(cb RuntimeCallbacks, format string, args ...any) error {
	if cb.Invalidf != nil {
		return cb.Invalidf(format, args...)
	}
	return fmt.Errorf(format, args...)
}

func facetViolation(cb RuntimeCallbacks, name string) error {
	if cb.FacetViolation != nil {
		return cb.FacetViolation(name)
	}
	return fmt.Errorf("%s violation", name)
}

func cachedEnumKey(cb RuntimeCallbacks) (runtime.ValueKind, []byte, bool) {
	if cb.CachedEnumKey == nil {
		return runtime.VKInvalid, nil, false
	}
	return cb.CachedEnumKey()
}

func deriveEnumKey(cb RuntimeCallbacks, canonical []byte) (runtime.ValueKind, []byte, error) {
	if cb.DeriveEnumKey == nil {
		return runtime.VKInvalid, nil, fmt.Errorf("enum key callback is nil")
	}
	return cb.DeriveEnumKey(canonical)
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
