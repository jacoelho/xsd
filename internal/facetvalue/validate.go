package facetvalue

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/durationconv"
	model "github.com/jacoelho/xsd/internal/model"
)

// Apply validates value against all facets in declaration order.
func Apply(value model.TypedValue, facets []model.Facet, baseType model.Type) error {
	for _, facet := range facets {
		if err := facet.Validate(value, baseType); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates a lexical value against the provided facets and base type.
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

// NewEnumeration creates an enumeration facet from lexical values.
func NewEnumeration(values []string) *model.Enumeration {
	return model.NewEnumeration(values)
}

// NewMinInclusive constructs a minInclusive facet.
func NewMinInclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return model.NewMinInclusive(lexical, baseType)
}

// NewMaxInclusive constructs a maxInclusive facet.
func NewMaxInclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return model.NewMaxInclusive(lexical, baseType)
}

// NewMinExclusive constructs a minExclusive facet.
func NewMinExclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return model.NewMinExclusive(lexical, baseType)
}

// NewMaxExclusive constructs a maxExclusive facet.
func NewMaxExclusive(lexical string, baseType model.Type) (model.Facet, error) {
	return model.NewMaxExclusive(lexical, baseType)
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
	dur, err := durationconv.ParseToStdDuration(text)
	if err != nil {
		switch {
		case errors.Is(err, durationconv.ErrIndeterminate):
			return 0, fmt.Errorf("durations with years or months cannot be converted to time.Duration (indeterminate)")
		case errors.Is(err, durationconv.ErrOverflow):
			return 0, fmt.Errorf("duration too large")
		case errors.Is(err, durationconv.ErrComponentRange):
			return 0, fmt.Errorf("duration component out of range")
		default:
			return 0, err
		}
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
