package validator

func (s *Session) acquireValueMetrics() *valueMetrics {
	if s == nil {
		return &valueMetrics{}
	}
	idx := s.metricsDepth
	if idx < len(s.metricsPool) {
		metrics := s.metricsPool[idx]
		*metrics = valueMetrics{}
		s.metricsDepth++
		return metrics
	}
	metrics := &valueMetrics{}
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
