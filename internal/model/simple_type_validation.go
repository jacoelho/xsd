package model

import (
	"fmt"

	qnamelex "github.com/jacoelho/xsd/internal/qname"
)

func (s *SimpleType) Validate(lexical string) error {
	return s.ValidateWithContext(lexical, nil)
}

// ValidateWithContext checks if a lexical value is valid for this type using namespace context.
func (s *SimpleType) ValidateWithContext(lexical string, context map[string]string) error {
	if s == nil {
		return fmt.Errorf("cannot validate value for nil simple type")
	}
	normalized, err := NormalizeValue(lexical, s)
	if err != nil {
		return err
	}
	return s.validateNormalizedWithContext(normalized, make(map[*SimpleType]bool), context)
}

func (s *SimpleType) validateNormalized(normalized string, visited map[*SimpleType]bool) error {
	return s.validateNormalizedWithContext(normalized, visited, nil)
}

func (s *SimpleType) validateNormalizedWithContext(normalized string, visited map[*SimpleType]bool, context map[string]string) error {
	if s == nil {
		return nil
	}
	if visited[s] {
		return nil
	}
	visited[s] = true
	defer delete(visited, s)

	if err := s.validateNormalizedLexicalWithContext(normalized, visited, context); err != nil {
		return err
	}
	facets, err := collectSimpleTypeFacets(s, make(map[*SimpleType]bool))
	if err != nil {
		return err
	}
	if len(facets) == 0 {
		return nil
	}
	return validateNormalizedFacetsWithContext(normalized, s, facets, context)
}

func (s *SimpleType) validateNormalizedLexicalWithContext(normalized string, visited map[*SimpleType]bool, context map[string]string) error {
	switch s.Variety() {
	case ListVariety:
		itemType, ok := ListItemType(s)
		if !ok || itemType == nil {
			return fmt.Errorf("list item type is missing")
		}
		for item := range FieldsXMLWhitespaceSeq(normalized) {
			if err := validateTypeLexicalWithContext(itemType, item, visited, context); err != nil {
				return err
			}
		}
		return nil
	case UnionVariety:
		members := s.MemberTypes
		if len(members) == 0 {
			members = unionMemberTypes(s)
		}
		if len(members) == 0 {
			return fmt.Errorf("union has no member types")
		}
		var firstErr error
		for _, member := range members {
			if err := validateTypeLexicalWithContext(member, normalized, visited, context); err == nil {
				return nil
			} else if firstErr == nil {
				firstErr = err
			}
		}
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("value %q does not match any member type", normalized)
	default:
		return s.validateAtomicLexicalWithContext(normalized, context)
	}
}

func (s *SimpleType) validateAtomicLexicalWithContext(normalized string, context map[string]string) error {
	if context != nil && IsQNameOrNotationType(s) {
		if _, err := qnamelex.ParseQNameValue(normalized, context); err != nil {
			return err
		}
	}
	if s.IsBuiltin() {
		if builtinType := GetBuiltinNS(s.QName.Namespace, s.QName.Local); builtinType != nil {
			return builtinType.Validate(normalized)
		}
	}
	if s.Restriction != nil {
		primitive := s.PrimitiveType()
		if builtinType, ok := AsBuiltinType(primitive); ok {
			return builtinType.Validate(normalized)
		}
		if primitiveST, ok := AsSimpleType(primitive); ok && primitiveST.IsBuiltin() {
			if builtinType := GetBuiltinNS(primitiveST.QName.Namespace, primitiveST.QName.Local); builtinType != nil {
				return builtinType.Validate(normalized)
			}
		}
	}
	return nil
}

func validateTypeLexicalWithContext(typ Type, lexical string, visited map[*SimpleType]bool, context map[string]string) error {
	if typ == nil {
		return nil
	}
	normalized, err := NormalizeValue(lexical, typ)
	if err != nil {
		return err
	}
	if st, ok := AsSimpleType(typ); ok {
		return st.validateNormalizedWithContext(normalized, visited, context)
	}
	if bt, ok := AsBuiltinType(typ); ok {
		if context != nil && IsQNameOrNotationType(bt) {
			if _, err := qnamelex.ParseQNameValue(normalized, context); err != nil {
				return err
			}
		}
		return bt.Validate(normalized)
	}
	return nil
}

func collectSimpleTypeFacets(st *SimpleType, visited map[*SimpleType]bool) ([]Facet, error) {
	if st == nil {
		return nil, nil
	}
	if visited[st] {
		return nil, nil
	}
	visited[st] = true
	defer delete(visited, st)

	var result []Facet
	if st.ResolvedBase != nil {
		if baseST, ok := AsSimpleType(st.ResolvedBase); ok {
			facets, err := collectSimpleTypeFacets(baseST, visited)
			if err != nil {
				return nil, err
			}
			result = append(result, facets...)
		}
	} else if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		if base := GetBuiltinNS(st.Restriction.Base.Namespace, st.Restriction.Base.Local); base != nil {
			if baseST, ok := AsSimpleType(base); ok {
				facets, err := collectSimpleTypeFacets(baseST, visited)
				if err != nil {
					return nil, err
				}
				result = append(result, facets...)
			}
		}
	}

	if needsBuiltinListMinLength(st) {
		result = append(result, &MinLength{Value: 1})
	}

	if st.Restriction != nil {
		for _, facet := range st.Restriction.Facets {
			if f, ok := facet.(Facet); ok {
				if compilable, ok := f.(interface{ ValidateSyntax() error }); ok {
					if err := compilable.ValidateSyntax(); err != nil {
						return nil, err
					}
				}
				result = append(result, f)
			}
		}
	}

	return result, nil
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
		if enumFacet, ok := facet.(*Enumeration); ok && context != nil && IsQNameOrNotationType(baseType) {
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
			typed = TypedValueForFacet(normalized, baseType)
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
	normalized, err := NormalizeValue(lexical, s)
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
		result, err = ParseValueForType(normalized, typeName, s)
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
	parsed, err := ParseValueForType(normalized, primitiveName, s)
	if err == nil {
		return parsed, nil
	}
	if s.Variety() != AtomicVariety {
		return &StringTypedValue{Value: normalized, Typ: s}, nil
	}
	return nil, err
}
