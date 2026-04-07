package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) canonicalizeUnion(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, xsderrors.Invalid("runtime schema missing")
	}
	return Union(
		s.rt.Patterns,
		s.rt.Facets,
		normalized,
		lexical,
		&s.rt.Enums,
		s.rt.Validators,
		meta,
		opts.ApplyWhitespace,
		needKey,
		metrics.result(),
		func(member runtime.ValidatorID, memberLex []byte, applyWhitespace, needKey bool) ([]byte, runtime.ValueKind, []byte, bool, error) {
			memberOpts := opts
			memberOpts.ApplyWhitespace = applyWhitespace
			memberOpts.TrackIDs = false
			memberOpts.RequireCanonical = true
			memberOpts.StoreValue = false
			memberOpts.NeedKey = needKey
			var memberMetrics ValueMetrics
			canon, err := s.validateValueCore(member, memberLex, resolver, memberOpts, &memberMetrics)
			if err != nil {
				return nil, runtime.VKInvalid, nil, false, err
			}
			keyKind, keyBytes, keySet := memberMetrics.State.Key()
			return canon, keyKind, keyBytes, keySet, nil
		},
	)
}
