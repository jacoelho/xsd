package types

// ListItemType returns the item type for list simple types.
// It resolves built-in list types and list derivations, returning false
// when the item type cannot be determined.
func ListItemType(typ Type) (Type, bool) {
	return listItemType(typ, make(map[Type]bool))
}

func listItemType(typ Type, visited map[Type]bool) (Type, bool) {
	if typ == nil {
		return nil, false
	}
	if visited[typ] {
		return nil, false
	}
	visited[typ] = true
	defer delete(visited, typ)

	if st, ok := AsSimpleType(typ); ok {
		if st.Variety() != ListVariety {
			return nil, false
		}
		if st.ItemType != nil {
			return st.ItemType, true
		}
		if st.List != nil && !st.List.ItemType.IsZero() {
			if builtin := GetBuiltinNS(st.List.ItemType.Namespace, st.List.ItemType.Local); builtin != nil {
				return builtin, true
			}
		}
		if st.ResolvedBase != nil {
			if itemType, ok := listItemType(st.ResolvedBase, visited); ok {
				return itemType, true
			}
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			if base := GetBuiltinNS(st.Restriction.Base.Namespace, st.Restriction.Base.Local); base != nil {
				if itemType, ok := listItemType(base, visited); ok {
					return itemType, true
				}
			}
		}
		return nil, false
	}

	if bt, ok := AsBuiltinType(typ); ok {
		if itemName, ok := builtinListItemTypeName(bt.Name().Local); ok {
			if item := GetBuiltin(itemName); item != nil {
				return item, true
			}
		}
	}

	return nil, false
}
