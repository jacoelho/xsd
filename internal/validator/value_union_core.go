package validator

import (
	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeUnion(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	memberValidators, memberTypes, memberSameWS, ok := s.unionMemberInfo(meta)
	if !ok || len(memberValidators) == 0 {
		return nil, valueErrorf(valueErrInvalid, "union validator out of range")
	}
	if s == nil || s.rt == nil {
		return nil, valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	program, err := facets.RuntimeProgramSlice(meta, s.rt.Facets)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	enumIDs := facets.RuntimeProgramEnumIDs(program)
	hasPatterns := false
	for _, instr := range program {
		if instr.Op == runtime.FPattern {
			hasPatterns = true
			break
		}
	}
	if hasPatterns {
		checked, err := s.checkUnionPatterns(program, normalized)
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
	outcome := s.tryUnionMembers(unionMemberRunInput{
		memberValidators:  memberValidators,
		memberTypes:       memberTypes,
		memberSameWS:      memberSameWS,
		normalized:        normalized,
		memberLexical:     memberLexical,
		resolver:          resolver,
		opts:              opts,
		needKey:           needKey,
		enumIDs:           enumIDs,
		needMemberMetrics: needMemberMetrics,
		callerMetrics:     metrics,
	})
	if outcome.matched {
		return outcome.canonical, nil
	}
	if outcome.sawValid {
		if len(enumIDs) > 0 {
			return nil, valueErrorf(valueErrFacet, "enumeration violation")
		}
	}
	if outcome.firstErr == nil {
		outcome.firstErr = valueErrorf(valueErrInvalid, "union value does not match any member type")
	}
	return nil, outcome.firstErr
}
