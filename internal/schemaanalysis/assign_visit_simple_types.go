package schemaanalysis

import "github.com/jacoelho/xsd/internal/model"

func (b *builder) visitSimpleType(st *model.SimpleType) error {
	if st == nil {
		return nil
	}
	if st.Restriction != nil && st.Restriction.SimpleType != nil {
		if err := b.assignAnonymousType(st.Restriction.SimpleType); err != nil {
			return err
		}
		if err := b.visitTypeChildren(st.Restriction.SimpleType); err != nil {
			return err
		}
	}
	if st.List != nil && st.List.InlineItemType != nil {
		if err := b.assignAnonymousType(st.List.InlineItemType); err != nil {
			return err
		}
		if err := b.visitTypeChildren(st.List.InlineItemType); err != nil {
			return err
		}
	}
	if st.Union != nil {
		for _, inline := range st.Union.InlineTypes {
			if err := b.assignAnonymousType(inline); err != nil {
				return err
			}
			if err := b.visitTypeChildren(inline); err != nil {
				return err
			}
		}
	}
	return nil
}
