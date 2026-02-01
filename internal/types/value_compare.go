package types

import (
	"math"
	"time"

	"github.com/jacoelho/xsd/internal/num"
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
		if isTemporalValueType(left.Type()) {
			leftHasTZ := HasTimezone(left.Lexical())
			rightHasTZ := HasTimezone(right.Lexical())
			if leftHasTZ != rightHasTZ {
				return false
			}
			// If both have timezones, compare UTC times (Z and +00:00 are equivalent)
			if leftHasTZ {
				if isTimeValueType(left.Type()) {
					leftSec, leftNanos := timeOfDayUTC(l)
					rightSec, rightNanos := timeOfDayUTC(r)
					return leftSec == rightSec && leftNanos == rightNanos
				}
				return l.UTC().Equal(r.UTC())
			}
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

func isTemporalValueType(typ Type) bool {
	if typ == nil {
		return false
	}
	primitive := typ.PrimitiveType()
	if primitive == nil {
		primitive = typ
	}
	switch primitive.Name().Local {
	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		return true
	default:
		return false
	}
}

func isTimeValueType(typ Type) bool {
	if typ == nil {
		return false
	}
	primitive := typ.PrimitiveType()
	if primitive == nil {
		primitive = typ
	}
	return primitive.Name().Local == "time"
}

func timeOfDayUTC(t time.Time) (int, int) {
	utc := t.UTC()
	seconds := utc.Hour()*3600 + utc.Minute()*60 + utc.Second()
	return seconds, utc.Nanosecond()
}
