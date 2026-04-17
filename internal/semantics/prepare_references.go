package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// ResolveReferences validates and resolves QName references in the parsed schema.
func ResolveReferences(schema *parser.Schema, registry *analysis.Registry) (*ResolvedReferences, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if err := requireResolved(schema); err != nil {
		return nil, err
	}
	if err := validatePreparedSchemaInput(schema); err != nil {
		return nil, err
	}

	resolver := newReferenceResolver(schema, registry)
	if err := resolver.resolveGlobalDeclarations(); err != nil {
		return nil, err
	}
	return resolver.refs, nil
}

type referenceResolver struct {
	schema           *parser.Schema
	registry         *analysis.Registry
	refs             *ResolvedReferences
	elementState     *Pointer[*model.ElementDecl]
	modelGroupState  *Pointer[*model.ModelGroup]
	simpleTypeState  *Pointer[*model.SimpleType]
	complexTypeState *Pointer[*model.ComplexType]
}

func newReferenceResolver(schema *parser.Schema, registry *analysis.Registry) *referenceResolver {
	return &referenceResolver{
		schema:   schema,
		registry: registry,
		refs: &ResolvedReferences{
			ElementRefs:   make(map[model.QName]analysis.ElemID),
			AttributeRefs: make(map[model.QName]analysis.AttrID),
			GroupRefs:     make(map[model.QName]model.QName),
		},
		elementState:     NewPointer[*model.ElementDecl](),
		modelGroupState:  NewPointer[*model.ModelGroup](),
		simpleTypeState:  NewPointer[*model.SimpleType](),
		complexTypeState: NewPointer[*model.ComplexType](),
	}
}

func (r *referenceResolver) resolveGlobalDeclarations() error {
	return parser.ForEachGlobalDecl(&r.schema.SchemaGraph, parser.GlobalDeclHandlers{
		Element: func(name model.QName, decl *model.ElementDecl) error {
			if decl == nil {
				return fmt.Errorf("missing global element %s", name)
			}
			return r.resolveGlobalElement(decl)
		},
		Type: func(name model.QName, typ model.Type) error {
			if typ == nil {
				return fmt.Errorf("missing global type %s", name)
			}
			if err := r.resolveType(typ); err != nil {
				return fmt.Errorf("type %s: %w", name, err)
			}
			return nil
		},
		Attribute: func(name model.QName, attr *model.AttributeDecl) error {
			if attr == nil {
				return fmt.Errorf("missing global attribute %s", name)
			}
			if err := r.resolveAttribute(attr); err != nil {
				return fmt.Errorf("attribute %s: %w", name, err)
			}
			return nil
		},
		AttributeGroup: func(name model.QName, group *model.AttributeGroup) error {
			if group == nil {
				return fmt.Errorf("missing attributeGroup %s", name)
			}
			return r.resolveAttributeGroup(name, group)
		},
		Group: func(name model.QName, group *model.ModelGroup) error {
			if group == nil {
				return fmt.Errorf("missing group %s", name)
			}
			return r.resolveModelGroup(group)
		},
		Notation: func(model.QName, *model.NotationDecl) error {
			return nil
		},
	})
}

func (r *referenceResolver) resolveType(typ model.Type) error {
	if typ == nil || typ.IsBuiltin() {
		return nil
	}

	switch typed := typ.(type) {
	case *model.SimpleType:
		if model.IsPlaceholderSimpleType(typed) {
			return r.resolveTypeQName(typed.QName)
		}
		return r.resolveSimpleType(typed)
	case *model.ComplexType:
		return r.resolveComplexType(typed)
	}
	return nil
}

func (r *referenceResolver) resolveSimpleType(st *model.SimpleType) error {
	if st == nil {
		return nil
	}
	return r.simpleTypeState.Resolve(st, nil, func() error {
		if st.Restriction != nil {
			if err := r.resolveTypeQName(st.Restriction.Base); err != nil {
				return err
			}
			if st.Restriction.SimpleType != nil {
				if err := r.resolveType(st.Restriction.SimpleType); err != nil {
					return err
				}
			}
		}
		if st.List != nil {
			if st.List.InlineItemType != nil {
				if err := r.resolveType(st.List.InlineItemType); err != nil {
					return err
				}
			}
			if !st.List.ItemType.IsZero() {
				if err := r.resolveTypeQName(st.List.ItemType); err != nil {
					return err
				}
			}
		}
		if st.Union != nil {
			for _, member := range st.Union.MemberTypes {
				if err := r.resolveTypeQName(member); err != nil {
					return err
				}
			}
			for _, inline := range st.Union.InlineTypes {
				if err := r.resolveType(inline); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *referenceResolver) resolveComplexType(ct *model.ComplexType) error {
	if ct == nil {
		return nil
	}
	return r.complexTypeState.Resolve(ct, nil, func() error {
		switch content := ct.Content().(type) {
		case *model.ElementContent:
			if err := r.resolveParticle(content.Particle); err != nil {
				return err
			}
		case *model.SimpleContent:
			if err := r.resolveSimpleContent(content); err != nil {
				return err
			}
		case *model.ComplexContent:
			if err := r.resolveComplexContent(content); err != nil {
				return err
			}
		case *model.EmptyContent:
		}

		return r.resolveAttributes(ct.Attributes(), ct.AttrGroups)
	})
}

func (r *referenceResolver) resolveSimpleContent(content *model.SimpleContent) error {
	if content == nil {
		return nil
	}
	if !content.Base.IsZero() {
		if err := r.resolveTypeQName(content.Base); err != nil {
			return err
		}
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := r.resolveTypeQName(ext.Base); err != nil {
			return err
		}
		return r.resolveAttributes(ext.Attributes, ext.AttrGroups)
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := r.resolveTypeQName(restr.Base); err != nil {
			return err
		}
		if restr.SimpleType != nil {
			if err := r.resolveType(restr.SimpleType); err != nil {
				return err
			}
		}
		return r.resolveAttributes(restr.Attributes, restr.AttrGroups)
	}
	return nil
}

func (r *referenceResolver) resolveComplexContent(content *model.ComplexContent) error {
	if content == nil {
		return nil
	}
	if !content.Base.IsZero() {
		if err := r.resolveTypeQName(content.Base); err != nil {
			return err
		}
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := r.resolveTypeQName(ext.Base); err != nil {
			return err
		}
		if err := r.resolveParticle(ext.Particle); err != nil {
			return err
		}
		return r.resolveAttributes(ext.Attributes, ext.AttrGroups)
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := r.resolveTypeQName(restr.Base); err != nil {
			return err
		}
		if err := r.resolveParticle(restr.Particle); err != nil {
			return err
		}
		return r.resolveAttributes(restr.Attributes, restr.AttrGroups)
	}
	return nil
}

func (r *referenceResolver) resolveTypeQName(qname model.QName) error {
	return parser.ValidateTypeQName(r.schema, qname)
}

func (r *referenceResolver) resolveGlobalElement(decl *model.ElementDecl) error {
	if decl != nil && !decl.SubstitutionGroup.IsZero() {
		if _, ok := r.schema.ElementDecls[decl.SubstitutionGroup]; !ok {
			return fmt.Errorf("element %s substitutionGroup %s not found", decl.Name, decl.SubstitutionGroup)
		}
	}
	return r.resolveElement(decl)
}

func (r *referenceResolver) resolveElement(decl *model.ElementDecl) error {
	if decl == nil {
		return nil
	}
	return r.elementState.Resolve(decl, nil, func() error {
		if decl.IsReference {
			return r.resolveElementReference(decl)
		}
		if decl.Type == nil {
			return nil
		}
		if st, ok := decl.Type.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(st) {
			if err := r.resolveTypeQName(st.QName); err != nil {
				return fmt.Errorf("element %s: %w", decl.Name, err)
			}
			return nil
		}
		if err := r.resolveType(decl.Type); err != nil {
			return fmt.Errorf("element %s: %w", decl.Name, err)
		}
		return nil
	})
}

func (r *referenceResolver) resolveElementReference(decl *model.ElementDecl) error {
	target := r.schema.ElementDecls[decl.Name]
	if target == nil {
		return fmt.Errorf("element ref %s not found", decl.Name)
	}
	id, ok := r.registry.Elements[decl.Name]
	if !ok {
		return fmt.Errorf("element ref %s missing ID", decl.Name)
	}
	if existing, exists := r.refs.ElementRefs[decl.Name]; exists && existing != id {
		return fmt.Errorf("element ref %s resolved inconsistently (%d != %d)", decl.Name, existing, id)
	}
	r.refs.ElementRefs[decl.Name] = id
	return nil
}

func (r *referenceResolver) resolveModelGroup(group *model.ModelGroup) error {
	if group == nil {
		return nil
	}
	return r.modelGroupState.Resolve(group, nil, func() error {
		for _, particle := range group.Particles {
			if err := r.resolveParticle(particle); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *referenceResolver) resolveParticle(particle model.Particle) error {
	switch typed := particle.(type) {
	case *model.ElementDecl:
		return r.resolveElement(typed)
	case *model.ModelGroup:
		return r.resolveModelGroup(typed)
	case *model.GroupRef:
		return r.resolveGroupRef(typed)
	}
	return nil
}

func (r *referenceResolver) resolveGroupRef(ref *model.GroupRef) error {
	group := r.schema.Groups[ref.RefQName]
	if group == nil {
		return fmt.Errorf("group ref %s not found", ref.RefQName)
	}
	targetName := ref.RefQName
	if existing, exists := r.refs.GroupRefs[ref.RefQName]; exists && existing != targetName {
		return fmt.Errorf("group ref %s resolved inconsistently (%s != %s)", ref.RefQName, existing, targetName)
	}
	r.refs.GroupRefs[ref.RefQName] = targetName
	return nil
}

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
	if err := WalkAttributeGroups(r.schema, group.AttrGroups, MissingError, nil); err != nil {
		var missingErr AttributeGroupMissingError
		if errors.As(err, &missingErr) {
			return fmt.Errorf("attributeGroup %s: nested group %s not found", name, missingErr.QName)
		}
		return err
	}
	for _, attr := range group.Attributes {
		if err := r.resolveAttribute(attr); err != nil {
			return fmt.Errorf("attributeGroup %s: %w", name, err)
		}
	}
	return nil
}

func (r *referenceResolver) resolveAttributes(attrs []*model.AttributeDecl, groups []model.QName) error {
	if err := WalkAttributeGroups(r.schema, groups, MissingError, nil); err != nil {
		var missingErr AttributeGroupMissingError
		if errors.As(err, &missingErr) {
			return fmt.Errorf("attributeGroup ref %s not found", missingErr.QName)
		}
		return err
	}
	for _, attr := range attrs {
		if err := r.resolveAttribute(attr); err != nil {
			return err
		}
	}
	return nil
}
