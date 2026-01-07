package validator

import (
	"math/big"
	"time"

	"github.com/jacoelho/xsd/internal/types"
)

func fixedValueMatches(actualValue, fixedValue string, typ types.Type) bool {
	normalizedValue := types.NormalizeWhiteSpace(actualValue, typ)
	normalizedFixed := types.NormalizeWhiteSpace(fixedValue, typ)
	return normalizedValue == normalizedFixed
}

// compareTypedValues compares two TypedValues for equality in the value space.
// Per XSD spec section 4.2.1, equality is determined by the value space, not lexical representation.
// For example, "1.0" and "1.00" are equal decimals, and "true" and "1" are equal booleans.
func compareTypedValues(left, right types.TypedValue) bool {
	if left == nil || right == nil {
		return false
	}

	leftNative := left.Native()
	rightNative := right.Native()

	if leftNative == nil || rightNative == nil {
		return leftNative == rightNative
	}

	switch l := leftNative.(type) {
	case *big.Rat:
		r, ok := rightNative.(*big.Rat)
		if !ok {
			return false
		}
		return l.Cmp(r) == 0

	case *big.Int:
		r, ok := rightNative.(*big.Int)
		if !ok {
			return false
		}
		return l.Cmp(r) == 0

	case time.Time:
		r, ok := rightNative.(time.Time)
		if !ok {
			return false
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
		r, ok := rightNative.(float32)
		if !ok {
			return false
		}
		return l == r

	case float64:
		r, ok := rightNative.(float64)
		if !ok {
			return false
		}
		return l == r

	case int64:
		r, ok := rightNative.(int64)
		if !ok {
			return false
		}
		return l == r

	case int32:
		r, ok := rightNative.(int32)
		if !ok {
			return false
		}
		return l == r

	case int16:
		r, ok := rightNative.(int16)
		if !ok {
			return false
		}
		return l == r

	case int8:
		r, ok := rightNative.(int8)
		if !ok {
			return false
		}
		return l == r

	case uint64:
		r, ok := rightNative.(uint64)
		if !ok {
			return false
		}
		return l == r

	case uint32:
		r, ok := rightNative.(uint32)
		if !ok {
			return false
		}
		return l == r

	case uint16:
		r, ok := rightNative.(uint16)
		if !ok {
			return false
		}
		return l == r

	case uint8:
		r, ok := rightNative.(uint8)
		if !ok {
			return false
		}
		return l == r

	case []byte:
		r, ok := rightNative.([]byte)
		if !ok {
			return false
		}
		if len(l) != len(r) {
			return false
		}
		for i := range l {
			if l[i] != r[i] {
				return false
			}
		}
		return true

	default:
		// for unknown types, fall back to string comparison of lexical forms
		// this handles any custom TypedValue implementations
		return left.Lexical() == right.Lexical()
	}
}
