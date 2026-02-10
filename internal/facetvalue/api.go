package facetvalue

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/durationlex"
	internalcore "github.com/jacoelho/xsd/internal/facetvalue/internalcore"
	model "github.com/jacoelho/xsd/internal/model"
)

func Apply(value model.TypedValue, facets []model.Facet, baseType model.Type) error {
	facetsAny := make([]any, len(facets))
	for i, facet := range facets {
		facetsAny[i] = facet
	}
	return internalcore.ApplyFacets(value, facetsAny, baseType, internalcore.ApplyFacetOps{
		ValidateFacet: func(facet any, value any, baseType any) error {
			f, ok := facet.(model.Facet)
			if !ok {
				return fmt.Errorf("invalid facet %T", facet)
			}
			tv, ok := value.(model.TypedValue)
			if !ok {
				return fmt.Errorf("invalid typed value %T", value)
			}
			bt, ok := baseType.(model.Type)
			if !ok {
				return fmt.Errorf("invalid base type %T", baseType)
			}
			return f.Validate(tv, bt)
		},
	})
}

func Validate(value string, baseType model.Type, facets []model.Facet, context map[string]string) error {
	facetsAny := make([]any, len(facets))
	for i, facet := range facets {
		facetsAny[i] = facet
	}

	return internalcore.ValidateValueAgainstFacets(value, baseType, facetsAny, context, internalcore.ValidateFacetOps{
		FacetName: func(facet any) string {
			f, ok := facet.(model.Facet)
			if !ok {
				return fmt.Sprintf("invalid facet %T", facet)
			}
			return f.Name()
		},
		ShouldSkipLengthFacet: func(baseType any, facet any) bool {
			bt, ok := baseType.(model.Type)
			if !ok {
				return false
			}
			f, ok := facet.(model.Facet)
			if !ok {
				return false
			}
			if !IsLengthFacet(f) {
				return false
			}
			if isListTypeForFacetValidation(bt) {
				return false
			}
			return IsQNameOrNotationType(bt)
		},
		IsQNameOrNotationType: func(baseType any) bool {
			bt, ok := baseType.(model.Type)
			return ok && IsQNameOrNotationType(bt)
		},
		IsListTypeForFacetValidation: func(baseType any) bool {
			bt, ok := baseType.(model.Type)
			return ok && isListTypeForFacetValidation(bt)
		},
		ValidateQNameEnumerationLexical: func(facet any, value string, baseType any, context map[string]string) (bool, error) {
			enumFacet, ok := facet.(*model.Enumeration)
			if !ok {
				return false, nil
			}
			bt, ok := baseType.(model.Type)
			if !ok {
				return true, fmt.Errorf("invalid base type %T", baseType)
			}
			return true, enumFacet.ValidateLexicalQName(value, bt, context)
		},
		ValidateLexicalFacet: func(facet any, value string, baseType any) (bool, error) {
			lexicalFacet, ok := facet.(model.LexicalValidator)
			if !ok {
				return false, nil
			}
			bt, ok := baseType.(model.Type)
			if !ok {
				return true, fmt.Errorf("invalid base type %T", baseType)
			}
			return true, lexicalFacet.ValidateLexical(value, bt)
		},
		TypedValueForFacet: func(value string, baseType any) any {
			bt, ok := baseType.(model.Type)
			if !ok {
				return &model.StringTypedValue{Value: value}
			}
			return TypedValueForFacet(value, bt)
		},
		ValidateFacet: func(facet any, value any, baseType any) error {
			f, ok := facet.(model.Facet)
			if !ok {
				return fmt.Errorf("invalid facet %T", facet)
			}
			tv, ok := value.(model.TypedValue)
			if !ok {
				return fmt.Errorf("invalid typed value %T", value)
			}
			bt, ok := baseType.(model.Type)
			if !ok {
				return fmt.Errorf("invalid base type %T", baseType)
			}
			return f.Validate(tv, bt)
		},
	})
}

// ValuesEqual reports whether two typed values are equal in the value space.
func ValuesEqual(left, right model.TypedValue) bool {
	return model.CompareTypedValues(left, right)
}

// TypedValueForFacet creates a typed value used during facet validation.
func TypedValueForFacet(value string, typ model.Type) model.TypedValue {
	switch t := typ.(type) {
	case *model.SimpleType:
		if parsed, err := t.ParseValue(value); err == nil {
			return parsed
		}
	case *model.BuiltinType:
		if parsed, err := t.ParseValue(value); err == nil {
			return parsed
		}
	}
	return &model.StringTypedValue{Value: value, Typ: typ}
}

// IsLengthFacet reports whether facet is one of length, minLength, or maxLength.
func IsLengthFacet(facet model.Facet) bool {
	switch facet.(type) {
	case *model.Length, *model.MinLength, *model.MaxLength:
		return true
	default:
		return false
	}
}

// ValidateApplicability checks whether a facet can be applied to a base type.
func ValidateApplicability(facetName string, baseType model.Type, baseQName model.QName) error {
	baseTypeName := baseTypeNameForApplicability(baseType, baseQName)

	if baseType != nil {
		if baseST, ok := baseType.(*model.SimpleType); ok && baseST.Variety() == model.UnionVariety {
			switch facetName {
			case "pattern", "enumeration":
			default:
				return fmt.Errorf("facet %s is not applicable to union type %s", facetName, baseTypeName)
			}
		}
	}

	if isRangeFacetName(facetName) {
		if isListType(baseType, baseTypeName) {
			return fmt.Errorf("facet %s is not applicable to list type %s", facetName, baseTypeName)
		}
		facets := fundamentalFacetsFor(baseType, baseQName)
		if facets == nil || (facets.Ordered != model.OrderedTotal && facets.Ordered != model.OrderedPartial) {
			return fmt.Errorf("facet %s is only applicable to ordered types, but base type %s is not ordered", facetName, baseTypeName)
		}
	}

	if isDigitFacetName(facetName) {
		facets := fundamentalFacetsFor(baseType, baseQName)
		if facets == nil || !facets.Numeric {
			return fmt.Errorf("facet %s is only applicable to numeric types, but base type %s is not numeric", facetName, baseTypeName)
		}
	}

	if isLengthFacetName(facetName) {
		if isListType(baseType, baseTypeName) {
			return nil
		}
		primitiveName := primitiveTypeName(baseType, baseQName)
		switch {
		case primitiveName == "boolean":
			return fmt.Errorf("facet %s is not applicable to boolean type", facetName)
		case primitiveName == "duration":
			return fmt.Errorf("facet %s is not applicable to duration type", facetName)
		case isNumericTypeName(primitiveName):
			return fmt.Errorf("facet %s is not applicable to numeric type %s", facetName, baseTypeName)
		case isDateTimeTypeName(primitiveName):
			return fmt.Errorf("facet %s is not applicable to date/time type %s", facetName, baseTypeName)
		}
	}

	return nil
}

func NewEnumeration(values []string) *model.Enumeration {
	return model.NewEnumeration(values)
}

func NewMinInclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return newMinInclusiveFacet(lexical, baseType)
}

func NewMaxInclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return newMaxInclusiveFacet(lexical, baseType)
}

func NewMinExclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return newMinExclusiveFacet(lexical, baseType)
}

func NewMaxExclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return newMaxExclusiveFacet(lexical, baseType)
}

// FormatEnumerationValues returns a quoted list for enumeration errors.
func FormatEnumerationValues(values []string) string {
	if len(values) == 0 {
		return "[]"
	}
	quoted := make([]string, len(values))
	for i, facetValue := range values {
		quoted[i] = strconv.Quote(facetValue)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// ParseDurationToTimeDuration parses an XSD duration into time.Duration.
func ParseDurationToTimeDuration(text string) (time.Duration, error) {
	xsdDur, err := durationlex.Parse(text)
	if err != nil {
		return 0, err
	}
	if xsdDur.Years != 0 || xsdDur.Months != 0 {
		return 0, fmt.Errorf("durations with years or months cannot be converted to time.Duration (indeterminate)")
	}

	const maxDuration = time.Duration(^uint64(0) >> 1)

	componentDuration := func(value int, unit time.Duration) (time.Duration, error) {
		if value == 0 {
			return 0, nil
		}
		if value < 0 {
			return 0, fmt.Errorf("duration component out of range")
		}
		limit := int64(maxDuration / unit)
		if int64(value) > limit {
			return 0, fmt.Errorf("duration too large")
		}
		return time.Duration(value) * unit, nil
	}

	addDuration := func(total, delta time.Duration) (time.Duration, error) {
		if delta < 0 {
			return 0, fmt.Errorf("duration component out of range")
		}
		if total > maxDuration-delta {
			return 0, fmt.Errorf("duration too large")
		}
		return total + delta, nil
	}

	dur := time.Duration(0)
	var delta time.Duration

	delta, err = componentDuration(xsdDur.Days, 24*time.Hour)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	delta, err = componentDuration(xsdDur.Hours, time.Hour)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	delta, err = componentDuration(xsdDur.Minutes, time.Minute)
	if err != nil {
		return 0, err
	}
	dur, err = addDuration(dur, delta)
	if err != nil {
		return 0, err
	}

	secondsDuration, err := model.SecondsToDuration(xsdDur.Seconds)
	if err != nil {
		return 0, err
	}
	if dur, err = addDuration(dur, secondsDuration); err != nil {
		return 0, err
	}

	if xsdDur.Negative {
		dur = -dur
	}
	return dur, nil
}

func isListTypeForFacetValidation(typ model.Type) bool {
	switch t := typ.(type) {
	case *model.SimpleType:
		return t.Variety() == model.ListVariety || t.List != nil
	case *model.BuiltinType:
		return builtins.IsBuiltinListTypeName(t.Name().Local)
	default:
		return false
	}
}

// IsQNameOrNotationType reports whether typ represents xs:QName or xs:NOTATION.
func IsQNameOrNotationType(typ model.Type) bool {
	if typ == nil {
		return false
	}
	switch t := typ.(type) {
	case *model.SimpleType:
		return t.IsQNameOrNotationType()
	default:
		return model.IsQNameOrNotation(typ.Name())
	}
}

func baseTypeNameForApplicability(baseType model.Type, baseQName model.QName) string {
	if baseType != nil {
		return baseType.Name().Local
	}
	return baseQName.Local
}

func fundamentalFacetsFor(baseType model.Type, baseQName model.QName) *model.FundamentalFacets {
	if baseType != nil {
		if baseType.IsBuiltin() {
			return baseType.FundamentalFacets()
		}
		if primitive := baseType.PrimitiveType(); primitive != nil {
			return primitive.FundamentalFacets()
		}
	}
	if baseQName.Namespace == model.XSDNamespace && baseQName.Local != "" {
		if builtin := builtins.Get(builtins.TypeName(baseQName.Local)); builtin != nil {
			return builtin.FundamentalFacets()
		}
	}
	return nil
}

func primitiveTypeName(baseType model.Type, baseQName model.QName) string {
	if baseType != nil {
		if primitive := baseType.PrimitiveType(); primitive != nil {
			return primitive.Name().Local
		}
		return baseType.Name().Local
	}
	return baseQName.Local
}

func isListType(baseType model.Type, baseTypeName string) bool {
	if baseTypeName != "" && builtins.IsBuiltinListTypeName(baseTypeName) {
		return true
	}
	if baseType == nil {
		return false
	}
	if baseST, ok := baseType.(*model.SimpleType); ok {
		return baseST.Variety() == model.ListVariety
	}
	return false
}

func isRangeFacetName(name string) bool {
	switch name {
	case "minExclusive", "maxExclusive", "minInclusive", "maxInclusive":
		return true
	default:
		return false
	}
}

func isDigitFacetName(name string) bool {
	switch name {
	case "totalDigits", "fractionDigits":
		return true
	default:
		return false
	}
}

func isLengthFacetName(name string) bool {
	switch name {
	case "length", "minLength", "maxLength":
		return true
	default:
		return false
	}
}

func isNumericTypeName(typeName string) bool {
	switch typeName {
	case "decimal", "float", "double", "integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger":
		return true
	default:
		return false
	}
}

func isDateTimeTypeName(typeName string) bool {
	switch typeName {
	case "dateTime", "date", "time", "gYearMonth", "gYear", "gMonthDay", "gDay", "gMonth":
		return true
	default:
		return false
	}
}
