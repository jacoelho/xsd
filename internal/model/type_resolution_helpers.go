package model

// ListItemTypeWithResolver returns the item type for list simple types.
// The resolver can resolve non-builtin QNames when list definitions are
// referenced through named base types.
func ListItemTypeWithResolver(typ Type, resolve func(QName) Type) (Type, bool) {
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

		if bt, ok := AsBuiltinType(current); ok {
			if bt == nil {
				return nil, false
			}
			if itemName, ok := builtinListItemTypeName(bt.Name().Local); ok {
				if item := GetBuiltin(itemName); item != nil {
					return item, true
				}
			}
			return nil, false
		}

		st, ok := AsSimpleType(current)
		if !ok || st == nil {
			return nil, false
		}

		if st.ItemType != nil {
			return st.ItemType, true
		}
		if st.List != nil {
			if st.List.InlineItemType != nil {
				return st.List.InlineItemType, true
			}
			if !st.List.ItemType.IsZero() {
				if item := resolveQNameType(st.List.ItemType, resolve); item != nil {
					return item, true
				}
			}
		}
		if st.ResolvedBase != nil {
			if itemType, ok := visit(st.ResolvedBase); ok {
				return itemType, true
			}
		}
		if st.Restriction != nil {
			if st.Restriction.SimpleType != nil {
				if itemType, ok := visit(st.Restriction.SimpleType); ok {
					return itemType, true
				}
			}
			if !st.Restriction.Base.IsZero() {
				if base := resolveQNameType(st.Restriction.Base, resolve); base != nil {
					if itemType, ok := visit(base); ok {
						return itemType, true
					}
				}
			}
		}
		return nil, false
	}
	return visit(typ)
}

// UnionMemberTypesWithResolver returns flattened member types for union simple types.
// The resolver can resolve non-builtin QNames from union memberTypes attributes
// and named restriction bases.
func UnionMemberTypesWithResolver(typ Type, resolve func(QName) Type) []Type {
	visited := make(map[Type]bool)
	var visit func(current Type) []Type
	visit = func(current Type) []Type {
		if current == nil {
			return nil
		}
		if visited[current] {
			return nil
		}
		visited[current] = true
		defer delete(visited, current)

		st, ok := AsSimpleType(current)
		if !ok || st == nil {
			return nil
		}
		if len(st.MemberTypes) > 0 {
			return st.MemberTypes
		}
		if st.Union != nil {
			memberTypes := make([]Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
			for _, inline := range st.Union.InlineTypes {
				memberTypes = append(memberTypes, inline)
			}
			for _, memberQName := range st.Union.MemberTypes {
				if member := resolveQNameType(memberQName, resolve); member != nil {
					memberTypes = append(memberTypes, member)
				}
			}
			if len(memberTypes) > 0 {
				return memberTypes
			}
		}
		if st.ResolvedBase != nil {
			if members := visit(st.ResolvedBase); len(members) > 0 {
				return members
			}
		}
		if st.Restriction != nil {
			if st.Restriction.SimpleType != nil {
				if members := visit(st.Restriction.SimpleType); len(members) > 0 {
					return members
				}
			}
			if !st.Restriction.Base.IsZero() {
				if base := resolveQNameType(st.Restriction.Base, resolve); base != nil {
					if members := visit(base); len(members) > 0 {
						return members
					}
				}
			}
		}
		return nil
	}
	return visit(typ)
}

func resolveQNameType(name QName, resolve func(QName) Type) Type {
	if name.IsZero() {
		return nil
	}
	if resolve != nil {
		if resolved := resolve(name); !isNilType(resolved) {
			return resolved
		}
	}
	if builtin := GetBuiltinNS(name.Namespace, name.Local); !isNilType(builtin) {
		return builtin
	}
	return nil
}
