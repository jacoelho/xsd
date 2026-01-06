package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateListType validates a list type definition
func validateListType(schema *schema.Schema, listType *types.ListType) error {
	// List type must have itemType (either via itemType attribute or inline simpleType child per XSD spec)
	if listType.ItemType.IsZero() {
		if listType.InlineItemType == nil {
			return fmt.Errorf("list type must have itemType attribute or inline simpleType child")
		}
		// Inline simpleType is present - validate it
		if err := validateSimpleTypeStructure(schema, listType.InlineItemType); err != nil {
			return fmt.Errorf("inline simpleType in list: %w", err)
		}
		// List itemType must be atomic or union (NOT list)
		variety := listType.InlineItemType.Variety()
		if variety != types.AtomicVariety && variety != types.UnionVariety {
			return fmt.Errorf("list itemType must be atomic or union, got %v", variety)
		}
		return nil // Inline simpleType is valid
	}

	// List itemType must be atomic or union (NOT list)
	// Check if it's a built-in type (always atomic)
	if listType.ItemType.Namespace == types.XSDNamespace {
		return nil // Built-in types are always atomic
	}

	// Check if it's a user-defined type in this schema
	if defType, ok := schema.TypeDefs[listType.ItemType]; ok {
		if st, ok := defType.(*types.SimpleType); ok {
			// List itemType must be atomic or union
			variety := st.Variety()
			if variety != types.AtomicVariety && variety != types.UnionVariety {
				return fmt.Errorf("list itemType must be atomic or union, got %v", variety)
			}
		} else {
			return fmt.Errorf("list itemType must be a simple type, got %T", defType)
		}
	}
	// If type not found, might be forward reference - skip validation

	return nil
}
