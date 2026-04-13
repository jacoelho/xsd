package validator

import (
	"encoding/base64"
	"fmt"
	"slices"
	"unsafe"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

func (s *Session) canonicalizeAtomic(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	switch meta.Kind {
	case runtime.VString:
		return s.canonicalizeAtomicString(meta, normalized, needKey, metrics)
	case runtime.VBoolean:
		return s.canonicalizeAtomicBoolean(normalized, needKey, metrics)
	case runtime.VDecimal:
		return s.canonicalizeAtomicDecimal(normalized, needKey, metrics)
	case runtime.VInteger:
		return s.canonicalizeAtomicInteger(meta, normalized, needKey, metrics)
	case runtime.VFloat:
		return s.canonicalizeAtomicFloat(normalized, needKey, metrics)
	case runtime.VDouble:
		return s.canonicalizeAtomicDouble(normalized, needKey, metrics)
	case runtime.VDuration:
		return s.canonicalizeAtomicDuration(normalized, needKey, metrics)
	default:
		return nil, xsderrors.Invalidf("unsupported atomic kind %d", meta.Kind)
	}
}

func (s *Session) canonicalizeAtomicString(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	kind, ok := s.stringKind(meta)
	if !ok {
		return nil, xsderrors.Invalid("string validator out of range")
	}
	canonical, err := value.CanonicalizeString(normalized, func(data []byte) error {
		return runtime.ValidateStringKind(kind, data)
	})
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKString, runtime.StringKeyBytes(s.keyTmp[:0], 0, canonical))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicBoolean(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, canonical, err := value.CanonicalizeBoolean(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if needKey && s != nil {
		key := s.keyTmp[:0]
		key = append(key, 0)
		if v {
			key[0] = 1
		}
		s.setAtomicKey(metrics, runtime.VKBool, key)
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicDecimal(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	buf1 := []byte(nil)
	buf2 := []byte(nil)
	if s != nil {
		buf1 = s.Scratch.Buf1
		buf2 = s.Scratch.Buf2[:0]
	}

	dec, parsedBuf, parseErr := num.ParseDecInto(normalized, buf1)
	if parseErr != nil {
		return nil, xsderrors.Invalid("invalid decimal")
	}
	if s != nil {
		s.Scratch.Buf1 = parsedBuf
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetDecimal(dec)
	}

	canonical := dec.RenderCanonical(buf2)
	if s != nil {
		s.Scratch.Buf2 = canonical
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKDecimal, num.EncodeDecKey(s.keyTmp[:0], dec))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicInteger(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	kind, ok := s.integerKind(meta)
	if !ok {
		return nil, xsderrors.Invalid("integer validator out of range")
	}

	intVal, parseErr := num.ParseInt(normalized)
	if parseErr != nil {
		return nil, xsderrors.Invalid("invalid integer")
	}
	if err := runtime.ValidateIntegerKind(kind, intVal); err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetInteger(intVal)
	}

	buf2 := []byte(nil)
	if s != nil {
		buf2 = s.Scratch.Buf2[:0]
	}
	canonical := intVal.RenderCanonical(buf2)
	if s != nil {
		s.Scratch.Buf2 = canonical
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKDecimal, num.EncodeDecKey(s.keyTmp[:0], intVal.AsDec()))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicFloat(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, class, canonical, err := value.CanonicalizeFloat32(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetFloat32(v, class)
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKFloat32, runtime.Float32Key(s.keyTmp[:0], v, class))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicDouble(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, class, canonical, err := value.CanonicalizeFloat64(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetFloat64(v, class)
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKFloat64, runtime.Float64Key(s.keyTmp[:0], v, class))
	}
	return canonical, nil
}

func (s *Session) canonicalizeAtomicDuration(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	dur, canonical, err := value.CanonicalizeDuration(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if needKey && s != nil {
		s.setAtomicKey(metrics, runtime.VKDuration, runtime.DurationKeyBytes(s.keyTmp[:0], dur))
	}
	return canonical, nil
}

func (s *Session) setAtomicKey(metrics *ValueMetrics, kind runtime.ValueKind, key []byte) {
	if s == nil {
		return
	}
	s.keyTmp = key
	s.setKey(metrics, kind, key, false)
}

func (s *Session) validateAtomicNoCanonical(meta runtime.ValidatorMeta, normalized []byte) error {
	stringKind := runtime.StringAny
	if meta.Kind == runtime.VString {
		kind, ok := s.stringKind(meta)
		if !ok {
			return xsderrors.Invalid("string validator out of range")
		}
		stringKind = kind
	}
	integerKind := runtime.IntegerAny
	if meta.Kind == runtime.VInteger {
		kind, ok := s.integerKind(meta)
		if !ok {
			return xsderrors.Invalid("integer validator out of range")
		}
		integerKind = kind
	}
	if err := validateAtomicLexical(meta.Kind, stringKind, integerKind, normalized); err != nil {
		return xsderrors.Invalid(err.Error())
	}
	return nil
}

func validateAtomicLexical(kind runtime.ValidatorKind, stringKind runtime.StringKind, integerKind runtime.IntegerKind, normalized []byte) error {
	switch kind {
	case runtime.VString:
		return runtime.ValidateStringKind(stringKind, normalized)
	case runtime.VBoolean:
		_, err := value.ParseBoolean(normalized)
		return err
	case runtime.VDecimal:
		if _, err := num.ParseDec(normalized); err != nil {
			return fmt.Errorf("invalid decimal")
		}
		return nil
	case runtime.VInteger:
		intVal, err := num.ParseInt(normalized)
		if err != nil {
			return fmt.Errorf("invalid integer")
		}
		return runtime.ValidateIntegerKind(integerKind, intVal)
	case runtime.VFloat:
		if err := num.ValidateFloatLexical(normalized); err != nil {
			return fmt.Errorf("invalid float")
		}
		return nil
	case runtime.VDouble:
		if err := num.ValidateFloatLexical(normalized); err != nil {
			return fmt.Errorf("invalid double")
		}
		return nil
	case runtime.VDuration:
		_, err := value.ParseDuration(unsafe.String(unsafe.SliceData(normalized), len(normalized)))
		return err
	default:
		return fmt.Errorf("unsupported atomic kind %d", kind)
	}
}

func (s *Session) canonicalizeHexBinary(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	decoded, err := value.ParseHexBinary(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetLength(len(decoded))
	}

	canonical := value.UpperHex(s.valueScratch[:0], decoded)
	s.valueScratch = canonical
	if needKey {
		key := runtime.BinaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canonical, nil
}

func (s *Session) canonicalizeBase64Binary(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	decoded, err := value.ParseBase64Binary(normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if cache := metrics.cache(); cache != nil {
		cache.SetLength(len(decoded))
	}

	canonicalLen := base64.StdEncoding.EncodedLen(len(decoded))
	canonical := s.valueScratch[:0]
	if cap(canonical) < canonicalLen {
		canonical = make([]byte, canonicalLen)
	} else {
		canonical = canonical[:canonicalLen]
	}
	base64.StdEncoding.Encode(canonical, decoded)
	s.valueScratch = canonical
	if needKey {
		key := runtime.BinaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canonical, nil
}

func (s *Session) canonicalizeTemporal(kind runtime.ValidatorKind, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	spec, ok := runtime.TemporalSpecForValidatorKind(kind)
	if !ok {
		return nil, xsderrors.Invalidf("unsupported temporal kind %d", kind)
	}
	tv, err := value.Parse(spec.Kind, normalized)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	canonical := []byte(value.Canonical(tv))
	if needKey && s != nil {
		key := runtime.TemporalKeyBytes(s.keyTmp[:0], spec.KeyTag, tv.Time, tv.TimezoneKind, tv.LeapSecond)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKDateTime, key, false)
	}
	return canonical, nil
}

func (s *Session) canonicalizeQName(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	canonicalBuf := []byte(nil)
	if s != nil {
		canonicalBuf = s.valueScratch[:0]
	}
	canon, err := value.CanonicalQName(normalized, resolver, canonicalBuf)
	if err != nil {
		return nil, xsderrors.Invalid(err.Error())
	}
	if s != nil {
		s.valueScratch = canon
	}

	if meta.Kind == runtime.VNotation && !s.notationDeclared(canon) {
		return nil, xsderrors.Invalid("notation not declared")
	}

	if needKey {
		tag := byte(0)
		if meta.Kind == runtime.VNotation {
			tag = 1
		}
		keyBuf := []byte(nil)
		if s != nil {
			keyBuf = s.keyTmp[:0]
		}
		key := runtime.QNameKeyCanonical(keyBuf, tag, canon)
		if len(key) == 0 {
			return nil, xsderrors.Invalid("invalid QName key")
		}
		if s != nil {
			s.keyTmp = key
			s.setKey(metrics, runtime.VKQName, key, false)
		}
	}
	return canon, nil
}

func (s *Session) notationDeclared(canon []byte) bool {
	if s == nil || s.rt == nil || len(s.rt.Notations) == 0 {
		return false
	}
	ns, local, err := splitCanonicalQName(canon)
	if err != nil {
		return false
	}
	nsID := s.namespaceID(ns)
	if nsID == 0 {
		return false
	}
	sym := s.rt.Symbols.Lookup(nsID, local)
	if sym == 0 {
		return false
	}
	_, ok := slices.BinarySearch(s.rt.Notations, sym)
	return ok
}
