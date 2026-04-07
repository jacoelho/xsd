package validator

import (
	"encoding/base64"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

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
