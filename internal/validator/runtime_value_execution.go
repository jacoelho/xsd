package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) prepareValueMetrics(plan valruntime.Plan, metricState *valruntime.State) (*valruntime.State, bool) {
	if metricState != nil || !plan.NeedLocalMetrics {
		return metricState, false
	}
	return &valruntime.State{}, true
}

func (s *Session) normalizeValueInput(meta runtime.ValidatorMeta, lexical []byte, opts valruntime.Options, plan valruntime.Plan) ([]byte, func()) {
	if !opts.ApplyWhitespace {
		return lexical, func() {}
	}
	if !plan.UseScratchNormalization {
		normalized := value.NormalizeWhitespace(runtimeWhitespaceValueMode(meta.WhiteSpace), lexical, s.normBuf)
		return normalized, func() {}
	}
	buf := s.pushNormBuf(len(lexical))
	normalized := value.NormalizeWhitespace(runtimeWhitespaceValueMode(meta.WhiteSpace), lexical, buf)
	return normalized, s.popNormBuf
}

func (s *Session) validateValueWithoutCanonical(id runtime.ValidatorID, meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valruntime.Options, metrics *valruntime.State) ([]byte, error) {
	canon, err := s.validateValueNoCanonical(meta, normalized, resolver, opts)
	if err != nil {
		return nil, err
	}
	if err := s.trackValueIDs(id, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (s *Session) validateValueWithCanonical(
	id runtime.ValidatorID,
	meta runtime.ValidatorMeta,
	lexical, normalized []byte,
	resolver value.NSResolver,
	opts valruntime.Options,
	plan valruntime.Plan,
	metrics *valruntime.State,
	metricsInternal bool,
) ([]byte, error) {
	canon, err := s.canonicalizeValueCore(meta, normalized, lexical, resolver, opts, plan.NeedKey, metrics)
	if err != nil {
		return nil, err
	}
	keyBuf, err := valruntime.Validate(meta, valruntime.Tables{
		Facets:   s.rt.Facets,
		Patterns: s.rt.Patterns,
		Enums:    s.rt.Enums,
		Values:   s.rt.Values,
	}, normalized, canon, metrics, s.keyTmp[:0])
	if err != nil {
		return nil, err
	}
	s.keyTmp = keyBuf
	canon = s.finishCanonicalValue(canon, opts, plan, metrics, metricsInternal)
	if err := s.trackValueIDs(id, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (s *Session) finishCanonicalValue(canonical []byte, opts valruntime.Options, plan valruntime.Plan, metrics *valruntime.State, metricsInternal bool) []byte {
	if plan.CloneCanonical {
		canonical = slices.Clone(canonical)
	}
	return s.finalizeValue(canonical, opts, metrics, metricsInternal)
}

func (s *Session) trackValueIDs(id runtime.ValidatorID, canonical []byte, resolver value.NSResolver, opts valruntime.Options, metrics *valruntime.State) error {
	if !opts.TrackIDs {
		return nil
	}
	return s.trackValidatedIDs(id, canonical, resolver, metrics)
}
