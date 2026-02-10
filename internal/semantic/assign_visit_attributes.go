package semantic

import "github.com/jacoelho/xsd/internal/model"

func (b *builder) visitAttributeDecls(attrs []*model.AttributeDecl) error {
	return b.visitAttributeDeclsWithAssigner(attrs, nil)
}

func (b *builder) visitAttributeDeclsWithIDs(attrs []*model.AttributeDecl) error {
	return b.visitAttributeDeclsWithAssigner(attrs, b.assignLocalAttribute)
}

func (b *builder) visitAttributeDeclsWithAssigner(attrs []*model.AttributeDecl, assign func(*model.AttributeDecl) error) error {
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		if assign != nil && !attr.IsReference {
			if err := assign(attr); err != nil {
				return err
			}
		}
		if err := b.visitAttributeType(attr); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) visitAttributeType(attr *model.AttributeDecl) error {
	if attr == nil || attr.IsReference || attr.Type == nil {
		return nil
	}
	if attr.Type.IsBuiltin() {
		return nil
	}
	if !attr.Type.Name().IsZero() {
		return nil
	}
	if err := b.assignAnonymousType(attr.Type); err != nil {
		return err
	}
	return b.visitTypeChildren(attr.Type)
}
