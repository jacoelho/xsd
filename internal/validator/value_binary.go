package validator

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) canonicalizeAnyURI(normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	if err := value.ValidateAnyURI(normalized); err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	canon := normalized
	if needKey {
		key := valuekey.StringKeyBytes(s.keyTmp[:0], 1, canon)
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

func (s *Session) canonicalizeHexBinary(normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	decoded, err := types.ParseHexBinaryBytes(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.length = len(decoded)
		metrics.lengthSet = true
	}
	canon := upperHex(s.valueBuf[:0], decoded)
	s.valueBuf = canon
	if needKey {
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canon, nil
}

func validateHexBinaryNoCanonical(normalized []byte) error {
	if _, err := types.ParseHexBinaryBytes(normalized); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}

func (s *Session) canonicalizeBase64Binary(normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	decoded, err := types.ParseBase64BinaryBytes(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.length = len(decoded)
		metrics.lengthSet = true
	}
	canonLen := base64.StdEncoding.EncodedLen(len(decoded))
	canon := s.valueBuf[:0]
	if cap(canon) < canonLen {
		canon = make([]byte, canonLen)
	} else {
		canon = canon[:canonLen]
	}
	base64.StdEncoding.Encode(canon, decoded)
	s.valueBuf = canon
	if needKey {
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canon, nil
}

func validateBase64BinaryNoCanonical(normalized []byte) error {
	if _, err := types.ParseBase64BinaryBytes(normalized); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}

func upperHex(dst, src []byte) []byte {
	size := hex.EncodedLen(len(src))
	if cap(dst) < size {
		dst = make([]byte, size)
	} else {
		dst = dst[:size]
	}
	hex.Encode(dst, src)
	for i := range dst {
		if dst[i] >= 'a' && dst[i] <= 'f' {
			dst[i] -= 'a' - 'A'
		}
	}
	return dst
}
