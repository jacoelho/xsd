package model

import "fmt"

// ValidateFacetValue validates lexical against facets for the provided base type.
func ValidateFacetValue(lexical string, baseType Type, facets []Facet, context map[string]string) error {
	if len(facets) == 0 {
		return nil
	}

	isQNameOrNotation := IsQNameOrNotationType(baseType)
	isListType := isListTypeForFacetValidation(baseType)

	var typed TypedValue
	for _, facet := range facets {
		if IsLengthFacet(facet) && !isListType && isQNameOrNotation {
			continue
		}

		if isQNameOrNotation && !isListType {
			if enumFacet, ok := facet.(*Enumeration); ok {
				if err := enumFacet.ValidateLexicalQName(lexical, baseType, context); err != nil {
					return err
				}
				continue
			}
		}

		if lexicalFacet, ok := facet.(LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(lexical, baseType); err != nil {
				return fmt.Errorf("facet '%s' violation: %w", facet.Name(), err)
			}
			continue
		}

		if typed == nil {
			typed = typedValueForFacet(lexical, baseType)
		}
		if err := facet.Validate(typed, baseType); err != nil {
			return fmt.Errorf("facet '%s' violation: %w", facet.Name(), err)
		}
	}

	return nil
}

// IsLengthFacet reports whether facet is one of length, minLength, or maxLength.
func IsLengthFacet(facet Facet) bool {
	switch facet.(type) {
	case *Length, *MinLength, *MaxLength:
		return true
	default:
		return false
	}
}

// IsQNameOrNotationType reports whether typ represents xs:QName or xs:NOTATION.
func IsQNameOrNotationType(typ Type) bool {
	if typ == nil {
		return false
	}
	switch t := typ.(type) {
	case *SimpleType:
		return t.IsQNameOrNotationType()
	default:
		return IsQNameOrNotation(typ.Name())
	}
}

func isListTypeForFacetValidation(typ Type) bool {
	switch t := typ.(type) {
	case *SimpleType:
		return t.Variety() == ListVariety || t.List != nil
	case *BuiltinType:
		return IsBuiltinListTypeName(t.Name().Local)
	default:
		return false
	}
}
