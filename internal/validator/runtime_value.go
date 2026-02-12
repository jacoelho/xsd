package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func (s *Session) validateValueInternalOptions(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	return s.validateValueCore(id, lexical, resolver, opts, nil)
}

func (s *Session) validateValueInternalWithMetrics(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, ValueMetrics, error) {
	metrics := s.acquireValueMetrics()
	defer s.releaseValueMetrics()
	canon, err := s.validateValueCore(id, lexical, resolver, opts, metrics)
	if err != nil {
		return nil, ValueMetrics{}, err
	}
	return canon, *metrics, nil
}
