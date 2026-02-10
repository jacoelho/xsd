package model

import (
	"fmt"

	typefacetcore "github.com/jacoelho/xsd/internal/typefacet/internalcore"
)

// validateValueAgainstFacets checks a normalized lexical value against facets.
// It applies lexical and typed facets in order, including QName/NOTATION handling.
func validateValueAgainstFacets(value string, baseType Type, facets []Facet, context map[string]string) error {
	facetsAny := make([]any, len(facets))
	for i, facet := range facets {
		facetsAny[i] = facet
	}

	return typefacetcore.ValidateValueAgainstFacets(value, baseType, facetsAny, context, typefacetcore.ValidateFacetOps{
		FacetName: func(facet any) string {
			f, ok := facet.(Facet)
			if !ok {
				return fmt.Sprintf("invalid facet %T", facet)
			}
			return f.Name()
		},
		ShouldSkipLengthFacet: func(baseType any, facet any) bool {
			bt, ok := baseType.(Type)
			if !ok {
				return false
			}
			f, ok := facet.(Facet)
			if !ok {
				return false
			}
			return shouldSkipLengthFacet(bt, f)
		},
		IsQNameOrNotationType: func(baseType any) bool {
			bt, ok := baseType.(Type)
			return ok && isQNameOrNotationType(bt)
		},
		IsListTypeForFacetValidation: func(baseType any) bool {
			bt, ok := baseType.(Type)
			return ok && isListTypeForFacetValidation(bt)
		},
		ValidateQNameEnumerationLexical: func(facet any, value string, baseType any, context map[string]string) (bool, error) {
			enumFacet, ok := facet.(*Enumeration)
			if !ok {
				return false, nil
			}
			bt, ok := baseType.(Type)
			if !ok {
				return true, fmt.Errorf("invalid base type %T", baseType)
			}
			return true, enumFacet.ValidateLexicalQName(value, bt, context)
		},
		ValidateLexicalFacet: func(facet any, value string, baseType any) (bool, error) {
			lexicalFacet, ok := facet.(LexicalValidator)
			if !ok {
				return false, nil
			}
			bt, ok := baseType.(Type)
			if !ok {
				return true, fmt.Errorf("invalid base type %T", baseType)
			}
			return true, lexicalFacet.ValidateLexical(value, bt)
		},
		TypedValueForFacet: func(value string, baseType any) any {
			bt, ok := baseType.(Type)
			if !ok {
				return &StringTypedValue{Value: value}
			}
			return typedValueForFacet(value, bt)
		},
		ValidateFacet: func(facet any, value any, baseType any) error {
			f, ok := facet.(Facet)
			if !ok {
				return fmt.Errorf("invalid facet %T", facet)
			}
			tv, ok := value.(TypedValue)
			if !ok {
				return fmt.Errorf("invalid typed value %T", value)
			}
			bt, ok := baseType.(Type)
			if !ok {
				return fmt.Errorf("invalid base type %T", baseType)
			}
			return f.Validate(tv, bt)
		},
	})
}

func isListTypeForFacetValidation(typ Type) bool {
	switch t := typ.(type) {
	case *SimpleType:
		return t.Variety() == ListVariety || t.List != nil
	case *BuiltinType:
		return isBuiltinListTypeName(t.Name().Local)
	default:
		return false
	}
}

func shouldSkipLengthFacet(baseType Type, facet Facet) bool {
	if !isLengthFacet(facet) {
		return false
	}
	if isListTypeForFacetValidation(baseType) {
		return false
	}
	return isQNameOrNotationType(baseType)
}
