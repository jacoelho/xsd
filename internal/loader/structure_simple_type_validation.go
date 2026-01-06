package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateSimpleTypeStructure validates structural constraints of a simple type
// Does not validate type references (which might be forward references or imports)
func validateSimpleTypeStructure(schema *schema.Schema, st *types.SimpleType) error {
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
		// List types can be defined by xs:list or by restriction of a list type
		if st.List != nil {
			if err := validateListType(schema, st.List); err != nil {
				return fmt.Errorf("list: %w", err)
			}
		} else if st.Restriction != nil {
			// List type derived by restriction of another list type - validate facets
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	case types.UnionVariety:
		// Union types can be defined by xs:union or by restriction of a union type
		if st.Union != nil {
			if err := validateUnionType(schema, st.Union); err != nil {
				return fmt.Errorf("union: %w", err)
			}
		} else if st.Restriction != nil {
			// Union type derived by restriction of another union type - validate facets
			if err := validateRestriction(schema, st, st.Restriction); err != nil {
				return fmt.Errorf("restriction: %w", err)
			}
		}
	}
	if err := validateSimpleTypeDerivationConstraints(schema, st); err != nil {
		return err
	}
	return nil
}

// validateSimpleTypeDerivationConstraints validates final constraints on simple type derivation
func validateSimpleTypeDerivationConstraints(schema *schema.Schema, st *types.SimpleType) error {
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
			itemType = resolveSimpleTypeReference(schema, st.List.ItemType)
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
				if member := resolveSimpleTypeReference(schema, memberQName); member != nil {
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
