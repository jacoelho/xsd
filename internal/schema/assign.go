package schema

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// AssignIDs walks the parsed schema in deterministic order and assigns IDs.
func AssignIDs(schema *parser.Schema) (*Registry, error) {
	if err := RequireResolved(schema); err != nil {
		return nil, err
	}
	if err := validateSchemaInput(schema); err != nil {
		return nil, err
	}

	b := &builder{
		schema:   schema,
		registry: newRegistry(),
		typeIDs:  make(map[types.Type]TypeID),
		nextType: 1,
		nextElem: 1,
		nextAttr: 1,
	}

	for _, decl := range schema.GlobalDecls {
		switch decl.Kind {
		case parser.GlobalDeclElement:
			declared := schema.ElementDecls[decl.Name]
			if declared == nil {
				return nil, fmt.Errorf("missing global element %s", decl.Name)
			}
			if err := b.visitGlobalElement(declared); err != nil {
				return nil, err
			}
		case parser.GlobalDeclType:
			typeDef := schema.TypeDefs[decl.Name]
			if typeDef == nil {
				return nil, fmt.Errorf("missing global type %s", decl.Name)
			}
			if err := b.visitGlobalType(decl.Name, typeDef); err != nil {
				return nil, err
			}
		case parser.GlobalDeclAttribute:
			attr := schema.AttributeDecls[decl.Name]
			if attr == nil {
				return nil, fmt.Errorf("missing global attribute %s", decl.Name)
			}
			if err := b.visitGlobalAttribute(decl.Name, attr); err != nil {
				return nil, err
			}
		case parser.GlobalDeclAttributeGroup:
			group := schema.AttributeGroups[decl.Name]
			if group == nil {
				return nil, fmt.Errorf("missing attributeGroup %s", decl.Name)
			}
			if err := b.visitAttributeGroup(group); err != nil {
				return nil, err
			}
		case parser.GlobalDeclGroup:
			group := schema.Groups[decl.Name]
			if group == nil {
				return nil, fmt.Errorf("missing group %s", decl.Name)
			}
			if err := b.visitGroup(group); err != nil {
				return nil, err
			}
		case parser.GlobalDeclNotation:
			continue
		default:
			return nil, fmt.Errorf("unknown global declaration kind %d", decl.Kind)
		}
	}

	return b.registry, nil
}

func hasGlobalDecls(schema *parser.Schema) bool {
	return len(schema.ElementDecls) > 0 || len(schema.TypeDefs) > 0 ||
		len(schema.AttributeDecls) > 0 || len(schema.AttributeGroups) > 0 ||
		len(schema.Groups) > 0 || len(schema.NotationDecls) > 0
}

type builder struct {
	schema   *parser.Schema
	registry *Registry
	typeIDs  map[types.Type]TypeID
	nextType TypeID
	nextElem ElemID
	nextAttr AttrID
}

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
		if restr.SimpleType != nil {
			if err := b.assignAnonymousType(restr.SimpleType); err != nil {
				return err
			}
			if err := b.visitTypeChildren(restr.SimpleType); err != nil {
				return err
			}
		}
		if err := b.visitAttributeDecls(restr.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func (b *builder) visitSimpleType(st *types.SimpleType) error {
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

func (b *builder) visitAttributeDecls(attrs []*types.AttributeDecl) error {
	return b.visitAttributeDeclsWithAssigner(attrs, nil)
}

func (b *builder) visitAttributeDeclsWithIDs(attrs []*types.AttributeDecl) error {
	return b.visitAttributeDeclsWithAssigner(attrs, b.assignLocalAttribute)
}

func (b *builder) visitAttributeDeclsWithAssigner(attrs []*types.AttributeDecl, assign func(*types.AttributeDecl) error) error {
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

func (b *builder) visitAttributeType(attr *types.AttributeDecl) error {
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

func (b *builder) assignGlobalType(name types.QName, typ types.Type) error {
	if typ == nil {
		return fmt.Errorf("global type %s is nil", name)
	}
	if _, exists := b.registry.Types[name]; exists {
		return fmt.Errorf("duplicate global type %s", name)
	}
	id := b.nextType
	b.nextType++
	b.registry.Types[name] = id
	b.typeIDs[typ] = id
	b.registry.TypeOrder = append(b.registry.TypeOrder, TypeEntry{
		ID:     id,
		QName:  name,
		Type:   typ,
		Global: true,
	})
	return nil
}

func (b *builder) assignAnonymousType(typ types.Type) error {
	if typ == nil {
		return nil
	}
	if !typ.Name().IsZero() {
		return fmt.Errorf("expected anonymous type, got %s", typ.Name())
	}
	if existing, ok := b.typeIDs[typ]; ok {
		if _, seen := b.registry.AnonymousTypes[typ]; !seen {
			b.registry.AnonymousTypes[typ] = existing
		}
		return nil
	}
	id := b.nextType
	b.nextType++
	b.typeIDs[typ] = id
	b.registry.AnonymousTypes[typ] = id
	b.registry.TypeOrder = append(b.registry.TypeOrder, TypeEntry{
		ID:     id,
		QName:  types.QName{},
		Type:   typ,
		Global: false,
	})
	return nil
}

func (b *builder) assignGlobalElement(decl *types.ElementDecl) error {
	if decl == nil {
		return fmt.Errorf("global element is nil")
	}
	if _, exists := b.registry.Elements[decl.Name]; exists {
		return fmt.Errorf("duplicate global element %s", decl.Name)
	}
	id := b.nextElem
	b.nextElem++
	b.registry.Elements[decl.Name] = id
	b.registry.ElementOrder = append(b.registry.ElementOrder, ElementEntry{
		ID:     id,
		QName:  decl.Name,
		Decl:   decl,
		Global: true,
	})
	return nil
}

func (b *builder) assignLocalElement(decl *types.ElementDecl) error {
	if decl == nil {
		return fmt.Errorf("local element is nil")
	}
	if _, exists := b.registry.LocalElements[decl]; exists {
		return nil
	}
	id := b.nextElem
	b.nextElem++
	b.registry.LocalElements[decl] = id
	b.registry.ElementOrder = append(b.registry.ElementOrder, ElementEntry{
		ID:     id,
		QName:  decl.Name,
		Decl:   decl,
		Global: false,
	})
	return nil
}

func (b *builder) assignGlobalAttribute(name types.QName, decl *types.AttributeDecl) error {
	if decl == nil {
		return fmt.Errorf("global attribute %s is nil", name)
	}
	if _, exists := b.registry.Attributes[name]; exists {
		return fmt.Errorf("duplicate global attribute %s", name)
	}
	id := b.nextAttr
	b.nextAttr++
	b.registry.Attributes[name] = id
	b.registry.AttributeOrder = append(b.registry.AttributeOrder, AttributeEntry{
		ID:     id,
		QName:  name,
		Decl:   decl,
		Global: true,
	})
	return nil
}

func (b *builder) assignLocalAttribute(decl *types.AttributeDecl) error {
	if decl == nil {
		return fmt.Errorf("local attribute is nil")
	}
	if _, exists := b.registry.LocalAttributes[decl]; exists {
		return nil
	}
	id := b.nextAttr
	b.nextAttr++
	b.registry.LocalAttributes[decl] = id
	b.registry.AttributeOrder = append(b.registry.AttributeOrder, AttributeEntry{
		ID:     id,
		QName:  decl.Name,
		Decl:   decl,
		Global: false,
	})
	return nil
}
