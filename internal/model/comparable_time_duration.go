package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

// ComparableTime wraps time.Time to implement ComparableValue
type ComparableTime struct {
	Value time.Time
	// XSD type this value represents
	Typ          Type
	TimezoneKind value.TimezoneKind
	Kind         temporal.Kind
	LeapSecond   bool
}

var errIndeterminateTimeComparison = errors.New("time comparison indeterminate")

// Compare compares with another ComparableValue (implements ComparableValue)
func (c ComparableTime) Compare(other ComparableValue) (int, error) {
	otherTime, ok := other.(ComparableTime)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableTime with %T", other)
	}

	leftValue := c.semanticValue()
	rightValue := otherTime.semanticValue()
	cmp, err := temporal.Compare(leftValue, rightValue)
	if err != nil {
		if errors.Is(err, temporal.ErrIndeterminateComparison) {
			return 0, errIndeterminateTimeComparison
		}
		return 0, err
	}
	return cmp, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableTime) String() string {
	return temporal.Canonical(c.semanticValue())
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableTime) Type() Type {
	return c.Typ
}

// Unwrap returns the inner time.Time value
func (c ComparableTime) Unwrap() any {
	return c.Value
}

func (c ComparableTime) semanticValue() temporal.Value {
	kind := c.Kind
	if kind == temporal.KindInvalid {
		if inferred, ok := temporalKindFromType(c.Typ); ok {
			kind = inferred
		} else {
			kind = temporal.KindDateTime
		}
	}
	return temporal.Value{
		Kind:         kind,
		Time:         c.Value,
		TimezoneKind: c.TimezoneKind,
		LeapSecond:   c.LeapSecond,
	}
}

// ComparableDuration wraps time.Duration to implement ComparableValue
// Note: Durations are partially ordered, so comparison is limited to pure day/time durations
type ComparableDuration struct {
	Typ   Type
	Value time.Duration
}

// Compare compares with another ComparableValue (implements ComparableValue)
// Both durations must be pure day/time durations (no years/months)
func (c ComparableDuration) Compare(other ComparableValue) (int, error) {
	// try ComparableXSDDuration first (for full XSD duration support)
	if otherXSDDur, ok := other.(ComparableXSDDuration); ok {
		negative := c.Value < 0
		durVal := c.Value
		if negative {
			durVal = -durVal
		}
		hours := int(durVal / time.Hour)
		durVal %= time.Hour
		minutes := int(durVal / time.Minute)
		durVal %= time.Minute
		seconds := num.DecFromScaledInt(num.FromInt64(int64(durVal)), 9)
		thisXSDDur := ComparableXSDDuration{
			Value: durationlex.Duration{
				Negative: negative,
				Years:    0,
				Months:   0,
				Days:     0,
				Hours:    hours,
				Minutes:  minutes,
				Seconds:  seconds,
			},
			Typ: c.Typ,
		}
		return thisXSDDur.Compare(otherXSDDur)
	}
	otherDur, ok := other.(ComparableDuration)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableDuration with %T", other)
	}
	if c.Value < otherDur.Value {
		return -1, nil
	}
	if c.Value > otherDur.Value {
		return 1, nil
	}
	return 0, nil
}

// String returns the string representation (implements ComparableValue)
func (c ComparableDuration) String() string {
	return c.Value.String()
}

// Type returns the XSD type (implements ComparableValue)
func (c ComparableDuration) Type() Type {
	return c.Typ
}

// Unwrap returns the inner time.Duration value
func (c ComparableDuration) Unwrap() any {
	return c.Value
}
