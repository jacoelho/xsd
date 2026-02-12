package semanticresolve

import (
	"fmt"

	model "github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveSimpleTypeList(qname model.QName, st *model.SimpleType) error {
	if st.List == nil {
		return nil
	}
	if st.List.InlineItemType != nil {
		if err := r.resolveSimpleType(st.List.InlineItemType.QName, st.List.InlineItemType); err != nil {
			return fmt.Errorf("type %s list inline item: %w", qname, err)
		}
		st.ItemType = st.List.InlineItemType
		if !st.WhiteSpaceExplicit() {
			st.SetWhiteSpace(model.WhiteSpaceCollapse)
		}
		return nil
	}
	if st.List.ItemType.IsZero() {
		if !st.WhiteSpaceExplicit() {
			st.SetWhiteSpace(model.WhiteSpaceCollapse)
		}
		return nil
	}
	item, err := r.lookupType(st.List.ItemType, st.QName)
	if err != nil {
		return fmt.Errorf("type %s list item: %w", qname, err)
	}
	st.ItemType = item
	if !st.WhiteSpaceExplicit() {
		st.SetWhiteSpace(model.WhiteSpaceCollapse)
	}
	return nil
}
