package semantic

import "github.com/jacoelho/xsd/internal/types"

func (b *builder) visitElementNested(decl *types.ElementDecl) error {
	if decl == nil || decl.IsReference || decl.Type == nil {
		return nil
	}
	if decl.Type.IsBuiltin() {
		return nil
	}
	if !decl.Type.Name().IsZero() {
		return nil
	}
	if err := b.assignAnonymousType(decl.Type); err != nil {
		return err
	}
	return b.visitTypeChildren(decl.Type)
}

func (b *builder) visitParticle(particle types.Particle) error {
	switch typed := particle.(type) {
	case *types.ElementDecl:
		if typed.IsReference {
			return nil
		}
		if err := b.assignLocalElement(typed); err != nil {
			return err
		}
		return b.visitElementNested(typed)
	case *types.ModelGroup:
		for _, child := range typed.Particles {
			if err := b.visitParticle(child); err != nil {
				return err
			}
		}
	case *types.GroupRef:
		return nil
	case *types.AnyElement:
		return nil
	}
	return nil
}

func (b *builder) visitTypeChildren(typ types.Type) error {
	switch typed := typ.(type) {
	case *types.ComplexType:
		return b.visitComplexType(typed)
	case *types.SimpleType:
		return b.visitSimpleType(typed)
	default:
		return nil
	}
}

func (b *builder) visitComplexType(ct *types.ComplexType) error {
	if ct == nil {
		return nil
	}
	switch content := ct.Content().(type) {
	case *types.ElementContent:
		if err := b.visitParticle(content.Particle); err != nil {
			return err
		}
	case *types.ComplexContent:
		if err := b.visitComplexContent(content); err != nil {
			return err
		}
	case *types.SimpleContent:
		if err := b.visitSimpleContent(content); err != nil {
			return err
		}
	case *types.EmptyContent:
		// no-op
	}

	if err := b.visitAttributeDecls(ct.Attributes()); err != nil {
		return err
	}

	return nil
}

func (b *builder) visitComplexContent(content *types.ComplexContent) error {
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := b.visitParticle(ext.Particle); err != nil {
			return err
		}
		if err := b.visitAttributeDecls(ext.Attributes); err != nil {
			return err
		}
		return nil
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := b.visitParticle(restr.Particle); err != nil {
			return err
		}
		if err := b.visitAttributeDecls(restr.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) visitSimpleContent(content *types.SimpleContent) error {
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := b.visitAttributeDecls(ext.Attributes); err != nil {
			return err
		}
		return nil
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := b.visitSimpleContentRestriction(restr); err != nil {
			return err
		}
		if err := b.visitAttributeDecls(restr.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) visitSimpleContentRestriction(restr *types.Restriction) error {
	if restr == nil || restr.SimpleType == nil {
		return nil
	}
	if err := b.assignAnonymousType(restr.SimpleType); err != nil {
		return err
	}
	return b.visitTypeChildren(restr.SimpleType)
}
