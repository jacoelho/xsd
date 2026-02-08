package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// validateSimpleTypeStructure validates structural constraints of a simple type
// Does not validate type references (which might be forward references or imports)
func validateSimpleTypeStructure(schema *parser.Schema, st *types.SimpleType) error {
	switch st.Variety() {
	case types.AtomicVariety:
		if st.Restriction != nil {
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
			if baseType := resolveSimpleTypeRestrictionBase(schema, st, st.Restriction); baseType != nil {
				if baseST, ok := baseType.(*types.SimpleType); ok {
					if err := validateLengthFacetInheritance(facetsFromRestriction(st.Restriction), baseST); err != nil {
						return fmt.Errorf("restriction: %w", err)
					}
				}
			}
		}
	case types.ListVariety:
		// list types can be defined by xs:list or by restriction of a list type
		if st.List != nil {
			if err := validateListType(schema, st.List); err != nil {
				return fmt.Errorf("list: %w", err)
			}
		} else if st.Restriction != nil {
			// list type derived by restriction of another list type - validate facets
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	case types.UnionVariety:
		// union types can be defined by xs:union or by restriction of a union type
		if st.Union != nil {
			if err := validateUnionType(schema, st.Union); err != nil {
				return fmt.Errorf("union: %w", err)
			}
		} else if st.Restriction != nil {
			// union type derived by restriction of another union type - validate facets
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	}
	if st.Variety() == types.ListVariety && st.WhiteSpace() != types.WhiteSpaceCollapse {
		return fmt.Errorf("list whiteSpace facet must be 'collapse'")
	}
	if err := validateSimpleTypeDerivationConstraints(schema, st); err != nil {
		return err
	}
	return nil
}

// validateSimpleTypeDerivationConstraints validates final constraints on simple type derivation
func validateSimpleTypeDerivationConstraints(schema *parser.Schema, st *types.SimpleType) error {
	if st == nil {
		return nil
	}

	if st.Restriction != nil {
		baseType := resolveSimpleTypeRestrictionBase(schema, st, st.Restriction)
		if baseST, ok := baseType.(*types.SimpleType); ok {
			if baseST.Final.Has(types.DerivationRestriction) {
				return fmt.Errorf("cannot restrict type '%s': base type is final for restriction", baseST.Name())
			}
		}
	}

	if st.List != nil {
		itemType := st.ItemType
		if itemType == nil && st.List.InlineItemType != nil {
			itemType = st.List.InlineItemType
		}
		if itemType == nil && !st.List.ItemType.IsZero() {
			itemType = typeops.ResolveSimpleTypeReferenceAllowMissing(schema, st.List.ItemType)
		}
		if itemST, ok := itemType.(*types.SimpleType); ok {
			if itemST.Final.Has(types.DerivationList) {
				return fmt.Errorf("cannot derive list from type '%s': base type is final for list", itemST.Name())
			}
		}
	}

	if st.Union != nil {
		memberTypes := st.MemberTypes
		if len(memberTypes) == 0 {
			for _, inlineType := range st.Union.InlineTypes {
				memberTypes = append(memberTypes, inlineType)
			}
			for _, memberQName := range st.Union.MemberTypes {
				if member := typeops.ResolveSimpleTypeReferenceAllowMissing(schema, memberQName); member != nil {
					memberTypes = append(memberTypes, member)
				}
			}
		}
		for _, member := range memberTypes {
			if memberST, ok := member.(*types.SimpleType); ok {
				if memberST.Final.Has(types.DerivationUnion) {
					return fmt.Errorf("cannot derive union from type '%s': base type is final for union", memberST.Name())
				}
			}
		}
	}

	return nil
}

// validateUnionType validates a union type definition
func validateUnionType(schema *parser.Schema, unionType *types.UnionType) error {
	// union must have at least one member type (from attribute or inline)
	if len(unionType.MemberTypes) == 0 && len(unionType.InlineTypes) == 0 {
		return fmt.Errorf("union type must have at least one member type")
	}

	// validate that all member types are simple types (not complex types)
	// union types can only have simple types as members
	for i, memberQName := range unionType.MemberTypes {
		// check if it's a built-in type (all built-in types in XSD namespace are simple)
		if memberQName.Namespace == types.XSDNamespace {
			// check if it's an XSD 1.1 type (not supported)
			if isXSD11Type(memberQName.Local) {
				return fmt.Errorf("union memberType %d: '%s' is an XSD 1.1 type (not supported in XSD 1.0)", i+1, memberQName.Local)
			}
			// built-in types in XSD namespace are always simple types
			continue
		}

		if memberType, ok := lookupTypeDef(schema, memberQName); ok {
			// union members must be simple types, not complex types
			if _, isComplex := memberType.(*types.ComplexType); isComplex {
				return fmt.Errorf("union memberType %d: '%s' is a complex type (union types can only have simple types as members)", i+1, memberQName.Local)
			}
		}
	}

	// validate inline types (they're already SimpleType, so no need to check)
	// inline types are parsed as SimpleType, so they're always valid

	return nil
}

// isXSD11Type checks if a type name is an XSD 1.1 type (not supported in XSD 1.0)
func isXSD11Type(typeName string) bool {
	xsd11Types := map[string]bool{
		"timeDuration":      true,
		"yearMonthDuration": true,
		"dayTimeDuration":   true,
		"dateTimeStamp":     true,
		"precisionDecimal":  true,
	}
	return xsd11Types[typeName]
}

// validateListType validates a list type definition
func validateListType(schema *parser.Schema, listType *types.ListType) error {
	// list type must have itemType (either via itemType attribute or inline simpleType child per XSD spec)
	if listType.ItemType.IsZero() {
		if listType.InlineItemType == nil {
			return fmt.Errorf("list type must have itemType attribute or inline simpleType child")
		}
		// inline simpleType is present - validate it
		if err := validateSimpleTypeStructure(schema, listType.InlineItemType); err != nil {
			return fmt.Errorf("inline simpleType in list: %w", err)
		}
		// list itemType must be atomic or union (NOT list)
		variety := listType.InlineItemType.Variety()
		if variety != types.AtomicVariety && variety != types.UnionVariety {
			return fmt.Errorf("list itemType must be atomic or union, got %s", variety)
		}
		return nil // inline simpleType is valid
	}

	// list itemType must be atomic or union (NOT list)
	// check if it's a built-in type (always atomic)
	if listType.ItemType.Namespace == types.XSDNamespace {
		return nil // built-in types are always atomic
	}

	// check if it's a user-defined type in this schema
	if defType, ok := lookupTypeDef(schema, listType.ItemType); ok {
		st, ok := defType.(*types.SimpleType)
		if !ok {
			return fmt.Errorf("list itemType must be a simple type, got %T", defType)
		}
		// list itemType must be atomic or union
		variety := st.Variety()
		if variety != types.AtomicVariety && variety != types.UnionVariety {
			return fmt.Errorf("list itemType must be atomic or union, got %s", variety)
		}
	}
	// if type not found, might be forward reference - skip validation

	return nil
}
