package schemair

import (
	"fmt"

	ast "github.com/jacoelho/xsd/internal/schemaast"
)

func (r *docResolver) validateImportVisibility() error {
	for i := range r.docs {
		doc := &r.docs[i]
		allowed := make(map[ast.NamespaceURI]bool, len(doc.Imports)+3)
		allowed[doc.TargetNamespace] = true
		allowed[ast.XSDNamespace] = true
		allowed[ast.XMLNamespace] = true
		for _, imp := range doc.Imports {
			allowed[imp.Namespace] = true
		}
		for j := range doc.Decls {
			if err := r.validateDeclImportVisibility(doc, allowed, &doc.Decls[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *docResolver) validateDeclImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, decl *ast.TopLevelDecl) error {
	switch decl.Kind {
	case ast.DeclSimpleType:
		return r.validateSimpleTypeImportVisibility(doc, allowed, decl.SimpleType)
	case ast.DeclComplexType:
		return r.validateComplexTypeImportVisibility(doc, allowed, decl.ComplexType)
	case ast.DeclElement:
		return r.validateElementImportVisibility(doc, allowed, decl.Element)
	case ast.DeclAttribute:
		return r.validateAttributeImportVisibility(doc, allowed, decl.Attribute)
	case ast.DeclGroup:
		if decl.Group == nil {
			return nil
		}
		if err := r.validateQNameImportVisibility(doc, allowed, decl.Group.Ref, "group ref"); err != nil {
			return err
		}
		return r.validateParticleImportVisibility(doc, allowed, decl.Group.Particle)
	case ast.DeclAttributeGroup:
		return r.validateAttributeGroupImportVisibility(doc, allowed, decl.AttributeGroup)
	default:
		return nil
	}
}

func (r *docResolver) validateSimpleTypeImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, decl *ast.SimpleTypeDecl) error {
	if decl == nil {
		return nil
	}
	if err := r.validateQNameImportVisibility(doc, allowed, decl.Base, "simpleType base"); err != nil {
		return err
	}
	if err := r.validateQNameImportVisibility(doc, allowed, decl.ItemType, "list itemType"); err != nil {
		return err
	}
	for _, member := range decl.MemberTypes {
		if err := r.validateQNameImportVisibility(doc, allowed, member, "union memberType"); err != nil {
			return err
		}
	}
	if err := r.validateSimpleTypeImportVisibility(doc, allowed, decl.InlineBase); err != nil {
		return err
	}
	if err := r.validateSimpleTypeImportVisibility(doc, allowed, decl.InlineItem); err != nil {
		return err
	}
	for i := range decl.InlineMembers {
		if err := r.validateSimpleTypeImportVisibility(doc, allowed, &decl.InlineMembers[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) validateComplexTypeImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, decl *ast.ComplexTypeDecl) error {
	if decl == nil {
		return nil
	}
	if err := r.validateQNameImportVisibility(doc, allowed, decl.Base, "complexType base"); err != nil {
		return err
	}
	if err := r.validateSimpleTypeImportVisibility(doc, allowed, decl.SimpleType); err != nil {
		return err
	}
	for _, group := range decl.AttributeGroups {
		if err := r.validateQNameImportVisibility(doc, allowed, group, "attributeGroup ref"); err != nil {
			return err
		}
	}
	for i := range decl.Attributes {
		if err := r.validateAttributeUseImportVisibility(doc, allowed, &decl.Attributes[i]); err != nil {
			return err
		}
	}
	return r.validateParticleImportVisibility(doc, allowed, decl.Particle)
}

func (r *docResolver) validateElementImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, decl *ast.ElementDecl) error {
	if decl == nil {
		return nil
	}
	if err := r.validateQNameImportVisibility(doc, allowed, decl.Ref, "element ref"); err != nil {
		return err
	}
	if err := r.validateTypeUseImportVisibility(doc, allowed, decl.Type); err != nil {
		return err
	}
	if err := r.validateQNameImportVisibility(doc, allowed, decl.SubstitutionGroup, "substitutionGroup"); err != nil {
		return err
	}
	for _, identity := range decl.Identity {
		if err := r.validateQNameImportVisibility(doc, allowed, identity.Refer, "keyref refer"); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) validateAttributeImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, decl *ast.AttributeDecl) error {
	if decl == nil {
		return nil
	}
	if err := r.validateQNameImportVisibility(doc, allowed, decl.Ref, "attribute ref"); err != nil {
		return err
	}
	return r.validateTypeUseImportVisibility(doc, allowed, decl.Type)
}

func (r *docResolver) validateTypeUseImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, typ ast.TypeUse) error {
	if err := r.validateQNameImportVisibility(doc, allowed, typ.Name, "type"); err != nil {
		return err
	}
	if err := r.validateSimpleTypeImportVisibility(doc, allowed, typ.Simple); err != nil {
		return err
	}
	return r.validateComplexTypeImportVisibility(doc, allowed, typ.Complex)
}

func (r *docResolver) validateAttributeUseImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, use *ast.AttributeUseDecl) error {
	if use == nil {
		return nil
	}
	if err := r.validateAttributeImportVisibility(doc, allowed, use.Attribute); err != nil {
		return err
	}
	return r.validateQNameImportVisibility(doc, allowed, use.AttributeGroup, "attributeGroup ref")
}

func (r *docResolver) validateAttributeGroupImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, decl *ast.AttributeGroupDecl) error {
	if decl == nil {
		return nil
	}
	if err := r.validateQNameImportVisibility(doc, allowed, decl.Ref, "attributeGroup ref"); err != nil {
		return err
	}
	for _, group := range decl.AttributeGroups {
		if err := r.validateQNameImportVisibility(doc, allowed, group, "attributeGroup ref"); err != nil {
			return err
		}
	}
	for i := range decl.Attributes {
		if err := r.validateAttributeUseImportVisibility(doc, allowed, &decl.Attributes[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) validateParticleImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, particle *ast.ParticleDecl) error {
	if particle == nil {
		return nil
	}
	if err := r.validateElementImportVisibility(doc, allowed, particle.Element); err != nil {
		return err
	}
	if err := r.validateQNameImportVisibility(doc, allowed, particle.GroupRef, "group ref"); err != nil {
		return err
	}
	for i := range particle.Children {
		if err := r.validateParticleImportVisibility(doc, allowed, &particle.Children[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *docResolver) validateQNameImportVisibility(doc *ast.SchemaDocument, allowed map[ast.NamespaceURI]bool, qname ast.QName, context string) error {
	if qname.IsZero() || allowed[qname.Namespace] {
		return nil
	}
	return fmt.Errorf("schema ir: %s %s references namespace %q not imported by schema %s", context, formatName(nameFromQName(qname)), qname.Namespace, doc.Location)
}
