package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/valuecodec"
	"github.com/jacoelho/xsd/internal/valuesemantics"
)

func (s *Session) canonicalizeAtomicString(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	kind, ok := s.stringKind(meta)
	if !ok {
		return nil, valueErrorf(valueErrInvalid, "string validator out of range")
	}
	if err := runtime.ValidateStringKind(kind, normalized); err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	canon := normalized
	if needKey {
		key := valuecodec.StringKeyBytes(s.keyTmp[:0], 0, canon)
		s.keyTmp = key
		s.setKey(metrics, runtime.VKString, key, false)
	}
	return canon, nil
}

func (s *Session) canonicalizeAtomicBoolean(normalized []byte, needKey bool, metrics *ValueMetrics) ([]byte, error) {
	v, canon, err := valuesemantics.CanonicalizeBoolean(normalized)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	if needKey {
		key := byte(0)
		if v {
			key = 1
		}
		s.setKey(metrics, runtime.VKBool, []byte{key}, false)
	}
	return canon, nil
}
