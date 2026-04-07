package model

import (
	"math"
	"time"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/num"
)

// CompareTypedValues reports whether two typed values are equal in the value space.
func CompareTypedValues(left, right TypedValue) bool {
	return valuesEqual(left, right)
}

// valuesEqual reports whether two typed values are equal in the value space.
// It follows XSD equality rules, not lexical equality.
func valuesEqual(left, right TypedValue) bool {
	if left == nil || right == nil {
		return false
	}
	if left == right {
		return true
	}

	leftNative := left.Native()
	rightNative := right.Native()
	if leftNative == nil || rightNative == nil {
		return leftNative == rightNative
	}

	switch l := leftNative.(type) {
	case num.Dec:
		return decimalValuesEqual(l, rightNative)
	case num.Int:
		return integerValuesEqual(l, rightNative)
	case time.Time:
		return timeValuesEqual(left, right, l, rightNative)
	case bool:
		return comparableValuesEqual(l, rightNative)
	case string:
		return comparableValuesEqual(l, rightNative)
	case float32:
		return float32ValuesEqual(l, rightNative)
	case float64:
		return float64ValuesEqual(l, rightNative)
	case QName:
		r, ok := rightNative.(QName)
		return ok && l.Equal(r)
	case value.Duration:
		return rawDurationValuesEqual(l, left.Type(), right, rightNative)
	case ComparableXSDDuration:
		return comparableDurationValuesEqual(l, right, rightNative)
	case int64:
		return comparableValuesEqual(l, rightNative)
	case int32:
		return comparableValuesEqual(l, rightNative)
	case int16:
		return comparableValuesEqual(l, rightNative)
	case int8:
		return comparableValuesEqual(l, rightNative)
	case uint64:
		return comparableValuesEqual(l, rightNative)
	case uint32:
		return comparableValuesEqual(l, rightNative)
	case uint16:
		return comparableValuesEqual(l, rightNative)
	case uint8:
		return comparableValuesEqual(l, rightNative)
	case []byte:
		return bytesValuesEqual(l, rightNative)
	}

	return left.Lexical() == right.Lexical()
}

func decimalValuesEqual(left num.Dec, right any) bool {
	switch r := right.(type) {
	case num.Dec:
		return left.Compare(r) == 0
	case num.Int:
		return left.Compare(r.AsDec()) == 0
	default:
		return false
	}
}

func integerValuesEqual(left num.Int, right any) bool {
	switch r := right.(type) {
	case num.Int:
		return left.Compare(r) == 0
	case num.Dec:
		return left.CompareDec(r) == 0
	default:
		return false
	}
}

func timeValuesEqual(left, right TypedValue, leftTime time.Time, rightNative any) bool {
	rightTime, ok := rightNative.(time.Time)
	if !ok {
		return false
	}
	leftKind, leftTemporal := temporalKindFromType(left.Type())
	rightKind, rightTemporal := temporalKindFromType(right.Type())
	if !leftTemporal && !rightTemporal {
		return leftTime.Equal(rightTime)
	}
	if !leftTemporal || !rightTemporal || leftKind != rightKind {
		return false
	}
	leftVal, lerr := value.Parse(leftKind, []byte(left.Lexical()))
	rightVal, rerr := value.Parse(rightKind, []byte(right.Lexical()))
	if lerr != nil || rerr != nil {
		return false
	}
	return value.Equal(leftVal, rightVal)
}

func float32ValuesEqual(left float32, right any) bool {
	switch r := right.(type) {
	case float32:
		return floatsEqual(float64(left), float64(r))
	case float64:
		return floatsEqual(float64(left), r)
	default:
		return false
	}
}

func float64ValuesEqual(left float64, right any) bool {
	switch r := right.(type) {
	case float64:
		return floatsEqual(left, r)
	case float32:
		return floatsEqual(left, float64(r))
	default:
		return false
	}
}

func floatsEqual(left, right float64) bool {
	if math.IsNaN(left) || math.IsNaN(right) {
		return math.IsNaN(left) && math.IsNaN(right)
	}
	return left == right
}

func rawDurationValuesEqual(left value.Duration, leftType Type, right TypedValue, rightNative any) bool {
	switch r := rightNative.(type) {
	case value.Duration:
		return durationsEqual(left, r, leftType, right.Type())
	case ComparableXSDDuration:
		return durationsEqual(left, r.Value, leftType, right.Type())
	default:
		return false
	}
}

func comparableDurationValuesEqual(left ComparableXSDDuration, right TypedValue, rightNative any) bool {
	switch r := rightNative.(type) {
	case ComparableXSDDuration:
		cmp, err := left.Compare(r)
		return err == nil && cmp == 0
	case value.Duration:
		return durationsEqual(left.Value, r, left.Type(), right.Type())
	default:
		return false
	}
}

func bytesValuesEqual(left []byte, right any) bool {
	rightBytes, ok := right.([]byte)
	if !ok || len(left) != len(rightBytes) {
		return false
	}
	for i := range left {
		if left[i] != rightBytes[i] {
			return false
		}
	}
	return true
}

func comparableValuesEqual[T comparable](left T, right any) bool {
	rightValue, ok := right.(T)
	return ok && left == rightValue
}

func durationsEqual(left, right value.Duration, leftType, rightType Type) bool {
	leftComp := ComparableXSDDuration{Value: left, Typ: leftType}
	rightComp := ComparableXSDDuration{Value: right, Typ: rightType}
	cmp, err := leftComp.Compare(rightComp)
	return err == nil && cmp == 0
}

func temporalKindFromType(typ Type) (value.Kind, bool) {
	if typ == nil {
		return value.KindInvalid, false
	}
	primitive := typ.PrimitiveType()
	if primitive == nil {
		primitive = typ
	}
	return value.KindFromPrimitiveName(primitive.Name().Local)
}
