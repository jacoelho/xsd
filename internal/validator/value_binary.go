package validator

import (
	"encoding/base64"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuecodec"
)

func (s *Session) canonicalizeAnyURI(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	if err := value.ValidateAnyURI(normalized); err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	canon := normalized
	if needKey {
		key := valuecodec.StringKeyBytes(s.keyTmp[:0], 1, canon)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKString, key, false)
	}
	return canon, nil
}

func validateAnyURINoCanonical(normalized []byte) error {
	if err := value.ValidateAnyURI(normalized); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}

func (s *Session) canonicalizeHexBinary(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	decoded, err := value.ParseHexBinary(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.length = len(decoded)
		metrics.lengthSet = true
	}
	canon := value.UpperHex(s.valueScratch[:0], decoded)
	s.valueScratch = canon
	if needKey {
		key := valuecodec.BinaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canon, nil
}

func validateHexBinaryNoCanonical(normalized []byte) error {
	if _, err := value.ParseHexBinary(normalized); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}

func (s *Session) canonicalizeBase64Binary(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	decoded, err := value.ParseBase64Binary(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.length = len(decoded)
		metrics.lengthSet = true
	}
	canonLen := base64.StdEncoding.EncodedLen(len(decoded))
	canon := s.valueScratch[:0]
	if cap(canon) < canonLen {
		canon = make([]byte, canonLen)
	} else {
		canon = canon[:canonLen]
	}
	base64.StdEncoding.Encode(canon, decoded)
	s.valueScratch = canon
	if needKey {
		key := valuecodec.BinaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canon, nil
}

func validateBase64BinaryNoCanonical(normalized []byte) error {
	if _, err := value.ParseBase64Binary(normalized); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}
