package validator

import (
	"unsafe"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

func (s *Session) compareValue(kind runtime.ValidatorKind, canonical, bound []byte, metrics *valueMetrics) (int, error) {
	switch kind {
	case runtime.VDecimal:
		val, err := s.decForCanonical(canonical, metrics)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseDec(bound)
		if perr != nil {
			return 0, valueErrorMsg(valueErrInvalid, "invalid decimal")
		}
		return val.Compare(boundVal), nil
	case runtime.VInteger:
		val, err := s.intForCanonical(canonical, metrics)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseInt(bound)
		if perr != nil {
			return 0, valueErrorMsg(valueErrInvalid, "invalid integer")
		}
		return val.Compare(boundVal), nil
	case runtime.VDuration:
		val, err := durationlex.Parse(unsafe.String(unsafe.SliceData(canonical), len(canonical)))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		boundVal, err := durationlex.Parse(unsafe.String(unsafe.SliceData(bound), len(bound)))
		if err != nil {
			return 0, valueErrorMsg(valueErrInvalid, err.Error())
		}
		cmp, err := durationlex.Compare(val, boundVal)
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		valTemporal, err := parseTemporalForKind(kind, canonical)
		if err != nil {
			return 0, err
		}
		boundTemporal, err := parseTemporalForKind(kind, bound)
		if err != nil {
			return 0, err
		}
		cmp, err := temporal.Compare(valTemporal, boundTemporal)
		if err != nil {
			return 0, valueErrorMsg(valueErrFacet, err.Error())
		}
		return cmp, nil
	default:
		return 0, valueErrorf(valueErrInvalid, "unsupported comparable type %d", kind)
	}
}

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

func compareRange(op runtime.FacetOp, cmp int) error {
	switch op {
	case runtime.FMinInclusive:
		if cmp < 0 {
			return rangeViolation(op)
		}
	case runtime.FMaxInclusive:
		if cmp > 0 {
			return rangeViolation(op)
		}
	case runtime.FMinExclusive:
		if cmp <= 0 {
			return rangeViolation(op)
		}
	case runtime.FMaxExclusive:
		if cmp >= 0 {
			return rangeViolation(op)
		}
	default:
		return rangeViolation(op)
	}
	return nil
}

func rangeViolation(op runtime.FacetOp) error {
	switch op {
	case runtime.FMinInclusive:
		return valueErrorf(valueErrFacet, "minInclusive violation")
	case runtime.FMaxInclusive:
		return valueErrorf(valueErrFacet, "maxInclusive violation")
	case runtime.FMinExclusive:
		return valueErrorf(valueErrFacet, "minExclusive violation")
	case runtime.FMaxExclusive:
		return valueErrorf(valueErrFacet, "maxExclusive violation")
	default:
		return valueErrorf(valueErrFacet, "range violation")
	}
}
