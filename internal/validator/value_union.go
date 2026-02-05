package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) unionMemberInfo(meta runtime.ValidatorMeta) ([]runtime.ValidatorID, []runtime.TypeID, []uint8, bool) {
	if int(meta.Index) >= len(s.rt.Validators.Union) {
		return nil, nil, nil, false
	}
	union := s.rt.Validators.Union[meta.Index]
	end := union.MemberOff + union.MemberLen
	if int(end) > len(s.rt.Validators.UnionMembers) || int(end) > len(s.rt.Validators.UnionMemberTypes) || int(end) > len(s.rt.Validators.UnionMemberSameWS) {
		return nil, nil, nil, false
	}
	return s.rt.Validators.UnionMembers[union.MemberOff:end],
		s.rt.Validators.UnionMemberTypes[union.MemberOff:end],
		s.rt.Validators.UnionMemberSameWS[union.MemberOff:end],
		true
}

func (s *Session) canonicalizeUnion(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *valueMetrics) ([]byte, error) {
	memberValidators, memberTypes, memberSameWS, ok := s.unionMemberInfo(meta)
	if !ok || len(memberValidators) == 0 {
		return nil, valueErrorf(valueErrInvalid, "union validator out of range")
	}
	facets, err := s.facetProgram(meta)
	if err != nil {
		return nil, err
	}
	enumIDs := collectEnumIDs(facets)
	hasPatterns := false
	for _, instr := range facets {
		if instr.Op == runtime.FPattern {
			hasPatterns = true
			break
		}
	}
	memberLexical := lexical
	if memberLexical == nil {
		memberLexical = normalized
	}
	sawValid := false
	var lastPatternErr error
	var lastErr error
	for i, member := range memberValidators {
		if int(member) >= len(s.rt.Validators.Meta) {
			lastErr = valueErrorf(valueErrInvalid, "validator %d out of range", member)
			continue
		}
		memberLex := memberLexical
		memberOpts := opts
		memberOpts.requireCanonical = true
		memberOpts.storeValue = false
		memberOpts.trackIDs = false
		memberOpts.needKey = needKey
		memberOpts.applyWhitespace = true
		if opts.applyWhitespace && i < len(memberSameWS) && memberSameWS[i] != 0 {
			// optimization: reuse union-normalized text when the member uses the same whitespace handling.
			memberOpts.applyWhitespace = false
			memberLex = normalized
		}
		canon, memberMetrics, err := s.validateValueInternalWithMetrics(member, memberLex, resolver, memberOpts)
		if err != nil {
			lastErr = err
			continue
		}
		sawValid = true
		if hasPatterns {
			memberMeta := s.rt.Validators.Meta[member]
			memberNormalized := memberLex
			if memberOpts.applyWhitespace {
				if memberMeta.Kind == runtime.VList || memberMeta.Kind == runtime.VUnion {
					buf := s.pushNormBuf(len(memberLex))
					memberNormalized = value.NormalizeWhitespace(memberMeta.WhiteSpace, memberLex, buf)
					s.popNormBuf()
				} else {
					memberNormalized = value.NormalizeWhitespace(memberMeta.WhiteSpace, memberLex, s.normBuf)
				}
			}
			checked, err := s.checkUnionPatterns(facets, memberNormalized)
			if err != nil {
				lastPatternErr = err
				continue
			}
			if metrics != nil {
				metrics.patternChecked = checked
			}
		}
		if len(enumIDs) > 0 && !s.enumSetsContainAll(enumIDs, memberMetrics.keyKind, memberMetrics.keyBytes) {
			continue
		}
		if metrics != nil {
			metrics.keyKind = memberMetrics.keyKind
			metrics.keyBytes = memberMetrics.keyBytes
			metrics.keySet = memberMetrics.keySet
			if len(enumIDs) > 0 {
				metrics.enumChecked = true
			}
			if i < len(memberTypes) {
				metrics.actualTypeID = memberTypes[i]
			}
			metrics.actualValidator = member
		}
		return canon, nil
	}
	if sawValid {
		if hasPatterns && lastPatternErr != nil {
			return nil, lastPatternErr
		}
		if len(enumIDs) > 0 {
			return nil, valueErrorf(valueErrFacet, "enumeration violation")
		}
	}
	if lastErr == nil {
		lastErr = valueErrorf(valueErrInvalid, "union value does not match any member type")
	}
	return nil, lastErr
}

func collectEnumIDs(facets []runtime.FacetInstr) []runtime.EnumID {
	if len(facets) == 0 {
		return nil
	}
	out := make([]runtime.EnumID, 0, len(facets))
	for _, instr := range facets {
		if instr.Op == runtime.FEnum {
			out = append(out, runtime.EnumID(instr.Arg0))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *Session) checkUnionPatterns(facets []runtime.FacetInstr, normalized []byte) (bool, error) {
	if len(facets) == 0 {
		return false, nil
	}
	seen := false
	for _, instr := range facets {
		if instr.Op != runtime.FPattern {
			continue
		}
		seen = true
		if int(instr.Arg0) >= len(s.rt.Patterns) {
			return seen, valueErrorf(valueErrInvalid, "pattern %d out of range", instr.Arg0)
		}
		pat := s.rt.Patterns[instr.Arg0]
		if pat.Re != nil && !pat.Re.Match(normalized) {
			return seen, valueErrorf(valueErrFacet, "pattern violation")
		}
	}
	return seen, nil
}

func (s *Session) enumSetsContainAll(enumIDs []runtime.EnumID, keyKind runtime.ValueKind, keyBytes []byte) bool {
	for _, enumID := range enumIDs {
		if !runtime.EnumContains(&s.rt.Enums, enumID, keyKind, keyBytes) {
			return false
		}
	}
	return true
}
