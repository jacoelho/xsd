package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeUnion(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valruntime.Options, needKey bool, metrics *valruntime.State) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, diag.Invalid("runtime schema missing")
	}
	outcome := valruntime.MatchUnion(
		valruntime.UnionInput{
			Patterns:        s.rt.Patterns,
			Facets:          s.rt.Facets,
			Normalized:      normalized,
			Lexical:         lexical,
			Enums:           &s.rt.Enums,
			Validators:      s.rt.Validators,
			Meta:            meta,
			ApplyWhitespace: opts.ApplyWhitespace,
			NeedKey:         needKey,
		},
		func(member runtime.ValidatorID, memberLex []byte, applyWhitespace, needKey bool) ([]byte, valruntime.UnionMemberResult, error) {
			memberOpts := valruntime.UnionMemberOptions(opts, applyWhitespace, needKey)
			canon, memberMetrics, err := s.validateValueInternalWithMetrics(member, memberLex, resolver, memberOpts)
			if err != nil {
				return nil, valruntime.UnionMemberResult{}, err
			}
			return canon, valruntime.UnionMemberResultOf(&memberMetrics.Result), nil
		},
	)
	return valruntime.ResolveUnion(outcome, metrics.ResultState())
}
