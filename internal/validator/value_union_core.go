package validator

import (
	facetengine "github.com/jacoelho/xsd/internal/facets"
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
	if s == nil || s.rt == nil {
		return nil, valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	facets, err := facetengine.RuntimeProgramSlice(meta, s.rt.Facets)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	enumIDs := facetengine.RuntimeProgramEnumIDs(facets)
	hasPatterns := false
	for _, instr := range facets {
		if instr.Op == runtime.FPattern {
			hasPatterns = true
			break
		}
	}
	if hasPatterns {
		checked, err := s.checkUnionPatterns(facets, normalized)
		if err != nil {
			return nil, err
		}
		if metrics != nil {
			metrics.patternChecked = checked
		}
	}
	memberLexical := lexical
	if memberLexical == nil {
		memberLexical = normalized
	}
	needMemberMetrics := needKey || len(enumIDs) > 0 || metrics != nil
	sawValid := false
	var firstErr error
	for i, member := range memberValidators {
		if int(member) >= len(s.rt.Validators.Meta) {
			if firstErr == nil {
				firstErr = valueErrorf(valueErrInvalid, "validator %d out of range", member)
			}
			continue
		}
		memberMeta := s.rt.Validators.Meta[member]
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
		if !memberOpts.applyWhitespace && unionMemberLexicallyImpossible(memberMeta.Kind, memberLex) {
			if mismatch := unionMemberLexicalMismatch(memberMeta.Kind); mismatch != nil {
				if firstErr == nil {
					firstErr = mismatch
				}
			}
			continue
		}

		var memberMetrics valueMetrics
		var memberMetricsPtr *valueMetrics
		if needMemberMetrics {
			memberMetricsPtr = &memberMetrics
		}
		canon, err := s.validateValueCore(member, memberLex, resolver, memberOpts, memberMetricsPtr)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		sawValid = true
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
		if len(enumIDs) > 0 {
			return nil, valueErrorf(valueErrFacet, "enumeration violation")
		}
	}
	if firstErr == nil {
		firstErr = valueErrorf(valueErrInvalid, "union value does not match any member type")
	}
	return nil, firstErr
}
