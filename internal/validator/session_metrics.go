package validator

import "github.com/jacoelho/xsd/internal/validator/valruntime"

func (s *Session) acquireMetricsState() *valruntime.State {
	if s == nil {
		return &valruntime.State{}
	}
	idx := s.metricsDepth
	if idx < len(s.metricsPool) {
		state := s.metricsPool[idx]
		*state = valruntime.State{}
		s.metricsDepth++
		return state
	}
	state := &valruntime.State{}
	s.metricsPool = append(s.metricsPool, state)
	s.metricsDepth++
	return state
}

func (s *Session) releaseMetricsState() {
	if s == nil {
		return
	}
	if s.metricsDepth > 0 {
		s.metricsDepth--
	}
}
