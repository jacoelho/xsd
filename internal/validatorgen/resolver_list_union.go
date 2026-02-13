package validatorgen

import (
	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/typewalk"
)

func (r *typeResolver) listItemTypeFromType(typ model.Type) (model.Type, bool) {
	var (
		item  model.Type
		found bool
	)
	typewalk.Walk(typ, r.nextType, func(current model.Type) bool {
		if bt := builtinForType(current); bt != nil {
			if itemName, ok := builtins.BuiltinListItemTypeName(bt.Name().Local); ok {
				item = builtins.Get(itemName)
				found = item != nil
			}
			return false
		}

		st, ok := model.AsSimpleType(current)
		if !ok {
			found = false
			return false
		}
		if r.variety(st) != model.ListVariety {
			found = false
			return false
		}
		if st.ItemType != nil {
			item = st.ItemType
			found = true
			return false
		}
		if st.List != nil {
			if st.List.InlineItemType != nil {
				item = st.List.InlineItemType
				found = true
				return false
			}
			if !st.List.ItemType.IsZero() {
				if resolved := r.resolveQName(st.List.ItemType); resolved != nil {
					item = resolved
					found = true
				}
				return false
			}
		}
		return true
	})
	return item, found
}

func (r *typeResolver) unionMemberTypesFromType(typ model.Type) []model.Type {
	var members []model.Type
	typewalk.Walk(typ, r.nextType, func(current model.Type) bool {
		st, ok := model.AsSimpleType(current)
		if !ok {
			members = nil
			return false
		}
		if r.variety(st) != model.UnionVariety {
			members = nil
			return false
		}
		if len(st.MemberTypes) > 0 {
			members = st.MemberTypes
			return false
		}
		if st.Union != nil {
			resolved := make([]model.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
			for _, qname := range st.Union.MemberTypes {
				if member := r.resolveQName(qname); member != nil {
					resolved = append(resolved, member)
				}
			}
			for _, inline := range st.Union.InlineTypes {
				resolved = append(resolved, inline)
			}
			if len(resolved) > 0 {
				members = resolved
			}
			return false
		}
		return true
	})
	return members
}
