package model

import (
	"fmt"
)

func (s *SimpleType) Validate(lexical string) error {
	return s.ValidateWithContext(lexical, nil)
}

// ValidateWithContext checks if a lexical value is valid for this type using namespace context.
func (s *SimpleType) ValidateWithContext(lexical string, context map[string]string) error {
	if s == nil {
		return fmt.Errorf("cannot validate value for nil simple type")
	}
	normalized, err := normalizeValue(lexical, s)
	if err != nil {
		return err
	}
	return s.validateNormalizedWithContext(normalized, make(map[*SimpleType]bool), context)
}

func (s *SimpleType) validateNormalized(normalized string, visited map[*SimpleType]bool) error {
	return s.validateNormalizedWithContext(normalized, visited, nil)
}

func (s *SimpleType) validateNormalizedWithContext(normalized string, visited map[*SimpleType]bool, context map[string]string) error {
	return validateSimpleTypeNormalizedWithOptions(s, normalized, context, visited, SimpleTypeValidationOptions{})
}

func collectSimpleTypeFacets(st *SimpleType, visited map[*SimpleType]bool) ([]Facet, error) {
	_ = visited
	return CollectSimpleTypeFacetsWithResolver(st, nil, nil)
}

func needsBuiltinListMinLength(st *SimpleType) bool {
	if st == nil {
		return false
	}
	if st.IsBuiltin() && isBuiltinListTypeName(st.QName.Local) {
		return true
	}
	if st.ResolvedBase != nil {
		if bt, ok := AsBuiltinType(st.ResolvedBase); ok && isBuiltinListTypeName(bt.Name().Local) {
			return true
		}
	}
	if st.Restriction != nil && !st.Restriction.Base.IsZero() &&
		st.Restriction.Base.Namespace == XSDNamespace &&
		isBuiltinListTypeName(st.Restriction.Base.Local) {
		return true
	}
	return false
}

func validateNormalizedFacetsWithContext(normalized string, baseType Type, facets []Facet, context map[string]string) error {
	var typed TypedValue
	for _, facet := range facets {
		if enumFacet, ok := facet.(*Enumeration); ok && context != nil && isQNameOrNotationType(baseType) {
			if err := enumFacet.ValidateLexicalQName(normalized, baseType, context); err != nil {
				return err
			}
			continue
		}
		if lexicalFacet, ok := facet.(LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(normalized, baseType); err != nil {
				return err
			}
			continue
		}
		if typed == nil {
			typed = typedValueForFacet(normalized, baseType)
		}
		if err := facet.Validate(typed, baseType); err != nil {
			return err
		}
	}
	return nil
}

// ParseValue converts a lexical value to a TypedValue.
func (s *SimpleType) ParseValue(lexical string) (TypedValue, error) {
	return s.parseValueInternal(lexical, true)
}

func (s *SimpleType) parseValueInternal(lexical string, validateFacets bool) (TypedValue, error) {
	if s == nil {
		return nil, fmt.Errorf("cannot parse value for nil simple type")
	}
	normalized, err := normalizeValue(lexical, s)
	if err != nil {
		return nil, err
	}
	if validateFacets {
		if vErr := s.validateNormalized(normalized, make(map[*SimpleType]bool)); vErr != nil {
			return nil, vErr
		}
	}

	// first, try to parse based on the type's own name (for built-in types)
	if s.IsBuiltin() {
		typeName := TypeName(s.QName.Local)
		var result TypedValue
		result, err = parseValueForType(normalized, typeName, s)
		if err == nil {
			return result, nil
		}
	}

	// for user-defined types or if built-in type not handled above, use primitive type
	primitiveType := s.PrimitiveType()
	if primitiveType == nil {
		if s.Variety() != AtomicVariety {
			return &StringTypedValue{Value: normalized, Typ: s}, nil
		}
		return nil, fmt.Errorf("cannot determine primitive type")
	}

	primitiveST, ok := as[*SimpleType](primitiveType)
	if !ok {
		// try BuiltinType
		if builtinType, ok := AsBuiltinType(primitiveType); ok {
			return builtinType.ParseValue(normalized)
		}
		if s.Variety() != AtomicVariety {
			return &StringTypedValue{Value: normalized, Typ: s}, nil
		}
		return nil, fmt.Errorf("primitive type is not a SimpleType or BuiltinType")
	}

	primitiveName := TypeName(primitiveST.QName.Local)
	parsed, err := parseValueForType(normalized, primitiveName, s)
	if err == nil {
		return parsed, nil
	}
	if s.Variety() != AtomicVariety {
		return &StringTypedValue{Value: normalized, Typ: s}, nil
	}
	return nil, err
}
