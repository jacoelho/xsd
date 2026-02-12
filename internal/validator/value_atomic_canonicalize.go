package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) canonicalizeAtomic(meta runtime.ValidatorMeta, normalized []byte, needKey bool, metrics *valueMetrics) ([]byte, error) {
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
		return nil, valueErrorf(valueErrInvalid, "unsupported atomic kind %d", meta.Kind)
	}
}
