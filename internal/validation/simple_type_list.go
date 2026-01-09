package validation

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

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
			return fmt.Errorf("list itemType must be atomic or union, got %v", variety)
		}
		return nil // inline simpleType is valid
	}

	// list itemType must be atomic or union (NOT list)
	// check if it's a built-in type (always atomic)
	if listType.ItemType.Namespace == types.XSDNamespace {
		return nil // built-in types are always atomic
	}

	// check if it's a user-defined type in this schema
	if defType, ok := schema.TypeDefs[listType.ItemType]; ok {
		if st, ok := defType.(*types.SimpleType); ok {
			// list itemType must be atomic or union
			variety := st.Variety()
			if variety != types.AtomicVariety && variety != types.UnionVariety {
				return fmt.Errorf("list itemType must be atomic or union, got %v", variety)
			}
		} else {
			return fmt.Errorf("list itemType must be a simple type, got %T", defType)
		}
	}
	// if type not found, might be forward reference - skip validation

	return nil
}
