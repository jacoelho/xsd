package model

// ListItemType returns the item type for list simple types.
// It resolves built-in list types and list derivations, returning false
// when the item type cannot be determined.
func ListItemType(typ Type) (Type, bool) {
	return ListItemTypeWithResolver(typ, nil)
}
