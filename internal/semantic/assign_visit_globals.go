package semantic

import "github.com/jacoelho/xsd/internal/model"

func (b *builder) visitGlobalElement(decl *model.ElementDecl) error {
	if err := b.assignGlobalElement(decl); err != nil {
		return err
	}
	return b.visitElementNested(decl)
}

func (b *builder) visitGlobalType(name model.QName, typ model.Type) error {
	if err := b.assignGlobalType(name, typ); err != nil {
		return err
	}
	return b.visitTypeChildren(typ)
}

func (b *builder) visitGlobalAttribute(name model.QName, decl *model.AttributeDecl) error {
	if err := b.assignGlobalAttribute(name, decl); err != nil {
		return err
	}
	return b.visitAttributeType(decl)
}

func (b *builder) visitAttributeGroup(group *model.AttributeGroup) error {
	return b.visitAttributeDeclsWithIDs(group.Attributes)
}

func (b *builder) visitGroup(group *model.ModelGroup) error {
	return b.visitParticle(group)
}
