package valruntime

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

// UnionInput holds the runtime tables and lexical inputs needed for union matching.
type UnionInput struct {
	Patterns        []runtime.Pattern
	Facets          []runtime.FacetInstr
	Normalized      []byte
	Lexical         []byte
	Enums           *runtime.EnumTable
	Validators      runtime.ValidatorsBundle
	Meta            runtime.ValidatorMeta
	ApplyWhitespace bool
	NeedKey         bool
}

// UnionMemberSet contains the runtime member slices for one union validator.
type UnionMemberSet struct {
	Validators     []runtime.ValidatorID
	Types          []runtime.TypeID
	SameWhitespace []uint8
}

// UnionMemberResult carries selected member key data back to the caller.
type UnionMemberResult struct {
	KeyBytes []byte
	KeyKind  runtime.ValueKind
	KeySet   bool
}

// UnionMemberResultOf converts caller-owned result state into one union member result.
func UnionMemberResultOf(state interface {
	Key() (runtime.ValueKind, []byte, bool)
}) UnionMemberResult {
	if state == nil {
		return UnionMemberResult{}
	}
	keyKind, keyBytes, keySet := state.Key()
	return UnionMemberResult{
		KeyBytes: keyBytes,
		KeyKind:  keyKind,
		KeySet:   keySet,
	}
}

// UnionOutcome describes the union selection result.
type UnionOutcome struct {
	FirstErr        error
	Canonical       []byte
	KeyBytes        []byte
	ActualTypeID    runtime.TypeID
	ActualValidator runtime.ValidatorID
	KeyKind         runtime.ValueKind
	Matched         bool
	SawValid        bool
	PatternChecked  bool
	EnumChecked     bool
	KeySet          bool
}

// ValidateUnionMember validates one union member with caller-owned runtime state.
type ValidateUnionMember func(member runtime.ValidatorID, lexical []byte, applyWhitespace, needKey bool) ([]byte, UnionMemberResult, error)

// UnionMembers returns the member slices for one union validator.
func UnionMembers(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle) (UnionMemberSet, bool) {
	if int(meta.Index) >= len(validators.Union) {
		return UnionMemberSet{}, false
	}
	union := validators.Union[meta.Index]
	startMembers, endMembers, okMembers := checkedSpan(union.MemberOff, union.MemberLen, len(validators.UnionMembers))
	startTypes, endTypes, okTypes := checkedSpan(union.MemberOff, union.MemberLen, len(validators.UnionMemberTypes))
	startWS, endWS, okWS := checkedSpan(union.MemberOff, union.MemberLen, len(validators.UnionMemberSameWS))
	if !okMembers || !okTypes || !okWS {
		return UnionMemberSet{}, false
	}
	return UnionMemberSet{
		Validators:     validators.UnionMembers[startMembers:endMembers],
		Types:          validators.UnionMemberTypes[startTypes:endTypes],
		SameWhitespace: validators.UnionMemberSameWS[startWS:endWS],
	}, true
}

// MatchUnion selects the first union member that validates and passes union facets.
func MatchUnion(in UnionInput, validate ValidateUnionMember) UnionOutcome {
	members, ok := UnionMembers(in.Meta, in.Validators)
	if !ok || len(members.Validators) == 0 {
		return UnionOutcome{FirstErr: invalid("union validator out of range")}
	}
	program, err := facets.RuntimeProgramSlice(in.Meta, in.Facets)
	if err != nil {
		return UnionOutcome{FirstErr: invalid(err.Error())}
	}
	patternChecked, err := checkPatterns(program, in.Patterns, in.Normalized)
	if err != nil {
		return UnionOutcome{FirstErr: err}
	}
	enumIDs := facets.RuntimeProgramEnumIDs(program)
	memberLexical := in.Lexical
	if memberLexical == nil {
		memberLexical = in.Normalized
	}
	out := UnionOutcome{PatternChecked: patternChecked}
	for i, member := range members.Validators {
		if int(member) >= len(in.Validators.Meta) {
			if out.FirstErr == nil {
				out.FirstErr = invalidf("validator %d out of range", member)
			}
			continue
		}
		memberMeta := in.Validators.Meta[member]
		memberLex := memberLexical
		memberApplyWhitespace := true
		if in.ApplyWhitespace && i < len(members.SameWhitespace) && members.SameWhitespace[i] != 0 {
			// Reuse union-normalized text when the member uses the same whitespace handling.
			memberApplyWhitespace = false
			memberLex = in.Normalized
		}
		if !memberApplyWhitespace && memberLexicallyImpossible(memberMeta.Kind, memberLex) {
			if mismatch := memberLexicalMismatch(memberMeta.Kind); mismatch != nil && out.FirstErr == nil {
				out.FirstErr = mismatch
			}
			continue
		}

		canon, result, err := validate(member, memberLex, memberApplyWhitespace, in.NeedKey)
		if err != nil {
			if out.FirstErr == nil {
				out.FirstErr = err
			}
			continue
		}
		out.SawValid = true
		if len(enumIDs) > 0 && !enumSetsContainAll(in.Enums, enumIDs, result.KeyKind, result.KeyBytes) {
			continue
		}
		out.Canonical = canon
		out.Matched = true
		out.KeyBytes = result.KeyBytes
		out.KeyKind = result.KeyKind
		out.KeySet = result.KeySet
		out.EnumChecked = len(enumIDs) > 0
		if i < len(members.Types) {
			out.ActualTypeID = members.Types[i]
		}
		out.ActualValidator = member
		return out
	}
	if out.SawValid && len(enumIDs) > 0 {
		out.FirstErr = facet("enumeration violation")
		return out
	}
	if out.FirstErr == nil {
		out.FirstErr = invalid("union value does not match any member type")
	}
	return out
}

// isUnionIntegerLexical reports whether lexical is a valid integer lexical form.
func isUnionIntegerLexical(lexical []byte) bool {
	if len(lexical) == 0 {
		return false
	}
	start := 0
	if lexical[0] == '+' || lexical[0] == '-' {
		start = 1
	}
	if start >= len(lexical) {
		return false
	}
	for _, b := range lexical[start:] {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func checkPatterns(program []runtime.FacetInstr, patterns []runtime.Pattern, normalized []byte) (bool, error) {
	if len(program) == 0 {
		return false, nil
	}
	seen := false
	for _, instr := range program {
		if instr.Op != runtime.FPattern {
			continue
		}
		seen = true
		if int(instr.Arg0) >= len(patterns) {
			return seen, invalidf("pattern %d out of range", instr.Arg0)
		}
		pat := patterns[instr.Arg0]
		if pat.Re != nil && !pat.Re.Match(normalized) {
			return seen, facet("pattern violation")
		}
	}
	return seen, nil
}

func enumSetsContainAll(table *runtime.EnumTable, enumIDs []runtime.EnumID, keyKind runtime.ValueKind, keyBytes []byte) bool {
	if table == nil {
		return false
	}
	for _, enumID := range enumIDs {
		if !runtime.EnumContains(table, enumID, keyKind, keyBytes) {
			return false
		}
	}
	return true
}

func memberLexicallyImpossible(kind runtime.ValidatorKind, lexical []byte) bool {
	switch kind {
	case runtime.VInteger:
		return !isUnionIntegerLexical(lexical)
	default:
		return false
	}
}

func memberLexicalMismatch(kind runtime.ValidatorKind) error {
	switch kind {
	case runtime.VInteger:
		return invalid("invalid integer")
	default:
		return nil
	}
}

func invalid(msg string) error {
	return diag.New(xsderrors.ErrDatatypeInvalid, msg)
}

func invalidf(format string, args ...any) error {
	return invalid(fmt.Sprintf(format, args...))
}

func facet(msg string) error {
	return diag.New(xsderrors.ErrFacetViolation, msg)
}

func checkedSpan(off, ln uint32, size int) (start, end int, ok bool) {
	if size < 0 {
		return 0, 0, false
	}
	size64 := uint64(size)
	off64 := uint64(off)
	end64 := off64 + uint64(ln)
	if off64 > size64 || end64 > size64 {
		return 0, 0, false
	}
	return int(off64), int(end64), true
}
