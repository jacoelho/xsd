package facetvalue

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

var (
	_ model.Facet        = (*rangeFacet)(nil)
	_ model.LexicalFacet = (*rangeFacet)(nil)
)

type rangeFacet struct {
	name    string
	lexical string
	value   model.ComparableValue
	cmpFunc func(int) bool
	errOp   string
}

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

func newMinInclusiveFacet(lexical string, baseType model.Type) (model.Facet, error) {
	compVal, err := newRangeFacetComparable("minInclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &rangeFacet{
		name:    "minInclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp >= 0 },
		errOp:   ">=",
	}, nil
}

func newMaxInclusiveFacet(lexical string, baseType model.Type) (model.Facet, error) {
	compVal, err := newRangeFacetComparable("maxInclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &rangeFacet{
		name:    "maxInclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp <= 0 },
		errOp:   "<=",
	}, nil
}

func newMinExclusiveFacet(lexical string, baseType model.Type) (model.Facet, error) {
	compVal, err := newRangeFacetComparable("minExclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &rangeFacet{
		name:    "minExclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp > 0 },
		errOp:   ">",
	}, nil
}

func newMaxExclusiveFacet(lexical string, baseType model.Type) (model.Facet, error) {
	compVal, err := newRangeFacetComparable("maxExclusive", lexical, baseType)
	if err != nil {
		return nil, err
	}
	return &rangeFacet{
		name:    "maxExclusive",
		lexical: lexical,
		value:   compVal,
		cmpFunc: func(cmp int) bool { return cmp < 0 },
		errOp:   "<",
	}, nil
}

func (r *rangeFacet) Name() string {
	return r.name
}

func (r *rangeFacet) GetLexical() string {
	return r.lexical
}

func (r *rangeFacet) Validate(typed model.TypedValue, baseType model.Type) error {
	if typed == nil {
		return fmt.Errorf("%s: cannot compare nil value", r.name)
	}

	valueType := typed.Type()
	if valueType == nil {
		valueType = baseType
	}

	compVal, err := parseRangeFacetValue(r.name, typed.Lexical(), valueType)
	if err != nil {
		return err
	}

	cmp, err := compVal.Compare(r.value)
	if err != nil {
		if isIndeterminateComparison(err) {
			return fmt.Errorf("value %s must be %s %s", typed.String(), r.errOp, r.lexical)
		}
		return fmt.Errorf("%s: cannot compare values: %w", r.name, err)
	}
	if !r.cmpFunc(cmp) {
		return fmt.Errorf("value %s must be %s %s", typed.String(), r.errOp, r.lexical)
	}
	return nil
}

func isIndeterminateComparison(err error) bool {
	if err == nil {
		return false
	}
	switch err.Error() {
	case "duration comparison indeterminate", "time comparison indeterminate":
		return true
	default:
		return false
	}
}

func newRangeFacetComparable(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	if err := validateRangeFacetApplicability(facetName, baseType); err != nil {
		return nil, err
	}
	return parseRangeFacetValue(facetName, lexical, baseType)
}

func validateRangeFacetApplicability(facetName string, baseType model.Type) error {
	facets := rangeFacetFundamentalFacets(baseType)
	if facets == nil {
		return nil
	}
	if facets.Ordered == model.OrderedTotal || facets.Ordered == model.OrderedPartial {
		return nil
	}
	typeName := "unknown"
	if builtinType, ok := model.AsBuiltinType(baseType); ok {
		typeName = builtinType.Name().Local
	} else if simpleType, ok := model.AsSimpleType(baseType); ok {
		typeName = simpleType.QName.Local
	}
	return fmt.Errorf("%s: only applicable to ordered types, but base type %s is not ordered", facetName, typeName)
}

func rangeFacetFundamentalFacets(baseType model.Type) *model.FundamentalFacets {
	if baseType == nil {
		return nil
	}
	facets := baseType.FundamentalFacets()
	if facets != nil {
		return facets
	}
	primitive := baseType.PrimitiveType()
	if primitive == nil || primitive == baseType {
		return nil
	}
	return primitive.FundamentalFacets()
}

func parseRangeFacetValue(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	if baseType == nil {
		return nil, fmt.Errorf("%s: %w", facetName, model.ErrCannotDeterminePrimitiveType)
	}
	typeName := baseType.Name().Local
	if parsed, handled, err := parseRangeFacetValueForTypeName(facetName, lexical, baseType, typeName); handled {
		return parsed, err
	}
	return parseRangeFacetValueForPrimitive(facetName, lexical, baseType)
}

func parseRangeFacetValueForTypeName(
	facetName, lexical string,
	baseType model.Type,
	typeName string,
) (model.ComparableValue, bool, error) {
	switch typeName {
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		parsed, err := parseRangeInteger(facetName, lexical, baseType)
		return parsed, true, err
	default:
		return nil, false, nil
	}
}

func parseRangeFacetValueForPrimitive(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	primitiveType := baseType.PrimitiveType()
	if primitiveType == nil {
		return nil, fmt.Errorf("%s: %w", facetName, model.ErrCannotDeterminePrimitiveType)
	}

	primitiveName := primitiveType.Name().Local
	integerDerived := isIntegerDerivedType(baseType)

	switch primitiveName {
	case "decimal":
		if integerDerived {
			return parseRangeInteger(facetName, lexical, baseType)
		}
		return parseRangeDecimal(facetName, lexical, baseType)
	case "integer":
		return parseRangeInteger(facetName, lexical, baseType)
	case "dateTime", "date", "time", "gYear", "gYearMonth", "gMonth", "gMonthDay", "gDay":
		return parseRangeTemporal(facetName, lexical, baseType, primitiveName)
	case "float":
		return parseRangeFloat(facetName, lexical, baseType)
	case "double":
		return parseRangeDouble(facetName, lexical, baseType)
	case "duration":
		return parseRangeDuration(facetName, lexical, baseType)
	default:
		return nil, fmt.Errorf("%s: no parser available for primitive type %s", facetName, primitiveName)
	}
}

func isIntegerDerivedType(typ model.Type) bool {
	if typ == nil {
		return false
	}

	typeName := typ.Name().Local
	if integerDerivedTypeNames[typeName] {
		return true
	}

	simpleType, ok := typ.(*model.SimpleType)
	if !ok {
		return false
	}

	current := simpleType.ResolvedBase
	for current != nil {
		currentName := current.Name().Local
		if integerDerivedTypeNames[currentName] {
			return true
		}
		next, ok := current.(*model.SimpleType)
		if !ok {
			break
		}
		current = next.ResolvedBase
	}
	return false
}

func parseRangeInteger(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	intVal, err := value.ParseInteger([]byte(lexical))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return model.ComparableInt{Value: intVal, Typ: baseType}, nil
}

func parseRangeDecimal(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	decimalVal, err := value.ParseDecimal([]byte(lexical))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return model.ComparableDec{Value: decimalVal, Typ: baseType}, nil
}

func parseRangeTemporal(facetName, lexical string, baseType model.Type, primitiveName string) (model.ComparableValue, error) {
	timeVal, err := temporal.ParsePrimitive(primitiveName, []byte(lexical))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return model.ComparableTime{
		Value:        timeVal.Time,
		Typ:          baseType,
		TimezoneKind: model.TimezoneKind(lexical),
		Kind:         timeVal.Kind,
		LeapSecond:   timeVal.LeapSecond,
	}, nil
}

func parseRangeFloat(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	floatVal, err := value.ParseFloat([]byte(lexical))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return model.ComparableFloat32{Value: floatVal, Typ: baseType}, nil
}

func parseRangeDouble(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	doubleVal, err := value.ParseDouble([]byte(lexical))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return model.ComparableFloat64{Value: doubleVal, Typ: baseType}, nil
}

func parseRangeDuration(facetName, lexical string, baseType model.Type) (model.ComparableValue, error) {
	xsdDur, err := durationlex.Parse(lexical)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", facetName, err)
	}
	return model.ComparableXSDDuration{Value: xsdDur, Typ: baseType}, nil
}
