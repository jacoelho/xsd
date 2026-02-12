package runtime

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/num"
)

// IntegerSignRule describes the sign constraint for an integer kind.
type IntegerSignRule uint8

const (
	// IntegerSignAny is an exported constant.
	IntegerSignAny IntegerSignRule = iota
	// IntegerSignNonNegative is an exported constant.
	IntegerSignNonNegative
	// IntegerSignPositive is an exported constant.
	IntegerSignPositive
	// IntegerSignNonPositive is an exported constant.
	IntegerSignNonPositive
	// IntegerSignNegative is an exported constant.
	IntegerSignNegative
)

// IntegerKindSpec describes the constraints for an integer kind.
type IntegerKindSpec struct {
	Label    string
	Min      num.Int
	Max      num.Int
	SignRule IntegerSignRule
	HasRange bool
}

// IntegerKindSpecFor returns the constraint spec for a given integer kind.
func IntegerKindSpecFor(kind IntegerKind) (IntegerKindSpec, bool) {
	switch kind {
	case IntegerAny:
		return IntegerKindSpec{SignRule: IntegerSignAny}, true
	case IntegerLong:
		return IntegerKindSpec{Label: "long", SignRule: IntegerSignAny, Min: num.MinInt64, Max: num.MaxInt64, HasRange: true}, true
	case IntegerInt:
		return IntegerKindSpec{Label: "int", SignRule: IntegerSignAny, Min: num.MinInt32, Max: num.MaxInt32, HasRange: true}, true
	case IntegerShort:
		return IntegerKindSpec{Label: "short", SignRule: IntegerSignAny, Min: num.MinInt16, Max: num.MaxInt16, HasRange: true}, true
	case IntegerByte:
		return IntegerKindSpec{Label: "byte", SignRule: IntegerSignAny, Min: num.MinInt8, Max: num.MaxInt8, HasRange: true}, true
	case IntegerNonNegative:
		return IntegerKindSpec{Label: "nonNegativeInteger", SignRule: IntegerSignNonNegative}, true
	case IntegerPositive:
		return IntegerKindSpec{Label: "positiveInteger", SignRule: IntegerSignPositive}, true
	case IntegerNonPositive:
		return IntegerKindSpec{Label: "nonPositiveInteger", SignRule: IntegerSignNonPositive}, true
	case IntegerNegative:
		return IntegerKindSpec{Label: "negativeInteger", SignRule: IntegerSignNegative}, true
	case IntegerUnsignedLong:
		return IntegerKindSpec{Label: "unsignedLong", SignRule: IntegerSignNonNegative, Min: num.IntZero, Max: num.MaxUint64, HasRange: true}, true
	case IntegerUnsignedInt:
		return IntegerKindSpec{Label: "unsignedInt", SignRule: IntegerSignNonNegative, Min: num.IntZero, Max: num.MaxUint32, HasRange: true}, true
	case IntegerUnsignedShort:
		return IntegerKindSpec{Label: "unsignedShort", SignRule: IntegerSignNonNegative, Min: num.IntZero, Max: num.MaxUint16, HasRange: true}, true
	case IntegerUnsignedByte:
		return IntegerKindSpec{Label: "unsignedByte", SignRule: IntegerSignNonNegative, Min: num.IntZero, Max: num.MaxUint8, HasRange: true}, true
	default:
		return IntegerKindSpec{}, false
	}
}

// ValidateIntegerKind validates an integer value against the kind constraints.
func ValidateIntegerKind(kind IntegerKind, v num.Int) error {
	spec, ok := IntegerKindSpecFor(kind)
	if !ok {
		return nil
	}
	switch spec.SignRule {
	case IntegerSignNonNegative:
		if v.Sign < 0 {
			return fmt.Errorf("%s must be >= 0", spec.Label)
		}
	case IntegerSignPositive:
		if v.Sign <= 0 {
			return fmt.Errorf("%s must be >= 1", spec.Label)
		}
	case IntegerSignNonPositive:
		if v.Sign > 0 {
			return fmt.Errorf("%s must be <= 0", spec.Label)
		}
	case IntegerSignNegative:
		if v.Sign >= 0 {
			return fmt.Errorf("%s must be <= -1", spec.Label)
		}
	}
	if spec.HasRange {
		return validateIntRange(v, spec.Min, spec.Max, spec.Label)
	}
	return nil
}

func validateIntRange(v, minValue, maxValue num.Int, label string) error {
	if v.Compare(minValue) < 0 || v.Compare(maxValue) > 0 {
		if label == "" {
			return fmt.Errorf("integer out of range")
		}
		return fmt.Errorf("%s out of range", label)
	}
	return nil
}
