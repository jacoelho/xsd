package validator

import (
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) checkFloatRange(kind runtime.ValidatorKind, op runtime.FacetOp, canonical, bound []byte, metrics *valueMetrics) error {
	var (
		val, boundVal        float64
		valClass, boundClass num.FloatClass
		err                  error
	)

	switch kind {
	case runtime.VFloat:
		var floatVal, floatBound float32
		floatVal, valClass, err = s.float32ForCanonical(canonical, metrics)
		if err != nil {
			return err
		}
		var parseErr *num.ParseError
		floatBound, boundClass, parseErr = num.ParseFloat32(bound)
		if parseErr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid float")
		}
		val = float64(floatVal)
		boundVal = float64(floatBound)
	case runtime.VDouble:
		val, valClass, err = s.float64ForCanonical(canonical, metrics)
		if err != nil {
			return err
		}
		var parseErr *num.ParseError
		boundVal, boundClass, parseErr = num.ParseFloat(bound, 64)
		if parseErr != nil {
			return valueErrorMsg(valueErrInvalid, "invalid double")
		}
	default:
		return valueErrorf(valueErrInvalid, "unsupported float range kind %d", kind)
	}

	if boundClass == num.FloatNaN || valClass == num.FloatNaN {
		return rangeViolation(op)
	}
	cmp, _ := num.CompareFloat(val, valClass, boundVal, boundClass)
	return compareRange(op, cmp)
}
