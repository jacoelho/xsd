package analysis

import "github.com/jacoelho/xsd/internal/types"

func (b *builder) visitGlobalElement(decl *types.ElementDecl) error {
	if err := b.assignGlobalElement(decl); err != nil {
		return err
	}
	return b.visitElementNested(decl)
}

func (b *builder) visitGlobalType(name types.QName, typ types.Type) error {
	if err := b.assignGlobalType(name, typ); err != nil {
		return err
	}
	return b.visitTypeChildren(typ)
}

func (b *builder) visitGlobalAttribute(name types.QName, decl *types.AttributeDecl) error {
	if err := b.assignGlobalAttribute(name, decl); err != nil {
		return err
	}
	return b.visitAttributeType(decl)
}

func (b *builder) visitAttributeGroup(group *types.AttributeGroup) error {
	return b.visitAttributeDeclsWithIDs(group.Attributes)
}

func (b *builder) visitGroup(group *types.ModelGroup) error {
	return b.visitParticle(group)
}
