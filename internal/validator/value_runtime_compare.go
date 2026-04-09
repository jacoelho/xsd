package validator

import (
	"unsafe"

	xsderrors "github.com/jacoelho/xsd/errors"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

// checkRuntimeRange evaluates one runtime range facet.
func checkRuntimeRange(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte, cache *ValueCache) error {
	switch kind {
	case runtime.VFloat, runtime.VDouble:
		return checkFloat(op, kind, canonical, bound, cache)
	default:
		cmp, err := compareValue(kind, canonical, bound, cache)
		if err != nil {
			return err
		}
		return checkRange(op, cmp)
	}
}

func compareValue(kind runtime.ValidatorKind, canonical, bound []byte, cache *ValueCache) (int, error) {
	switch kind {
	case runtime.VDecimal:
		val, err := cache.Decimal(canonical)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseDec(bound)
		if perr != nil {
			return 0, xsderrors.Invalid("invalid decimal")
		}
		return val.Compare(boundVal), nil
	case runtime.VInteger:
		val, err := cache.Integer(canonical)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseInt(bound)
		if perr != nil {
			return 0, xsderrors.Invalid("invalid integer")
		}
		return val.Compare(boundVal), nil
	case runtime.VDuration:
		val, err := value.ParseDuration(unsafe.String(unsafe.SliceData(canonical), len(canonical)))
		if err != nil {
			return 0, xsderrors.Invalid(err.Error())
		}
		boundVal, err := value.ParseDuration(unsafe.String(unsafe.SliceData(bound), len(bound)))
		if err != nil {
			return 0, xsderrors.Invalid(err.Error())
		}
		cmp, err := value.CompareDuration(val, boundVal)
		if err != nil {
			return 0, xsderrors.Facet(err.Error())
		}
		return cmp, nil
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		valTemporal, err := parseTemporal(kind, canonical)
		if err != nil {
			return 0, xsderrors.Invalid(err.Error())
		}
		boundTemporal, err := parseTemporal(kind, bound)
		if err != nil {
			return 0, xsderrors.Invalid(err.Error())
		}
		cmp, err := value.Compare(valTemporal, boundTemporal)
		if err != nil {
			return 0, xsderrors.Facet(err.Error())
		}
		return cmp, nil
	default:
		return 0, xsderrors.Invalidf("unsupported comparable type %d", kind)
	}
}

func checkFloat(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte, cache *ValueCache) error {
	var (
		val, boundVal        float64
		valClass, boundClass num.FloatClass
		err                  error
	)

	switch kind {
	case runtime.VFloat:
		var floatVal, floatBound float32
		floatVal, valClass, err = cache.Float32(canonical)
		if err != nil {
			return err
		}
		var parseErr *num.ParseError
		floatBound, boundClass, parseErr = num.ParseFloat32(bound)
		if parseErr != nil {
			return xsderrors.Invalid("invalid float")
		}
		val = float64(floatVal)
		boundVal = float64(floatBound)
	case runtime.VDouble:
		var parseErr *num.ParseError
		val, valClass, err = cache.Float64(canonical)
		if err != nil {
			return err
		}
		boundVal, boundClass, parseErr = num.ParseFloat(bound, 64)
		if parseErr != nil {
			return xsderrors.Invalid("invalid double")
		}
	default:
		return xsderrors.Invalidf("unsupported float range kind %d", kind)
	}

	if boundClass == num.FloatNaN || valClass == num.FloatNaN {
		return rangeViolation(op)
	}
	cmp, _ := num.CompareFloat(val, valClass, boundVal, boundClass)
	return checkRange(op, cmp)
}

func checkRange(op runtime.FacetOp, cmp int) error {
	matches, ok := RuntimeRangeSatisfied(op, cmp)
	if !ok || !matches {
		return rangeViolation(op)
	}
	return nil
}

func rangeViolation(op runtime.FacetOp) error {
	if rule, ok := RuntimeRange(op); ok {
		return xsderrors.Facet(rule.Violation)
	}
	return xsderrors.Facet("range violation")
}
