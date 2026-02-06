package validator

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
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
	decoded, err := types.ParseHexBinary(string(normalized))
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.length = len(decoded)
		metrics.lengthSet = true
	}
	canon := []byte(strings.ToUpper(fmt.Sprintf("%x", decoded)))
	if needKey {
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 0, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canon, nil
}

func validateHexBinaryNoCanonical(normalized []byte) error {
	if _, err := types.ParseHexBinary(string(normalized)); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}

func (s *Session) canonicalizeBase64Binary(normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
	decoded, err := types.ParseBase64Binary(string(normalized))
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if metrics != nil {
		metrics.length = len(decoded)
		metrics.lengthSet = true
	}
	canon := []byte(semantics.EncodeBase64(decoded))
	if needKey {
		key := valuekey.BinaryKeyBytes(s.keyTmp[:0], 1, decoded)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKBinary, key, false)
	}
	return canon, nil
}

func validateBase64BinaryNoCanonical(normalized []byte) error {
	if _, err := types.ParseBase64Binary(string(normalized)); err != nil {
		return valueErrorMsg(valueErrInvalid, err.Error())
	}
	return nil
}
