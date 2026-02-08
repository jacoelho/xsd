package runtimecompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) listItemType(st *types.SimpleType) (types.Type, bool) {
	return r.listItemTypeFromType(st)
}

func (r *typeResolver) listItemTypeFromType(typ types.Type) (types.Type, bool) {
	return r.listItemTypeFromTypeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) listItemTypeFromTypeSeen(typ types.Type, seen map[types.Type]bool) (types.Type, bool) {
	if typ == nil {
		return nil, false
	}
	if seen[typ] {
		return nil, false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		if itemName, ok := types.BuiltinListItemTypeName(bt.Name().Local); ok {
			if item := types.GetBuiltin(itemName); item != nil {
				return item, true
			}
		}
		return nil, false
	}

	st, ok := types.AsSimpleType(typ)
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
		return r.listItemTypeFromTypeSeen(base, seen)
	}
	return nil, false
}

func (r *typeResolver) unionMemberTypes(st *types.SimpleType) []types.Type {
	return r.unionMemberTypesFromType(st)
}

func (r *typeResolver) unionMemberTypesFromType(typ types.Type) []types.Type {
	return r.unionMemberTypesFromTypeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) unionMemberTypesFromTypeSeen(typ types.Type, seen map[types.Type]bool) []types.Type {
	if typ == nil {
		return nil
	}
	if seen[typ] {
		return nil
	}
	seen[typ] = true
	defer delete(seen, typ)

	st, ok := types.AsSimpleType(typ)
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
		return r.unionMemberTypesFromTypeSeen(base, seen)
	}
	return nil
}
