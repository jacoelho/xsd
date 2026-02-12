package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

func getXSDTypeName(value TypedValue) string {
	if value == nil {
		return "unknown"
	}
	typ := value.Type()
	if typ == nil {
		return "unknown"
	}
	return typ.Name().Local
}

// durationToXSD converts a time.Duration to durationlex.Duration.
func durationToXSD(d time.Duration) durationlex.Duration {
	negative := d < 0
	if negative {
		d = -d
	}
	hours := int(d / time.Hour)
	d %= time.Hour
	minutes := int(d / time.Minute)
	d %= time.Minute
	seconds := num.DecFromScaledInt(num.FromInt64(int64(d)), 9)
	return durationlex.Duration{
		Negative: negative,
		Years:    0,
		Months:   0,
		Days:     0,
		Hours:    hours,
		Minutes:  minutes,
		Seconds:  seconds,
	}
}

// isIntegerDerivedType checks if t derives from xs:integer by walking the derivation chain.
func isIntegerDerivedType(t Type) bool {
	if t == nil {
		return false
	}

	typeName := t.Name().Local

	// check if the type name itself is integer-derived
	if IsIntegerTypeName(typeName) {
		return true
	}

	// for SimpleType, walk the derivation chain
	if st, ok := t.(*SimpleType); ok {
		current := st.ResolvedBase
		for current != nil {
			// use Name() interface method instead of type assertions
			currentName := current.Name().Local
			if IsIntegerTypeName(currentName) {
				return true
			}
			// continue walking the chain if it's a SimpleType
			if currentST, ok := current.(*SimpleType); ok {
				current = currentST.ResolvedBase
			} else {
				// BuiltinType or other type - stop here
				break
			}
		}
	}

	return false
}

// extractComparableValue extracts a ComparableValue from a TypedValue.
// This is the shared logic used by all range facet validators.
func extractComparableValue(value TypedValue, baseType Type) (ComparableValue, error) {
	if value == nil {
		return nil, fmt.Errorf("cannot compare nil value")
	}

	native := value.Native()
	typ := value.Type()
	if typ == nil {
		typ = baseType
	}

	// try to convert native to ComparableValue directly
	if compVal, ok := native.(ComparableValue); ok {
		return compVal, nil
	}
	if unwrappable, ok := native.(Unwrappable); ok {
		native = unwrappable.Unwrap()
	}

	switch v := native.(type) {
	case num.Dec:
		return ComparableDec{Value: v, Typ: typ}, nil
	case num.Int:
		return ComparableInt{Value: v, Typ: typ}, nil
	case time.Time:
		tzKind := TimezoneKind(value.Lexical())
		kind, ok := temporalKindFromType(typ)
		if ok {
			tval, err := temporal.Parse(kind, []byte(value.Lexical()))
			if err == nil {
				return ComparableTime{
					Value:        tval.Time,
					Typ:          typ,
					TimezoneKind: tzKind,
					Kind:         tval.Kind,
					LeapSecond:   tval.LeapSecond,
				}, nil
			}
		}
		return ComparableTime{
			Value:        v,
			Typ:          typ,
			TimezoneKind: tzKind,
			Kind:         temporal.KindDateTime,
		}, nil
	case time.Duration:
		xsdDur := durationToXSD(v)
		return ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil
	case durationlex.Duration:
		return ComparableXSDDuration{Value: v, Typ: typ}, nil
	case float64:
		return ComparableFloat64{Value: v, Typ: typ}, nil
	case float32:
		return ComparableFloat32{Value: v, Typ: typ}, nil
	case string:
		return parseStringToComparableValue(v, typ)
	}

	// all conversion attempts failed
	xsdTypeName := getXSDTypeName(value)
	return nil, fmt.Errorf("value type %s cannot be compared with facet value", xsdTypeName)
}

// parseStringToComparableValue parses a string value according to the TypedValue's type
// and converts it to the appropriate ComparableValue.
func parseStringToComparableValue(lexical string, typ Type) (ComparableValue, error) {
	if typ == nil {
		return nil, fmt.Errorf("cannot parse string: value has no type")
	}

	comparableValue, err := parseLexicalToComparable(lexical, typ)
	if err == nil {
		return comparableValue, nil
	}
	if errors.Is(err, ErrCannotDeterminePrimitiveType) {
		return nil, fmt.Errorf("cannot parse string: cannot determine primitive type")
	}
	var unsupported unsupportedComparablePrimitiveError
	if errors.As(err, &unsupported) {
		return nil, fmt.Errorf(
			"cannot parse string: unsupported primitive type %s for Comparable conversion",
			unsupported.primitive,
		)
	}
	return nil, fmt.Errorf("cannot parse %s: %w", comparableParseErrorCategory(typ), err)
}
