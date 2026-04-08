package validator

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) prepareValueMetrics(plan valueExecutionPlan, metricState *ValueMetrics) (*ValueMetrics, bool) {
	if metricState != nil || !plan.NeedLocalMetrics {
		return metricState, false
	}
	return &ValueMetrics{}, true
}

func (s *Session) normalizeValueInput(meta runtime.ValidatorMeta, lexical []byte, opts valueOptions, plan valueExecutionPlan) ([]byte, func()) {
	if !opts.ApplyWhitespace {
		return lexical, func() {}
	}
	mode := runtimeWhitespaceValueMode(meta.WhiteSpace)
	if !plan.UseScratchNormalization {
		normalized := value.NormalizeWhitespace(mode, lexical, s.normBuf)
		return normalized, func() {}
	}
	if !value.NeedsWhitespaceNormalization(mode, lexical) {
		return lexical, func() {}
	}
	buf := s.pushNormBuf(len(lexical))
	normalized := value.NormalizeWhitespace(mode, lexical, buf)
	return normalized, s.popNormBuf
}

func (s *Session) validateValueWithoutCanonical(id runtime.ValidatorID, meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) ([]byte, error) {
	canon, err := s.validateValueNoCanonical(meta, normalized, resolver, opts)
	if err != nil {
		return nil, err
	}
	if err := s.trackValueIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (s *Session) validateValueWithCanonical(
	id runtime.ValidatorID,
	meta runtime.ValidatorMeta,
	lexical, normalized []byte,
	resolver value.NSResolver,
	opts valueOptions,
	plan valueExecutionPlan,
	metrics *ValueMetrics,
	metricsInternal bool,
) ([]byte, error) {
	canon, err := s.canonicalizeValueCore(meta, normalized, lexical, resolver, opts, plan.NeedKey, metrics)
	if err != nil {
		return nil, err
	}
	keyBuf, err := validateRuntimeFacets(
		meta,
		s.rt.Facets,
		s.rt.Patterns,
		s.rt.Enums,
		s.rt.Values,
		normalized,
		canon,
		metrics,
		s.keyTmp[:0],
	)
	if err != nil {
		return nil, err
	}
	s.keyTmp = keyBuf
	canon = s.finishCanonicalValue(canon, opts, plan, metrics, metricsInternal)
	if err := s.trackValueIDs(id, meta, canon, resolver, opts, metrics); err != nil {
		return nil, err
	}
	return canon, nil
}

func (s *Session) finishCanonicalValue(canonical []byte, opts valueOptions, plan valueExecutionPlan, metrics *ValueMetrics, metricsInternal bool) []byte {
	if plan.CloneCanonical {
		canonical = slices.Clone(canonical)
	}
	return s.finalizeValue(canonical, opts, metrics, metricsInternal)
}

func (s *Session) trackValueIDs(id runtime.ValidatorID, meta runtime.ValidatorMeta, canonical []byte, resolver value.NSResolver, opts valueOptions, metrics *ValueMetrics) error {
	if !opts.TrackIDs {
		return nil
	}
	if meta.Flags&runtime.ValidatorMayTrackIDs == 0 {
		return nil
	}
	return s.trackValidatedIDs(id, canonical, resolver, metrics)
}
