package validator

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
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

func (s *Session) validateValueInternalOptions(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	return s.validateValueCore(id, lexical, resolver, opts, nil)
}

func (s *Session) validateValueInternalWithMetrics(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions) ([]byte, valueMetrics, error) {
	var metrics valueMetrics
	canon, err := s.validateValueCore(id, lexical, resolver, opts, &metrics)
	return canon, metrics, err
}

func (s *Session) validateValueCore(id runtime.ValidatorID, lexical []byte, resolver value.NSResolver, opts valueOptions, metrics *valueMetrics) ([]byte, error) {
	if s == nil || s.rt == nil {
		return nil, valueErrorf(valueErrInvalid, "runtime schema missing")
	}
	if id == 0 {
		return nil, valueErrorf(valueErrInvalid, "validator missing")
	}
	if int(id) >= len(s.rt.Validators.Meta) {
		return nil, valueErrorf(valueErrInvalid, "validator %d out of range", id)
	}
	meta := s.rt.Validators.Meta[id]
	var localMetrics valueMetrics
	metricsInternal := false
	ensureMetrics := func() {
		if metrics == nil {
			metrics = &localMetrics
			metricsInternal = true
		}
	}
	if (meta.Kind == runtime.VHexBinary || meta.Kind == runtime.VBase64Binary) && s.hasLengthFacet(meta) {
		ensureMetrics()
	}
	if opts.trackIDs && meta.Kind == runtime.VUnion {
		ensureMetrics()
	}
	normalized := lexical
	popNorm := false
	if opts.applyWhitespace {
		if meta.Kind == runtime.VList || meta.Kind == runtime.VUnion {
			buf := s.pushNormBuf(len(lexical))
			popNorm = true
			normalized = value.NormalizeWhitespace(valueWhitespaceMode(meta.WhiteSpace), lexical, buf)
		} else {
			normalized = value.NormalizeWhitespace(valueWhitespaceMode(meta.WhiteSpace), lexical, s.normBuf)
		}
	}
	if popNorm {
		defer s.popNormBuf()
	}
	needsCanonical := opts.requireCanonical || meta.Facets.Len != 0 || meta.Kind == runtime.VUnion || meta.Kind == runtime.VQName || meta.Kind == runtime.VNotation
	if opts.storeValue || opts.needKey {
		needsCanonical = true
	}
	needEnumKey := meta.Flags&runtime.ValidatorHasEnum != 0
	if needEnumKey {
		ensureMetrics()
	}
	needKey := opts.needKey || opts.storeValue || needEnumKey
	if !needsCanonical {
		canon, err := s.validateValueNoCanonical(meta, normalized, resolver, opts)
		if err != nil {
			return nil, err
		}
		if opts.trackIDs {
			if err := s.trackValidatedIDs(id, canon, resolver, metrics); err != nil {
				return nil, err
			}
		}
		return canon, nil
	}
	canon, err := s.canonicalizeValueCore(meta, normalized, lexical, resolver, opts, needKey, metrics)
	if err != nil {
		return nil, err
	}
	if err := s.applyFacets(meta, normalized, canon, metrics); err != nil {
		return nil, err
	}
	canon = s.finalizeValue(canon, opts, metrics, metricsInternal)
	if opts.trackIDs {
		if err := s.trackValidatedIDs(id, canon, resolver, metrics); err != nil {
			return nil, err
		}
	}
	return canon, nil
}

func valueWhitespaceMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WS_Replace:
		return value.WhitespaceReplace
	case runtime.WS_Collapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}

func (s *Session) canonicalizeValueCore(meta runtime.ValidatorMeta, normalized, lexical []byte, resolver value.NSResolver, opts valueOptions, needKey bool, metrics *valueMetrics) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString, runtime.VBoolean, runtime.VDecimal, runtime.VInteger, runtime.VFloat, runtime.VDouble, runtime.VDuration:
		return s.canonicalizeAtomic(meta, normalized, needKey, metrics)
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		return s.canonicalizeTemporal(meta.Kind, normalized, needKey, metrics)
	case runtime.VAnyURI:
		return s.canonicalizeAnyURI(normalized, needKey, metrics)
	case runtime.VQName, runtime.VNotation:
		return s.canonicalizeQName(meta, normalized, resolver, needKey, metrics)
	case runtime.VHexBinary:
		return s.canonicalizeHexBinary(normalized, needKey, metrics)
	case runtime.VBase64Binary:
		return s.canonicalizeBase64Binary(normalized, needKey, metrics)
	case runtime.VList:
		return s.canonicalizeList(meta, normalized, resolver, opts, needKey, metrics)
	case runtime.VUnion:
		return s.canonicalizeUnion(meta, normalized, lexical, resolver, opts, needKey, metrics)
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", meta.Kind)
	}
}

func (s *Session) validateValueNoCanonical(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, opts valueOptions) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString, runtime.VBoolean, runtime.VDecimal, runtime.VInteger, runtime.VFloat, runtime.VDouble, runtime.VDuration:
		if err := s.validateAtomicNoCanonical(meta, normalized); err != nil {
			return nil, err
		}
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		if err := validateTemporalNoCanonical(meta.Kind, normalized); err != nil {
			return nil, err
		}
	case runtime.VAnyURI:
		if err := validateAnyURINoCanonical(normalized); err != nil {
			return nil, err
		}
	case runtime.VHexBinary:
		if err := validateHexBinaryNoCanonical(normalized); err != nil {
			return nil, err
		}
	case runtime.VBase64Binary:
		if err := validateBase64BinaryNoCanonical(normalized); err != nil {
			return nil, err
		}
	case runtime.VList:
		if err := s.validateListNoCanonical(meta, normalized, resolver, opts); err != nil {
			return nil, err
		}
	default:
		return nil, valueErrorf(valueErrInvalid, "unsupported validator kind %d", meta.Kind)
	}
	return s.maybeStore(normalized, opts.storeValue), nil
}
