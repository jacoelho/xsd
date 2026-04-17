package validator

func (s *Session) acquireMetricsState() *ValueMetrics {
	if s == nil {
		return &ValueMetrics{}
	}
	idx := s.buffers.metricsDepth
	if idx < len(s.buffers.metricsPool) {
		state := s.buffers.metricsPool[idx]
		*state = ValueMetrics{}
		s.buffers.metricsDepth++
		return state
	}
	state := &ValueMetrics{}
	s.buffers.metricsPool = append(s.buffers.metricsPool, state)
	s.buffers.metricsDepth++
	return state
}

func (s *Session) releaseMetricsState() {
	if s == nil {
		return
	}
	if s.buffers.metricsDepth > 0 {
		s.buffers.metricsDepth--
	}
}
