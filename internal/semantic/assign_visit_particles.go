package semantic

import "github.com/jacoelho/xsd/internal/model"

func (b *builder) visitElementNested(decl *model.ElementDecl) error {
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

func (b *builder) visitParticle(particle model.Particle) error {
	switch typed := particle.(type) {
	case *model.ElementDecl:
		if typed.IsReference {
			return nil
		}
		if err := b.assignLocalElement(typed); err != nil {
			return err
		}
		return b.visitElementNested(typed)
	case *model.ModelGroup:
		for _, child := range typed.Particles {
			if err := b.visitParticle(child); err != nil {
				return err
			}
		}
	case *model.GroupRef:
		return nil
	case *model.AnyElement:
		return nil
	}
	return nil
}

func (b *builder) visitTypeChildren(typ model.Type) error {
	switch typed := typ.(type) {
	case *model.ComplexType:
		return b.visitComplexType(typed)
	case *model.SimpleType:
		return b.visitSimpleType(typed)
	default:
		return nil
	}
}

func (b *builder) visitComplexType(ct *model.ComplexType) error {
	if ct == nil {
		return nil
	}
	switch content := ct.Content().(type) {
	case *model.ElementContent:
		if err := b.visitParticle(content.Particle); err != nil {
			return err
		}
	case *model.ComplexContent:
		if err := b.visitComplexContent(content); err != nil {
			return err
		}
	case *model.SimpleContent:
		if err := b.visitSimpleContent(content); err != nil {
			return err
		}
	case *model.EmptyContent:
		// no-op
	}

	if err := b.visitAttributeDecls(ct.Attributes()); err != nil {
		return err
	}

	return nil
}

func (b *builder) visitComplexContent(content *model.ComplexContent) error {
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

func (b *builder) visitSimpleContent(content *model.SimpleContent) error {
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

func (b *builder) visitSimpleContentRestriction(restr *model.Restriction) error {
	if restr == nil || restr.SimpleType == nil {
		return nil
	}
	if err := b.assignAnonymousType(restr.SimpleType); err != nil {
		return err
	}
	return b.visitTypeChildren(restr.SimpleType)
}
