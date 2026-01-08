package facets

import (
	"fmt"
	"math/big"
	"time"

	lexicalparser "github.com/jacoelho/xsd/internal/parser/lexical"
	"github.com/jacoelho/xsd/internal/types"
)

// getXSDTypeName returns a user-friendly XSD type name for error messages
func getXSDTypeName(value types.TypedValue) string {
	if value == nil {
		return "unknown"
	}
	typ := value.Type()
	if typ == nil {
		return "unknown"
	}
	return typ.Name().Local
}

// parseTemporalValue parses a lexical value according to its primitive type name.
func parseTemporalValue(primitiveName, lexical string) (time.Time, error) {
	switch primitiveName {
	case "dateTime":
		return lexicalparser.ParseDateTime(lexical)
	case "date":
		return types.ParseDate(lexical)
	case "time":
		return types.ParseTime(lexical)
	case "gYear":
		return types.ParseGYear(lexical)
	case "gYearMonth":
		return types.ParseGYearMonth(lexical)
	case "gMonth":
		return types.ParseGMonth(lexical)
	case "gMonthDay":
		return types.ParseGMonthDay(lexical)
	case "gDay":
		return types.ParseGDay(lexical)
	default:
		return time.Time{}, fmt.Errorf("unsupported date/time type: %s", primitiveName)
	}
}

// durationToXSD converts a time.Duration to XSDDuration.
func durationToXSD(d time.Duration) types.XSDDuration {
	negative := d < 0
	if negative {
		d = -d
	}
	hours := int(d / time.Hour)
	d %= time.Hour
	minutes := int(d / time.Minute)
	d %= time.Minute
	seconds := float64(d) / float64(time.Second)
	return types.XSDDuration{
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
func isIntegerDerivedType(t types.Type) bool {
	if t == nil {
		return false
	}

	typeName := t.Name().Local

	// check if the type name itself is integer-derived
	if integerDerivedTypeNames[typeName] {
		return true
	}

	// for SimpleType, walk the derivation chain
	if st, ok := t.(*types.SimpleType); ok {
		current := st.ResolvedBase
		for current != nil {
			// use Name() interface method instead of type assertions
			currentName := current.Name().Local
			if integerDerivedTypeNames[currentName] {
				return true
			}
			// continue walking the chain if it's a SimpleType
			if currentST, ok := current.(*types.SimpleType); ok {
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
func extractComparableValue(value types.TypedValue, baseType types.Type) (types.ComparableValue, error) {
	native := value.Native()
	typ := value.Type()
	if typ == nil {
		typ = baseType
	}

	// try to convert native to ComparableValue directly
	if compVal, ok := native.(types.ComparableValue); ok {
		return compVal, nil
	}

	switch v := native.(type) {
	case *big.Rat:
		return types.ComparableBigRat{Value: v, Typ: typ}, nil
	case *big.Int:
		return types.ComparableBigInt{Value: v, Typ: typ}, nil
	case time.Time:
		return types.ComparableTime{Value: v, Typ: typ}, nil
	case time.Duration:
		xsdDur := durationToXSD(v)
		return types.ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil
	case float64:
		return types.ComparableFloat64{Value: v, Typ: typ}, nil
	case float32:
		return types.ComparableFloat32{Value: v, Typ: typ}, nil
	case string:
		return parseStringToComparableValue(value, v, typ)
	}

	// try to extract using ValueAs helper for known types
	if rat, err := types.ValueAs[*big.Rat](value); err == nil {
		return types.ComparableBigRat{Value: rat, Typ: typ}, nil
	}
	if intVal, err := types.ValueAs[*big.Int](value); err == nil {
		return types.ComparableBigInt{Value: intVal, Typ: typ}, nil
	}
	if timeVal, err := types.ValueAs[time.Time](value); err == nil {
		return types.ComparableTime{Value: timeVal, Typ: typ}, nil
	}
	if float64Val, err := types.ValueAs[float64](value); err == nil {
		return types.ComparableFloat64{Value: float64Val, Typ: typ}, nil
	}
	if float32Val, err := types.ValueAs[float32](value); err == nil {
		return types.ComparableFloat32{Value: float32Val, Typ: typ}, nil
	}
	if durVal, err := types.ValueAs[time.Duration](value); err == nil {
		xsdDur := durationToXSD(durVal)
		return types.ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil
	}

	// all conversion attempts failed
	xsdTypeName := getXSDTypeName(value)
	return nil, fmt.Errorf("value type %s cannot be compared with facet value", xsdTypeName)
}

// parseStringToComparableValue parses a string value according to the TypedValue's type
// and converts it to the appropriate ComparableValue.
func parseStringToComparableValue(value types.TypedValue, lexical string, typ types.Type) (types.ComparableValue, error) {
	if typ == nil {
		typ = value.Type()
	}
	if typ == nil {
		return nil, fmt.Errorf("cannot parse string: value has no type")
	}

	typeName := typ.Name().Local

	// check if the actual type is integer (even though primitive is decimal)
	if typeName == "integer" {
		intVal, err := lexicalparser.ParseInteger(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse integer: %w", err)
		}
		return types.ComparableBigInt{Value: intVal, Typ: typ}, nil
	}

	var primitiveType types.Type
	if st, ok := typ.(*types.SimpleType); ok {
		primitiveType = st.PrimitiveType()
	} else if bt, ok := typ.(*types.BuiltinType); ok {
		primitiveType = bt.PrimitiveType()
	} else {
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
			intVal, err := lexicalparser.ParseInteger(lexical)
			if err != nil {
				return nil, fmt.Errorf("cannot parse integer: %w", err)
			}
			return types.ComparableBigInt{Value: intVal, Typ: typ}, nil
		}
		rat, err := lexicalparser.ParseDecimal(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse decimal: %w", err)
		}
		return types.ComparableBigRat{Value: rat, Typ: typ}, nil

	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		timeVal, err := parseTemporalValue(primitiveName, lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse date/time: %w", err)
		}
		return types.ComparableTime{Value: timeVal, Typ: typ}, nil

	case "float":
		floatVal, err := lexicalparser.ParseFloat(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse float: %w", err)
		}
		return types.ComparableFloat32{Value: floatVal, Typ: typ}, nil

	case "double":
		doubleVal, err := lexicalparser.ParseDouble(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse double: %w", err)
		}
		return types.ComparableFloat64{Value: doubleVal, Typ: typ}, nil

	case "duration":
		xsdDur, err := types.ParseXSDDuration(lexical)
		if err != nil {
			return nil, fmt.Errorf("cannot parse duration: %w", err)
		}
		return types.ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil

	default:
		return nil, fmt.Errorf("cannot parse string: unsupported primitive type %s for Comparable conversion", primitiveName)
	}
}

// RangeFacet is a unified implementation for all range facets.
type RangeFacet struct {
	// Facet name (minInclusive, maxInclusive, etc.)
	name string
	// Keep lexical for schema/error messages
	lexical string
	// Comparable value
	value types.ComparableValue
	// Comparison function: returns true if validation passes
	cmpFunc func(cmp int) bool
	// Error operator string (">=", "<=", ">", "<")
	errOp string
}

// Name returns the facet name
func (r *RangeFacet) Name() string {
	return r.name
}

// GetLexical returns the lexical value (implements LexicalFacet)
func (r *RangeFacet) GetLexical() string {
	return r.lexical
}

// Validate validates a TypedValue using ComparableValue comparison
func (r *RangeFacet) Validate(value types.TypedValue, baseType types.Type) error {
	compVal, err := extractComparableValue(value, baseType)
	if err != nil {
		return fmt.Errorf("%s: %w", r.name, err)
	}

	// compare using ComparableValue interface
	cmp, err := compVal.Compare(r.value)
	if err != nil {
		return fmt.Errorf("%s: cannot compare values: %w", r.name, err)
	}

	if !r.cmpFunc(cmp) {
		return fmt.Errorf("value %s must be %s %s", value.String(), r.errOp, r.lexical)
	}

	return nil
}
