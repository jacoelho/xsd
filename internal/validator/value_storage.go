package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) setKey(metrics *ValueMetrics, kind runtime.ValueKind, key []byte, store bool) {
	if s == nil || metrics == nil {
		return
	}
	metrics.keyKind = kind
	if store {
		metrics.keyBytes = s.storeKey(key)
	} else {
		metrics.keyBytes = key
	}
	metrics.keySet = true
}

func (s *Session) storeValue(data []byte) []byte {
	if s == nil {
		return nil
	}
	start := len(s.valueBuf)
	s.valueBuf = append(s.valueBuf, data...)
	return s.valueBuf[start:len(s.valueBuf)]
}

func (s *Session) maybeStore(data []byte, store bool) []byte {
	if store {
		return s.storeValue(data)
	}
	return data
}

func (s *Session) storeKey(data []byte) []byte {
	if s == nil {
		return nil
	}
	start := len(s.keyBuf)
	s.keyBuf = append(s.keyBuf, data...)
	return s.keyBuf[start:len(s.keyBuf)]
}

func (s *Session) finalizeValue(canonical []byte, opts valueOptions, metrics *ValueMetrics, metricsInternal bool) []byte {
	if !opts.storeValue {
		return canonical
	}
	canonStored := s.storeValue(canonical)
	if metrics != nil && metrics.keySet && !metricsInternal {
		s.setKey(metrics, metrics.keyKind, metrics.keyBytes, true)
	}
	return canonStored
}
