package types

import (
	"fmt"
	"math"

	"github.com/jacoelho/xsd/internal/num"
)

// ComparableValue is a unified interface for comparable values.
type ComparableValue interface {
	Compare(other ComparableValue) (int, error)
	String() string
	Type() Type
}

// Unwrappable is an interface for types that can unwrap their inner value.
type Unwrappable interface {
	Unwrap() any
}

// ComparableDec wraps num.Dec to implement ComparableValue.
type ComparableDec struct {
	Typ   Type
	Value num.Dec
}

// Compare compares with another ComparableValue.
func (c ComparableDec) Compare(other ComparableValue) (int, error) {
	switch otherVal := other.(type) {
	case ComparableDec:
		return c.Value.Compare(otherVal.Value), nil
	case ComparableInt:
		return c.Value.Compare(otherVal.Value.AsDec()), nil
	default:
		return 0, fmt.Errorf("cannot compare ComparableDec with %T", other)
	}
}

// String returns the canonical string representation.
func (c ComparableDec) String() string {
	return string(c.Value.RenderCanonical(nil))
}

// Type returns the XSD type represented by the value.
func (c ComparableDec) Type() Type {
	return c.Typ
}

// Unwrap returns the inner num.Dec value.
func (c ComparableDec) Unwrap() any {
	return c.Value
}

// ComparableInt wraps num.Int to implement ComparableValue.
type ComparableInt struct {
	Typ   Type
	Value num.Int
}

// Compare compares with another ComparableValue.
func (c ComparableInt) Compare(other ComparableValue) (int, error) {
	switch otherVal := other.(type) {
	case ComparableInt:
		return c.Value.Compare(otherVal.Value), nil
	case ComparableDec:
		return c.Value.CompareDec(otherVal.Value), nil
	default:
		return 0, fmt.Errorf("cannot compare ComparableInt with %T", other)
	}
}

// String returns the canonical string representation.
func (c ComparableInt) String() string {
	return string(c.Value.RenderCanonical(nil))
}

// Type returns the XSD type represented by the value.
func (c ComparableInt) Type() Type {
	return c.Typ
}

// Unwrap returns the inner num.Int value.
func (c ComparableInt) Unwrap() any {
	return c.Value
}

// ComparableFloat64 wraps float64 to implement ComparableValue with NaN/INF handling.
type ComparableFloat64 struct {
	Typ   Type
	Value float64
}

// Compare compares with another ComparableValue.
func (c ComparableFloat64) Compare(other ComparableValue) (int, error) {
	otherFloat, ok := other.(ComparableFloat64)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableFloat64 with %T", other)
	}
	if math.IsNaN(c.Value) || math.IsNaN(otherFloat.Value) {
		return 0, fmt.Errorf("cannot compare NaN values")
	}

	cIsInf := math.IsInf(c.Value, 0)
	otherIsInf := math.IsInf(otherFloat.Value, 0)

	if cIsInf && otherIsInf {
		if math.IsInf(c.Value, 1) && math.IsInf(otherFloat.Value, 1) {
			return 0, nil
		}
		if math.IsInf(c.Value, -1) && math.IsInf(otherFloat.Value, -1) {
			return 0, nil
		}
		if math.IsInf(c.Value, 1) && math.IsInf(otherFloat.Value, -1) {
			return 1, nil
		}
		return -1, nil
	}

	if cIsInf {
		if math.IsInf(c.Value, 1) {
			return 1, nil
		}
		return -1, nil
	}

	if otherIsInf {
		if math.IsInf(otherFloat.Value, 1) {
			return -1, nil
		}
		return 1, nil
	}

	if c.Value < otherFloat.Value {
		return -1, nil
	}
	if c.Value > otherFloat.Value {
		return 1, nil
	}
	return 0, nil
}

// String returns the string representation.
func (c ComparableFloat64) String() string {
	return fmt.Sprintf("%g", c.Value)
}

// Type returns the XSD type represented by the value.
func (c ComparableFloat64) Type() Type {
	return c.Typ
}

// Unwrap returns the inner float64 value.
func (c ComparableFloat64) Unwrap() any {
	return c.Value
}

// ComparableFloat32 wraps float32 to implement ComparableValue with NaN/INF handling.
type ComparableFloat32 struct {
	Typ   Type
	Value float32
}

// Compare compares with another ComparableValue.
func (c ComparableFloat32) Compare(other ComparableValue) (int, error) {
	otherFloat, ok := other.(ComparableFloat32)
	if !ok {
		return 0, fmt.Errorf("cannot compare ComparableFloat32 with %T", other)
	}
	c64 := ComparableFloat64{Value: float64(c.Value), Typ: c.Typ}
	other64 := ComparableFloat64{Value: float64(otherFloat.Value), Typ: otherFloat.Typ}
	return c64.Compare(other64)
}

// String returns the string representation.
func (c ComparableFloat32) String() string {
	return fmt.Sprintf("%g", c.Value)
}

// Type returns the XSD type represented by the value.
func (c ComparableFloat32) Type() Type {
	return c.Typ
}

// Unwrap returns the inner float32 value.
func (c ComparableFloat32) Unwrap() any {
	return c.Value
}
