package semantic

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func (r *referenceResolver) resolveAttribute(attr *model.AttributeDecl) error {
	if attr == nil {
		return nil
	}
	if attr.IsReference {
		return r.resolveAttributeReference(attr)
	}
	if attr.Type == nil {
		return nil
	}
	if st, ok := attr.Type.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(st) {
		if err := r.resolveTypeQName(st.QName); err != nil {
			return fmt.Errorf("attribute %s: %w", attr.Name, err)
		}
		return nil
	}
	if err := r.resolveType(attr.Type); err != nil {
		return fmt.Errorf("attribute %s: %w", attr.Name, err)
	}
	return nil
}

func (r *referenceResolver) resolveAttributeReference(attr *model.AttributeDecl) error {
	target := r.schema.AttributeDecls[attr.Name]
	if target == nil {
		return fmt.Errorf("attribute ref %s not found", attr.Name)
	}
	id, ok := r.registry.Attributes[attr.Name]
	if !ok {
		return fmt.Errorf("attribute ref %s missing ID", attr.Name)
	}
	if existing, exists := r.refs.AttributeRefs[attr.Name]; exists && existing != id {
		return fmt.Errorf("attribute ref %s resolved inconsistently (%d != %d)", attr.Name, existing, id)
	}
	r.refs.AttributeRefs[attr.Name] = id
	return nil
}

func (r *referenceResolver) resolveAttributeGroup(name model.QName, group *model.AttributeGroup) error {
	for _, ref := range group.AttrGroups {
		if _, ok := r.schema.AttributeGroups[ref]; !ok {
			return fmt.Errorf("attributeGroup %s: nested group %s not found", name, ref)
		}
	}
	for _, attr := range group.Attributes {
		if err := r.resolveAttribute(attr); err != nil {
			return fmt.Errorf("attributeGroup %s: %w", name, err)
		}
	}
	return nil
}

func (r *referenceResolver) resolveAttributes(attrs []*model.AttributeDecl, groups []model.QName) error {
	for _, ref := range groups {
		if _, ok := r.schema.AttributeGroups[ref]; !ok {
			return fmt.Errorf("attributeGroup ref %s not found", ref)
		}
	}
	for _, attr := range attrs {
		if err := r.resolveAttribute(attr); err != nil {
			return err
		}
	}
	return nil
}
