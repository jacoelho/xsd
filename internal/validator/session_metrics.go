package validator

func (s *Session) acquireValueMetrics() *ValueMetrics {
	if s == nil {
		return &ValueMetrics{}
	}
	idx := s.metricsDepth
	if idx < len(s.metricsPool) {
		metrics := s.metricsPool[idx]
		*metrics = ValueMetrics{}
		s.metricsDepth++
		return metrics
	}
	metrics := &ValueMetrics{}
	s.metricsPool = append(s.metricsPool, metrics)
	s.metricsDepth++
	return metrics
}

func (s *Session) releaseValueMetrics() {
	if s == nil {
		return
	}
	if s.metricsDepth > 0 {
		s.metricsDepth--
	}
}
