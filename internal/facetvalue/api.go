package facetvalue

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/durationlex"
	model "github.com/jacoelho/xsd/internal/model"
)

func Apply(value model.TypedValue, facets []model.Facet, baseType model.Type) error {
	for _, facet := range facets {
		if err := facet.Validate(value, baseType); err != nil {
			return err
		}
	}
	return nil
}

func Validate(value string, baseType model.Type, facets []model.Facet, context map[string]string) error {
	if len(facets) == 0 {
		return nil
	}

	isQNameOrNotation := IsQNameOrNotationType(baseType)
	isListType := isListTypeForFacetValidation(baseType)

	var typed model.TypedValue
	for _, facet := range facets {
		if IsLengthFacet(facet) && !isListType && isQNameOrNotation {
			continue
		}

		if isQNameOrNotation && !isListType {
			if enumFacet, ok := facet.(*model.Enumeration); ok {
				if err := enumFacet.ValidateLexicalQName(value, baseType, context); err != nil {
					return err
				}
				continue
			}
		}

		if lexicalFacet, ok := facet.(model.LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(value, baseType); err != nil {
				return fmt.Errorf("facet '%s' violation: %w", facet.Name(), err)
			}
			continue
		}

		if typed == nil {
			typed = TypedValueForFacet(value, baseType)
		}
		if err := facet.Validate(typed, baseType); err != nil {
			return fmt.Errorf("facet '%s' violation: %w", facet.Name(), err)
		}
	}

	return nil
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
	return model.ValidateFacetApplicability(facetName, baseType, baseQName)
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
