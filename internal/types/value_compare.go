package types

import (
	"math"
	"math/big"
	"time"
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
	case *big.Rat:
		switch r := rightNative.(type) {
		case *big.Rat:
			return l.Cmp(r) == 0
		case *big.Int:
			return l.Cmp(new(big.Rat).SetInt(r)) == 0
		default:
			return false
		}

	case *big.Int:
		switch r := rightNative.(type) {
		case *big.Int:
			return l.Cmp(r) == 0
		case *big.Rat:
			return new(big.Rat).SetInt(l).Cmp(r) == 0
		default:
			return false
		}

	case time.Time:
		r, ok := rightNative.(time.Time)
		if !ok {
			return false
		}
		if isTemporalValueType(left.Type()) {
			if HasTimezone(left.Lexical()) != HasTimezone(right.Lexical()) {
				return false
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
				return false
			}
			return l == r
		case float64:
			if math.IsNaN(float64(l)) || math.IsNaN(r) {
				return false
			}
			return float64(l) == r
		default:
			return false
		}

	case float64:
		switch r := rightNative.(type) {
		case float64:
			if math.IsNaN(l) || math.IsNaN(r) {
				return false
			}
			return l == r
		case float32:
			if math.IsNaN(l) || math.IsNaN(float64(r)) {
				return false
			}
			return l == float64(r)
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
