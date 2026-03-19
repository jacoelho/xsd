package valruntime

import (
	"unsafe"

	"github.com/jacoelho/xsd/internal/facetrules"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
	"github.com/jacoelho/xsd/internal/value"
)

// Loader provides caller-owned cached parsers for canonical lexical values.
type Loader struct {
	Decimal func([]byte) (num.Dec, error)
	Integer func([]byte) (num.Int, error)
	Float32 func([]byte) (float32, num.FloatClass, error)
	Float64 func([]byte) (float64, num.FloatClass, error)
}

// Check evaluates one runtime range facet.
func Check(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte, load Loader) error {
	switch kind {
	case runtime.VFloat, runtime.VDouble:
		return checkFloat(op, kind, canonical, bound, load)
	default:
		cmp, err := compareValue(kind, canonical, bound, load)
		if err != nil {
			return err
		}
		return checkRange(op, cmp)
	}
}

func compareValue(kind runtime.ValidatorKind, canonical, bound []byte, load Loader) (int, error) {
	switch kind {
	case runtime.VDecimal:
		if load.Decimal == nil {
			return 0, diag.Invalid("decimal loader missing")
		}
		val, err := load.Decimal(canonical)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseDec(bound)
		if perr != nil {
			return 0, diag.Invalid("invalid decimal")
		}
		return val.Compare(boundVal), nil
	case runtime.VInteger:
		if load.Integer == nil {
			return 0, diag.Invalid("integer loader missing")
		}
		val, err := load.Integer(canonical)
		if err != nil {
			return 0, err
		}
		boundVal, perr := num.ParseInt(bound)
		if perr != nil {
			return 0, diag.Invalid("invalid integer")
		}
		return val.Compare(boundVal), nil
	case runtime.VDuration:
		val, err := value.ParseDuration(unsafe.String(unsafe.SliceData(canonical), len(canonical)))
		if err != nil {
			return 0, diag.Invalid(err.Error())
		}
		boundVal, err := value.ParseDuration(unsafe.String(unsafe.SliceData(bound), len(bound)))
		if err != nil {
			return 0, diag.Invalid(err.Error())
		}
		cmp, err := value.CompareDuration(val, boundVal)
		if err != nil {
			return 0, diag.Facet(err.Error())
		}
		return cmp, nil
	case runtime.VDateTime, runtime.VTime, runtime.VDate, runtime.VGYearMonth, runtime.VGYear, runtime.VGMonthDay, runtime.VGDay, runtime.VGMonth:
		valTemporal, err := ParseTemporal(kind, canonical)
		if err != nil {
			return 0, diag.Invalid(err.Error())
		}
		boundTemporal, err := ParseTemporal(kind, bound)
		if err != nil {
			return 0, diag.Invalid(err.Error())
		}
		cmp, err := value.Compare(valTemporal, boundTemporal)
		if err != nil {
			return 0, diag.Facet(err.Error())
		}
		return cmp, nil
	default:
		return 0, diag.Invalidf("unsupported comparable type %d", kind)
	}
}

func checkFloat(op runtime.FacetOp, kind runtime.ValidatorKind, canonical, bound []byte, load Loader) error {
	var (
		val, boundVal        float64
		valClass, boundClass num.FloatClass
		err                  error
	)

	switch kind {
	case runtime.VFloat:
		if load.Float32 == nil {
			return diag.Invalid("float loader missing")
		}
		var floatVal, floatBound float32
		floatVal, valClass, err = load.Float32(canonical)
		if err != nil {
			return err
		}
		var parseErr *num.ParseError
		floatBound, boundClass, parseErr = num.ParseFloat32(bound)
		if parseErr != nil {
			return diag.Invalid("invalid float")
		}
		val = float64(floatVal)
		boundVal = float64(floatBound)
	case runtime.VDouble:
		if load.Float64 == nil {
			return diag.Invalid("double loader missing")
		}
		var parseErr *num.ParseError
		val, valClass, err = load.Float64(canonical)
		if err != nil {
			return err
		}
		boundVal, boundClass, parseErr = num.ParseFloat(bound, 64)
		if parseErr != nil {
			return diag.Invalid("invalid double")
		}
	default:
		return diag.Invalidf("unsupported float range kind %d", kind)
	}

	if boundClass == num.FloatNaN || valClass == num.FloatNaN {
		return rangeViolation(op)
	}
	cmp, _ := num.CompareFloat(val, valClass, boundVal, boundClass)
	return checkRange(op, cmp)
}

func checkRange(op runtime.FacetOp, cmp int) error {
	matches, ok := facetrules.RuntimeRangeSatisfied(op, cmp)
	if !ok || !matches {
		return rangeViolation(op)
	}
	return nil
}

func rangeViolation(op runtime.FacetOp) error {
	if rule, ok := facetrules.RuntimeRange(op); ok {
		return diag.Facet(rule.Violation)
	}
	return diag.Facet("range violation")
}
