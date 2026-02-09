package validatorcompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) listItemTypeFromType(typ types.Type) (types.Type, bool) {
	seen := make(map[types.Type]bool)
	var walk func(types.Type) (types.Type, bool)
	walk = func(current types.Type) (types.Type, bool) {
		if current == nil {
			return nil, false
		}
		if seen[current] {
			return nil, false
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			if itemName, ok := types.BuiltinListItemTypeName(bt.Name().Local); ok {
				if item := types.GetBuiltin(itemName); item != nil {
					return item, true
				}
			}
			return nil, false
		}

		st, ok := types.AsSimpleType(current)
		if !ok {
			return nil, false
		}
		if r.variety(st) != types.ListVariety {
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
				if item := r.resolveQName(st.List.ItemType); item != nil {
					return item, true
				}
			}
		}
		if base := r.baseType(st); base != nil {
			return walk(base)
		}
		return nil, false
	}
	return walk(typ)
}

func (r *typeResolver) unionMemberTypesFromType(typ types.Type) []types.Type {
	seen := make(map[types.Type]bool)
	var walk func(types.Type) []types.Type
	walk = func(current types.Type) []types.Type {
		if current == nil {
			return nil
		}
		if seen[current] {
			return nil
		}
		seen[current] = true
		defer delete(seen, current)

		st, ok := types.AsSimpleType(current)
		if !ok {
			return nil
		}
		if r.variety(st) != types.UnionVariety {
			return nil
		}
		if len(st.MemberTypes) > 0 {
			return st.MemberTypes
		}
		if st.Union != nil {
			members := make([]types.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
			for _, qname := range st.Union.MemberTypes {
				if member := r.resolveQName(qname); member != nil {
					members = append(members, member)
				}
			}
			for _, inline := range st.Union.InlineTypes {
				members = append(members, inline)
			}
			if len(members) > 0 {
				return members
			}
		}
		if base := r.baseType(st); base != nil {
			return walk(base)
		}
		return nil
	}
	return walk(typ)
}
