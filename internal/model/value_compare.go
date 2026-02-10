package model

import (
	"math"
	"time"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

// ValuesEqual reports whether two typed values are equal in the value space.
// It follows XSD equality rules, not lexical equality.
func ValuesEqual(left, right TypedValue) bool {
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
		switch r := rightNative.(type) {
		case num.Dec:
			return l.Compare(r) == 0
		case num.Int:
			return l.Compare(r.AsDec()) == 0
		default:
			return false
		}

	case num.Int:
		switch r := rightNative.(type) {
		case num.Int:
			return l.Compare(r) == 0
		case num.Dec:
			return l.CompareDec(r) == 0
		default:
			return false
		}

	case time.Time:
		r, ok := rightNative.(time.Time)
		if !ok {
			return false
		}
		leftKind, leftTemporal := temporalKindFromType(left.Type())
		rightKind, rightTemporal := temporalKindFromType(right.Type())
		if leftTemporal || rightTemporal {
			if !leftTemporal || !rightTemporal || leftKind != rightKind {
				return false
			}
			leftVal, lerr := temporal.Parse(leftKind, []byte(left.Lexical()))
			rightVal, rerr := temporal.Parse(rightKind, []byte(right.Lexical()))
			if lerr != nil || rerr != nil {
				return false
			}
			return temporal.Equal(leftVal, rightVal)
		}
		return l.Equal(r)

	case bool:
		r, ok := rightNative.(bool)
		if !ok {
			return false
		}
		return l == r

	case string:
		r, ok := rightNative.(string)
		if !ok {
			return false
		}
		return l == r

	case float32:
		switch r := rightNative.(type) {
		case float32:
			if math.IsNaN(float64(l)) || math.IsNaN(float64(r)) {
				return math.IsNaN(float64(l)) && math.IsNaN(float64(r))
			}
			return l == r
		case float64:
			if math.IsNaN(float64(l)) || math.IsNaN(r) {
				return math.IsNaN(float64(l)) && math.IsNaN(r)
			}
			return float64(l) == r
		default:
			return false
		}

	case float64:
		switch r := rightNative.(type) {
		case float64:
			if math.IsNaN(l) || math.IsNaN(r) {
				return math.IsNaN(l) && math.IsNaN(r)
			}
			return l == r
		case float32:
			if math.IsNaN(l) || math.IsNaN(float64(r)) {
				return math.IsNaN(l) && math.IsNaN(float64(r))
			}
			return l == float64(r)
		default:
			return false
		}

	case QName:
		r, ok := rightNative.(QName)
		return ok && l.Equal(r)

	case XSDDuration:
		switch r := rightNative.(type) {
		case XSDDuration:
			return durationsEqual(l, r, left.Type(), right.Type())
		case ComparableXSDDuration:
			return durationsEqual(l, r.Value, left.Type(), right.Type())
		default:
			return false
		}

	case ComparableXSDDuration:
		switch r := rightNative.(type) {
		case ComparableXSDDuration:
			cmp, err := l.Compare(r)
			return err == nil && cmp == 0
		case XSDDuration:
			return durationsEqual(l.Value, r, left.Type(), right.Type())
		default:
			return false
		}

	case int64:
		r, ok := rightNative.(int64)
		return ok && l == r
	case int32:
		r, ok := rightNative.(int32)
		return ok && l == r
	case int16:
		r, ok := rightNative.(int16)
		return ok && l == r
	case int8:
		r, ok := rightNative.(int8)
		return ok && l == r
	case uint64:
		r, ok := rightNative.(uint64)
		return ok && l == r
	case uint32:
		r, ok := rightNative.(uint32)
		return ok && l == r
	case uint16:
		r, ok := rightNative.(uint16)
		return ok && l == r
	case uint8:
		r, ok := rightNative.(uint8)
		return ok && l == r
	case []byte:
		r, ok := rightNative.([]byte)
		if !ok || len(l) != len(r) {
			return false
		}
		for i := range l {
			if l[i] != r[i] {
				return false
			}
		}
		return true
	}

	return left.Lexical() == right.Lexical()
}

func durationsEqual(left, right XSDDuration, leftType, rightType Type) bool {
	leftComp := ComparableXSDDuration{Value: left, Typ: leftType}
	rightComp := ComparableXSDDuration{Value: right, Typ: rightType}
	cmp, err := leftComp.Compare(rightComp)
	return err == nil && cmp == 0
}

func temporalKindFromType(typ Type) (temporal.Kind, bool) {
	if typ == nil {
		return temporal.KindInvalid, false
	}
	primitive := typ.PrimitiveType()
	if primitive == nil {
		primitive = typ
	}
	return temporal.KindFromPrimitiveName(primitive.Name().Local)
}
