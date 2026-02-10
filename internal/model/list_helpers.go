package model

// ListItemType returns the item type for list simple types.
// It resolves built-in list types and list derivations, returning false
// when the item type cannot be determined.
func ListItemType(typ Type) (Type, bool) {
	visited := make(map[Type]bool)
	var visit func(current Type) (Type, bool)
	visit = func(current Type) (Type, bool) {
		if current == nil {
			return nil, false
		}
		if visited[current] {
			return nil, false
		}
		visited[current] = true
		defer delete(visited, current)

		if st, ok := AsSimpleType(current); ok {
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
				if itemType, ok := visit(st.ResolvedBase); ok {
					return itemType, true
				}
			}
			if st.Restriction != nil && !st.Restriction.Base.IsZero() {
				if base := GetBuiltinNS(st.Restriction.Base.Namespace, st.Restriction.Base.Local); base != nil {
					if itemType, ok := visit(base); ok {
						return itemType, true
					}
				}
			}
			return nil, false
		}

		if bt, ok := AsBuiltinType(current); ok {
			if itemName, ok := builtinListItemTypeName(bt.Name().Local); ok {
				if item := GetBuiltin(itemName); item != nil {
					return item, true
				}
			}
		}

		return nil, false
	}
	return visit(typ)
}
