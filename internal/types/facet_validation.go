package types

import "fmt"

// ValidateValueAgainstFacets checks a normalized lexical value against facets.
// It applies lexical and typed facets in order, including QName/NOTATION handling.
func ValidateValueAgainstFacets(value string, baseType Type, facets []Facet, context map[string]string) error {
	if len(facets) == 0 {
		return nil
	}

	var typed TypedValue
	for _, facet := range facets {
		if shouldSkipLengthFacet(baseType, facet) {
			continue
		}
		if enumFacet, ok := facet.(*Enumeration); ok && IsQNameOrNotationType(baseType) && !isListTypeForFacetValidation(baseType) {
			if err := enumFacet.ValidateLexicalQName(value, baseType, context); err != nil {
				return err
			}
			continue
		}
		if lexicalFacet, ok := facet.(LexicalValidator); ok {
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

func isListTypeForFacetValidation(typ Type) bool {
	switch t := typ.(type) {
	case *SimpleType:
		return t.Variety() == ListVariety || t.List != nil
	case *BuiltinType:
		return isBuiltinListType(t.Name().Local)
	default:
		return false
	}
}

func shouldSkipLengthFacet(baseType Type, facet Facet) bool {
	if !IsLengthFacet(facet) {
		return false
	}
	if isListTypeForFacetValidation(baseType) {
		return false
	}
	return IsQNameOrNotationType(baseType)
}
