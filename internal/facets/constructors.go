package facets

import (
	"errors"
	"fmt"

	lexicalparser "github.com/jacoelho/xsd/internal/parser/lexical"
	"github.com/jacoelho/xsd/internal/types"
)

// ErrCannotDeterminePrimitiveType is returned when the primitive type cannot be
// determined during parsing (e.g., for user-defined types whose full hierarchy
// isn't resolved yet). This is expected during parsing and will be validated
// during the schema validation phase.
var ErrCannotDeterminePrimitiveType = errors.New("cannot determine primitive type")

// NewMinInclusive creates a minInclusive facet based on the base type
// It automatically determines the correct ComparableValue type and parses the value
func NewMinInclusive(lexical string, baseType types.Type) (Facet, error) {
	compVal, err := newRangeFacet("minInclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "minInclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}, nil
}

// NewMaxInclusive creates a maxInclusive facet based on the base type
func NewMaxInclusive(lexical string, baseType types.Type) (Facet, error) {
	compVal, err := newRangeFacet("maxInclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "maxInclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}, nil
}

// NewMinExclusive creates a minExclusive facet based on the base type
func NewMinExclusive(lexical string, baseType types.Type) (Facet, error) {
	compVal, err := newRangeFacet("minExclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "minExclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp > 0 },
		errOp:   ">",
	}, nil
}

// NewMaxExclusive creates a maxExclusive facet based on the base type
func NewMaxExclusive(lexical string, baseType types.Type) (Facet, error) {
	compVal, err := newRangeFacet("maxExclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &RangeFacet{
		name:    "maxExclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}, nil
}

// newRangeFacet is a helper that parses the lexical value and creates the appropriate ComparableValue
func newRangeFacet(facetName, lexical string, baseType types.Type) (types.ComparableValue, error) {
	// Check if base type is ordered (range facets apply to OrderedTotal or OrderedPartial)
	// Per XSD 1.0 spec: range facets apply to types with ordered != none
	var facets *types.FundamentalFacets

	// Try to get fundamental facets from the base type
	// Per spec: "for derived types, the ultimate primitive base"
	// Use PrimitiveType() uniformly - it handles both ResolvedBase and Restriction.Base cases
	if baseType != nil {
		facets = baseType.FundamentalFacets()

		// If base type doesn't have facets yet, get them from primitive type
		if facets == nil {
			primitive := baseType.PrimitiveType()
			if primitive != nil && primitive != baseType {
				facets = primitive.FundamentalFacets()
			}
		}
	}

	// If we still can't determine facets, be lenient during parsing
	// Schema validation will catch issues after all types are resolved
	if facets == nil {
		// During parsing, if we can't determine facets, allow it
		// Full validation will happen during schema validation phase
		// This handles cases where user-defined types chain (e.g., s2 -> s1 -> s -> int)
	} else if facets.Ordered != types.OrderedTotal && facets.Ordered != types.OrderedPartial {
		typeName := "unknown"
		if bt, ok := baseType.(*types.BuiltinType); ok {
			typeName = bt.Name().Local
		} else if st, ok := baseType.(*types.SimpleType); ok {
			typeName = st.QName.Local
		}
		return nil, fmt.Errorf("%s: only applicable to ordered types, but base type %s is not ordered", facetName, typeName)
	}

	typeName := baseType.Name().Local
	// Parse based on actual type name first (for built-in types)
	switch typeName {
	case "integer":
		intVal, err := lexicalparser.ParseInteger(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableBigInt{Value: intVal, Typ: baseType}, nil

	case "long":
		// For long, parse as integer (big.Int) since long values can be large
		intVal, err := lexicalparser.ParseInteger(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableBigInt{Value: intVal, Typ: baseType}, nil

	case "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		// For these, parse as integer (big.Int) since they're all integer types
		intVal, err := lexicalparser.ParseInteger(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableBigInt{Value: intVal, Typ: baseType}, nil
	}

	// If not a built-in integer type, use primitive type
	// Per spec: "for derived types, the ultimate primitive base"
	primitiveType := baseType.PrimitiveType()
	if primitiveType == nil {
		// Can't determine primitive type - this can happen during parsing
		// for user-defined type chains (e.g., s2 -> s1 -> s -> int)
		// Schema validation will catch this later
		return nil, fmt.Errorf("%s: %w", facetName, ErrCannotDeterminePrimitiveType)
	}

	primitiveName := primitiveType.Name().Local

	// Check if type is integer-derived (integer and its derived types)
	// This is needed because integer's primitive is decimal, but integer facets should use ComparableBigInt
	isIntegerDerived := isIntegerDerivedType(baseType)

	switch primitiveName {
	case "decimal":
		// If type is integer-derived, parse as integer (ComparableBigInt) instead of decimal (ComparableBigRat)
		if isIntegerDerived {
			intVal, err := lexicalparser.ParseInteger(lexical)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", facetName, err)
			}
			return types.ComparableBigInt{Value: intVal, Typ: baseType}, nil
		}
		rat, err := lexicalparser.ParseDecimal(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableBigRat{Value: rat, Typ: baseType}, nil

	case "integer":
		intVal, err := lexicalparser.ParseInteger(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableBigInt{Value: intVal, Typ: baseType}, nil

	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		timeVal, err := parseTemporalValue(primitiveName, lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableTime{Value: timeVal, Typ: baseType}, nil

	case "float":
		floatVal, err := lexicalparser.ParseFloat(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableFloat32{Value: floatVal, Typ: baseType}, nil

	case "double":
		doubleVal, err := lexicalparser.ParseDouble(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableFloat64{Value: doubleVal, Typ: baseType}, nil

	case "duration":
		// Parse duration as full XSD duration (supports years/months)
		xsdDur, err := types.ParseXSDDuration(lexical)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", facetName, err)
		}
		return types.ComparableXSDDuration{Value: xsdDur, Typ: baseType}, nil

	default:
		// For types without Comparable wrappers, return error
		// This will fall back to string-based validation
		return nil, fmt.Errorf("%s: no parser available for primitive type %s", facetName, primitiveName)
	}
}
