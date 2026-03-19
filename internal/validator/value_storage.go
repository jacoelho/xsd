package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/valruntime"
)

func (s *Session) setKey(metrics *valruntime.State, kind runtime.ValueKind, key []byte, store bool) {
	if s == nil {
		return
	}
	state := metrics.ResultState()
	if state == nil {
		return
	}
	if store {
		key = s.storeKey(key)
	}
	state.SetKey(kind, key)
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

func (s *Session) finalizeValue(canonical []byte, opts valruntime.Options, metrics *valruntime.State, metricsInternal bool) []byte {
	if !opts.StoreValue {
		return canonical
	}
	canonStored := s.storeValue(canonical)
	state := metrics.ResultState()
	if state != nil && state.HasKey() && !metricsInternal {
		kind, key, _ := state.Key()
		s.setKey(metrics, kind, key, true)
	}
	return canonStored
}
