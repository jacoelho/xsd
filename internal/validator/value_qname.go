package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/valuekey"
)

func (s *Session) canonicalizeQName(meta runtime.ValidatorMeta, normalized []byte, resolver value.NSResolver, needKey bool, metrics *valueMetrics) ([]byte, error) {
	canon, err := value.CanonicalQName(normalized, resolver, nil)
	if err != nil {
		return nil, valueErrorMsg(valueErrInvalid, err.Error())
	}
	canonStored := canon
	if needKey {
		tag := byte(0)
		if meta.Kind == runtime.VNotation {
			tag = 1
		}
		key := valuekey.QNameKeyCanonical(s.keyTmp[:0], tag, canonStored)
		if len(key) == 0 {
			return nil, valueErrorf(valueErrInvalid, "invalid QName key")
		}
		s.keyTmp = key
		s.setKey(metrics, runtime.VKQName, key, false)
	}
	return canonStored, nil
}
