package validator

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
)

type valueErrorKind uint8

const (
	valueErrInvalid valueErrorKind = iota
	valueErrFacet
)

type valueError struct {
	msg  string
	kind valueErrorKind
}

func (e valueError) Error() string { return e.msg }

func valueErrorf(kind valueErrorKind, format string, args ...any) error {
	return valueError{kind: kind, msg: fmt.Sprintf(format, args...)}
}

func valueErrorMsg(kind valueErrorKind, msg string) error {
	return valueError{kind: kind, msg: msg}
}

func valueErrorKindOf(err error) (valueErrorKind, bool) {
	if err == nil {
		return 0, false
	}
	var ve valueError
	if errors.As(err, &ve) {
		return ve.kind, true
	}
	return 0, false
}

type valueMetrics struct {
	intVal          num.Int
	keyBytes        []byte
	decVal          num.Dec
	fractionDigits  int
	totalDigits     int
	listCount       int
	length          int
	float64Val      float64
	float32Val      float32
	actualTypeID    runtime.TypeID
	actualValidator runtime.ValidatorID
	patternChecked  bool
	enumChecked     bool
	keySet          bool
	decSet          bool
	intSet          bool
	float32Set      bool
	float64Set      bool
	listSet         bool
	digitsSet       bool
	lengthSet       bool
	float32Class    num.FloatClass
	keyKind         runtime.ValueKind
	float64Class    num.FloatClass
}

type valueOptions struct {
	applyWhitespace  bool
	trackIDs         bool
	requireCanonical bool
	storeValue       bool
	needKey          bool
}

func (s *Session) setKey(metrics *valueMetrics, kind runtime.ValueKind, key []byte, store bool) {
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

func (s *Session) finalizeValue(canonical []byte, opts valueOptions, metrics *valueMetrics, metricsInternal bool) []byte {
	if !opts.storeValue {
		return canonical
	}
	canonStored := s.storeValue(canonical)
	if metrics != nil && metrics.keySet && !metricsInternal {
		s.setKey(metrics, metrics.keyKind, metrics.keyBytes, true)
	}
	return canonStored
}
