package model

import (
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

// durationToXSD converts a time.Duration to XSDDuration.
func durationToXSD(d time.Duration) XSDDuration {
	negative := d < 0
	if negative {
		d = -d
	}
	hours := int(d / time.Hour)
	d %= time.Hour
	minutes := int(d / time.Minute)
	d %= time.Minute
	seconds := num.DecFromScaledInt(num.FromInt64(int64(d)), 9)
	return XSDDuration{
		Negative: negative,
		Years:    0,
		Months:   0,
		Days:     0,
		Hours:    hours,
		Minutes:  minutes,
		Seconds:  seconds,
	}
}

// integerDerivedTypeNames is a lookup table for types derived from xs:integer.
// Package-level var avoids repeated allocation.
var integerDerivedTypeNames = map[string]bool{
	"integer":            true,
	"long":               true,
	"int":                true,
	"short":              true,
	"byte":               true,
	"unsignedLong":       true,
	"unsignedInt":        true,
	"unsignedShort":      true,
	"unsignedByte":       true,
	"nonNegativeInteger": true,
	"positiveInteger":    true,
	"negativeInteger":    true,
	"nonPositiveInteger": true,
}

// isIntegerDerivedType checks if t derives from xs:integer by walking the derivation chain.
func isIntegerDerivedType(t Type) bool {
	if t == nil {
		return false
	}

	typeName := t.Name().Local

	// check if the type name itself is integer-derived
	if integerDerivedTypeNames[typeName] {
		return true
	}

	// for SimpleType, walk the derivation chain
	if st, ok := t.(*SimpleType); ok {
		current := st.ResolvedBase
		for current != nil {
			// use Name() interface method instead of type assertions
			currentName := current.Name().Local
			if integerDerivedTypeNames[currentName] {
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
	case float64:
		return ComparableFloat64{Value: v, Typ: typ}, nil
	case float32:
		return ComparableFloat32{Value: v, Typ: typ}, nil
	case string:
		return parseStringToComparableValue(value, v, typ)
	}

	// all conversion attempts failed
	xsdTypeName := getXSDTypeName(value)
	return nil, fmt.Errorf("value type %s cannot be compared with facet value", xsdTypeName)
}

// parseStringToComparableValue parses a string value according to the TypedValue's type
// and converts it to the appropriate ComparableValue.
func parseStringToComparableValue(value TypedValue, lexical string, typ Type) (ComparableValue, error) {
	if typ == nil {
		typ = value.Type()
	}
	if typ == nil {
		return nil, fmt.Errorf("cannot parse string: value has no type")
	}

	typeName := typ.Name().Local

	// check if the actual type is integer (even though primitive is decimal)
	if typeName == "integer" {
		intVal, err := ParseInteger(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse integer: %w", err)
		}
		return ComparableInt{Value: intVal, Typ: typ}, nil
	}

	var primitiveType Type
	switch t := typ.(type) {
	case *SimpleType:
		primitiveType = t.PrimitiveType()
	case *BuiltinType:
		primitiveType = t.PrimitiveType()
	default:
		return nil, fmt.Errorf("cannot parse string: unsupported type %T", typ)
	}

	if primitiveType == nil {
		return nil, fmt.Errorf("cannot parse string: cannot determine primitive type")
	}

	primitiveName := primitiveType.Name().Local

	// check if type is integer-derived
	isIntegerDerived := isIntegerDerivedType(typ)

	switch primitiveName {
	case "decimal":
		// if type is integer-derived, parse as integer
		if isIntegerDerived {
			intVal, err := ParseInteger(lexical)
			if err != nil {
				return nil, fmt.Errorf("cannot parse integer: %w", err)
			}
			return ComparableInt{Value: intVal, Typ: typ}, nil
		}
		rat, err := ParseDecimal(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse decimal: %w", err)
		}
		return ComparableDec{Value: rat, Typ: typ}, nil

	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		timeVal, err := temporal.ParsePrimitive(primitiveName, []byte(lexical))
		if err != nil {
			return nil, fmt.Errorf("cannot parse date/time: %w", err)
		}
		return ComparableTime{
			Value:        timeVal.Time,
			Typ:          typ,
			TimezoneKind: TimezoneKind(lexical),
			Kind:         timeVal.Kind,
			LeapSecond:   timeVal.LeapSecond,
		}, nil

	case "float":
		floatVal, err := ParseFloat(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse float: %w", err)
		}
		return ComparableFloat32{Value: floatVal, Typ: typ}, nil

	case "double":
		doubleVal, err := ParseDouble(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse double: %w", err)
		}
		return ComparableFloat64{Value: doubleVal, Typ: typ}, nil

	case "duration":
		xsdDur, err := durationlex.Parse(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse duration: %w", err)
		}
		return ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil

	default:
		return nil, fmt.Errorf("cannot parse string: unsupported primitive type %s for Comparable conversion", primitiveName)
	}
}
