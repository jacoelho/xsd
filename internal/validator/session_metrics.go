package validator

func (s *Session) acquireMetricsState() *ValueMetrics {
	if s == nil {
		return &ValueMetrics{}
	}
	idx := s.metricsDepth
	if idx < len(s.metricsPool) {
		state := s.metricsPool[idx]
		*state = ValueMetrics{}
		s.metricsDepth++
		return state
	}
	state := &ValueMetrics{}
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
