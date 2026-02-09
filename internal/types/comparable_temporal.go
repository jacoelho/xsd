package types

import (
	"errors"
	"fmt"
	"time"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
)

// XSDDuration represents a full XSD duration with all components.
type XSDDuration = durationlex.Duration

// ComparableXSDDuration wraps XSDDuration to implement ComparableValue
// This supports full XSD durations including years and months
type ComparableXSDDuration struct {
	Typ   Type
	Value XSDDuration
}

var errIndeterminateDurationComparison = errors.New("duration comparison indeterminate")

func secondsToDuration(sec num.Dec) (time.Duration, error) {
	if sec.Sign < 0 {
		return 0, fmt.Errorf("second value cannot be negative")
	}
	scaled, err := num.DecToScaledIntExact(sec, 9)
	if err != nil {
		return 0, err
	}
	const maxDuration = time.Duration(^uint64(0) >> 1)
	maxSeconds := num.FromInt64(int64(maxDuration))
	if scaled.Compare(maxSeconds) > 0 {
		return 0, fmt.Errorf("second value too large")
	}
	val, ok := int64FromDigits(scaled.Digits)
	if !ok {
		return 0, fmt.Errorf("second value too large")
	}
	if scaled.Sign < 0 {
		val = -val
	}
	return time.Duration(val), nil
}

func int64FromDigits(digits []byte) (int64, bool) {
	if len(digits) == 0 {
		return 0, true
	}
	var n int64
	for _, d := range digits {
		if n > (int64(^uint64(0)>>1)-int64(d-'0'))/10 {
			return 0, false
		}
		n = n*10 + int64(d-'0')
	}
	return n, true
}

// Compare orders durations using the XSD 1.0 order relation for duration.
func (c ComparableXSDDuration) Compare(other ComparableValue) (int, error) {
	otherDur, ok := other.(ComparableXSDDuration)
	if !ok {
		if otherCompDur, ok := other.(ComparableDuration); ok {
			otherDur = ComparableXSDDuration{Value: durationToXSD(otherCompDur.Value), Typ: otherCompDur.Typ}
		} else {
			return 0, fmt.Errorf("cannot compare ComparableXSDDuration with %T", other)
		}
	}

	cmp, err := durationlex.Compare(c.Value, otherDur.Value)
	if err != nil {
		if errors.Is(err, durationlex.ErrIndeterminateComparison) {
			return 0, errIndeterminateDurationComparison
		}
		return 0, err
	}
	return cmp, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableXSDDuration) String() string {
	return durationlex.CanonicalString(c.Value)
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableXSDDuration) Type() Type {
	return c.Typ
}

// Unwrap returns the inner XSDDuration value
func (c ComparableXSDDuration) Unwrap() any {
	return c.Value
}
