package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

type unionMemberRunInput struct {
	resolver          value.NSResolver
	callerMetrics     *valueMetrics
	memberValidators  []runtime.ValidatorID
	memberTypes       []runtime.TypeID
	memberSameWS      []uint8
	normalized        []byte
	memberLexical     []byte
	enumIDs           []runtime.EnumID
	opts              valueOptions
	needKey           bool
	needMemberMetrics bool
}

type unionMemberOutcome struct {
	firstErr  error
	canonical []byte
	matched   bool
	sawValid  bool
}

func (s *Session) tryUnionMembers(in unionMemberRunInput) unionMemberOutcome {
	out := unionMemberOutcome{}
	for i, member := range in.memberValidators {
		if int(member) >= len(s.rt.Validators.Meta) {
			if out.firstErr == nil {
				out.firstErr = valueErrorf(valueErrInvalid, "validator %d out of range", member)
			}
			continue
		}
		memberMeta := s.rt.Validators.Meta[member]
		memberLex := in.memberLexical
		memberOpts := unionMemberValueOptions(in.opts, in.needKey)
		if in.opts.applyWhitespace && i < len(in.memberSameWS) && in.memberSameWS[i] != 0 {
			// optimization: reuse union-normalized text when the member uses the same whitespace handling.
			memberOpts.applyWhitespace = false
			memberLex = in.normalized
		}
		if !memberOpts.applyWhitespace && unionMemberLexicallyImpossible(memberMeta.Kind, memberLex) {
			if mismatch := unionMemberLexicalMismatch(memberMeta.Kind); mismatch != nil && out.firstErr == nil {
				out.firstErr = mismatch
			}
			continue
		}

		var memberMetrics valueMetrics
		var memberMetricsPtr *valueMetrics
		if in.needMemberMetrics {
			memberMetricsPtr = &memberMetrics
		}
		canon, err := s.validateValueCore(member, memberLex, in.resolver, memberOpts, memberMetricsPtr)
		if err != nil {
			if out.firstErr == nil {
				out.firstErr = err
			}
			continue
		}
		out.sawValid = true
		if len(in.enumIDs) > 0 && !s.enumSetsContainAll(in.enumIDs, memberMetrics.keyKind, memberMetrics.keyBytes) {
			continue
		}
		if in.callerMetrics != nil {
			in.callerMetrics.keyKind = memberMetrics.keyKind
			in.callerMetrics.keyBytes = memberMetrics.keyBytes
			in.callerMetrics.keySet = memberMetrics.keySet
			if len(in.enumIDs) > 0 {
				in.callerMetrics.enumChecked = true
			}
			if i < len(in.memberTypes) {
				in.callerMetrics.actualTypeID = in.memberTypes[i]
			}
			in.callerMetrics.actualValidator = member
		}
		out.canonical = canon
		out.matched = true
		return out
	}
	return out
}

func unionMemberValueOptions(opts valueOptions, needKey bool) valueOptions {
	memberOpts := opts
	memberOpts.requireCanonical = true
	memberOpts.storeValue = false
	memberOpts.trackIDs = false
	memberOpts.needKey = needKey
	memberOpts.applyWhitespace = true
	return memberOpts
}
