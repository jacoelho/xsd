package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metricState *ValueMetrics) ([]byte, error) {
	meta, err := s.validatorMeta(id)
	if err != nil {
		return nil, err
	}

	plan := buildValueExecutionPlan(meta, opts, hasLengthFacet(meta, s.rt.Facets))
	metrics, metricsInternal := s.prepareValueMetrics(plan, metricState)
	normalized, finishNormalize := s.normalizeValueInput(meta, lexical, opts, plan)
	defer finishNormalize()

	if !plan.NeedCanonical {
		return s.validateValueWithoutCanonical(id, meta, normalized, resolver, opts, metrics)
	}
	return s.validateValueWithCanonical(id, meta, lexical, normalized, resolver, opts, plan, metrics, metricsInternal)
}

func hasLengthFacet(meta runtime.ValidatorMeta, facetCode []runtime.FacetInstr) bool {
	if meta.Facets.Len == 0 {
		return false
	}
	ok, err := semantics.RuntimeProgramHasOp(meta, facetCode, runtime.FLength, runtime.FMinLength, runtime.FMaxLength)
	return err == nil && ok
}
