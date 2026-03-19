package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateValueInternalOptions(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valruntime.Options) ([]byte, error) {
	return s.validateValueCore(id, lexical, resolver, opts, nil)
}

func (s *Session) validateValueInternalWithMetrics(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valruntime.Options) ([]byte, valruntime.State, error) {
	metricState := s.acquireMetricsState()
	defer s.releaseMetricsState()
	canon, err := s.validateValueCore(id, lexical, resolver, opts, metricState)
	if err != nil {
		return nil, valruntime.State{}, err
	}
	return canon, *metricState, nil
}
