package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/runtime"
)

type unionOutcome struct {
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

func unionMembers(meta runtime.ValidatorMeta, validators runtime.ValidatorsBundle) ([]runtime.ValidatorID, []runtime.TypeID, []uint8, bool) {
	if int(meta.Index) >= len(validators.Union) {
		return nil, nil, nil, false
	}
	union := validators.Union[meta.Index]
	startMembers, endMembers, okMembers := checkedSpan(union.MemberOff, union.MemberLen, len(validators.UnionMembers))
	startTypes, endTypes, okTypes := checkedSpan(union.MemberOff, union.MemberLen, len(validators.UnionMemberTypes))
	startWS, endWS, okWS := checkedSpan(union.MemberOff, union.MemberLen, len(validators.UnionMemberSameWS))
	if !okMembers || !okTypes || !okWS {
		return nil, nil, nil, false
	}
	return validators.UnionMembers[startMembers:endMembers],
		validators.UnionMemberTypes[startTypes:endTypes],
		validators.UnionMemberSameWS[startWS:endWS],
		true
}

// Union validates union members and applies the selected result to state.
func Union(
	patterns []runtime.Pattern,
	facetCode []runtime.FacetInstr,
	normalized, lexical []byte,
	enums *runtime.EnumTable,
	validators runtime.ValidatorsBundle,
	meta runtime.ValidatorMeta,
	applyWhitespace bool,
	needKey bool,
	state *ValueState,
	validate func(member runtime.ValidatorID, lexical []byte, applyWhitespace, needKey bool) ([]byte, runtime.ValueKind, []byte, bool, error),
) ([]byte, error) {
	outcome := matchUnion(
		patterns,
		facetCode,
		normalized,
		lexical,
		enums,
		validators,
		meta,
		applyWhitespace,
		needKey,
		validate,
	)
	return resolveUnion(outcome, state)
}

func matchUnion(
	patterns []runtime.Pattern,
	facetCode []runtime.FacetInstr,
	normalized, lexical []byte,
	enums *runtime.EnumTable,
	validators runtime.ValidatorsBundle,
	meta runtime.ValidatorMeta,
	applyWhitespace bool,
	needKey bool,
	validate func(member runtime.ValidatorID, lexical []byte, applyWhitespace, needKey bool) ([]byte, runtime.ValueKind, []byte, bool, error),
) unionOutcome {
	members, memberTypes, memberSameWhitespace, ok := unionMembers(meta, validators)
	if !ok || len(members) == 0 {
		return unionOutcome{FirstErr: invalid("union validator out of range")}
	}
	program, err := facets.RuntimeProgramSlice(meta, facetCode)
	if err != nil {
		return unionOutcome{FirstErr: invalid(err.Error())}
	}
	patternChecked, err := checkPatterns(program, patterns, normalized)
	if err != nil {
		return unionOutcome{FirstErr: err}
	}
	enumIDs := facets.RuntimeProgramEnumIDs(program)
	memberLexical := lexical
	if memberLexical == nil {
		memberLexical = normalized
	}
	out := unionOutcome{PatternChecked: patternChecked}
	for i, member := range members {
		if int(member) >= len(validators.Meta) {
			if out.FirstErr == nil {
				out.FirstErr = invalidf("validator %d out of range", member)
			}
			continue
		}
		memberMeta := validators.Meta[member]
		memberLex := memberLexical
		memberApplyWhitespace := true
		if applyWhitespace && i < len(memberSameWhitespace) && memberSameWhitespace[i] != 0 {
			// Reuse union-normalized text when the member uses the same whitespace handling.
			memberApplyWhitespace = false
			memberLex = normalized
		}
		if !memberApplyWhitespace && memberMeta.Kind == runtime.VInteger && !isUnionIntegerLexical(memberLex) {
			if out.FirstErr == nil {
				out.FirstErr = invalid("invalid integer")
			}
			continue
		}

		canon, keyKind, keyBytes, keySet, err := validate(member, memberLex, memberApplyWhitespace, needKey)
		if err != nil {
			if out.FirstErr == nil {
				out.FirstErr = err
			}
			continue
		}
		out.SawValid = true
		if len(enumIDs) > 0 && !enumSetsContainAll(enums, enumIDs, keyKind, keyBytes) {
			continue
		}
		out.Canonical = canon
		out.Matched = true
		out.KeyBytes = keyBytes
		out.KeyKind = keyKind
		out.KeySet = keySet
		out.EnumChecked = len(enumIDs) > 0
		if i < len(memberTypes) {
			out.ActualTypeID = memberTypes[i]
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

func invalid(msg string) error {
	return xsderrors.New(xsderrors.ErrDatatypeInvalid, msg)
}

func invalidf(format string, args ...any) error {
	return invalid(fmt.Sprintf(format, args...))
}

func facet(msg string) error {
	return xsderrors.New(xsderrors.ErrFacetViolation, msg)
}
