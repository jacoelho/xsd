package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valruntime.Options, metricState *valruntime.State) ([]byte, error) {
	meta, err := s.validatorMeta(id)
	if err != nil {
		return nil, err
	}
	return valruntime.Execute(meta, lexical, opts, valruntime.HasLengthFacet(meta, s.rt.Facets), metricState, valruntime.ExecuteCallbacks[*valruntime.State]{
		PrepareMetrics: s.prepareValueMetrics,
		Normalize:      s.normalizeValueInput,
		ValidateNoCanonical: func(meta runtime.ValidatorMeta, normalized []byte, metrics *valruntime.State) ([]byte, error) {
			return s.validateValueWithoutCanonical(id, meta, normalized, resolver, opts, metrics)
		},
		ValidateCanonical: func(meta runtime.ValidatorMeta, lexical, normalized []byte, plan valruntime.Plan, metrics *valruntime.State, metricsInternal bool) ([]byte, error) {
			return s.validateValueWithCanonical(id, meta, lexical, normalized, resolver, opts, plan, metrics, metricsInternal)
		},
	})
}
