package semantic

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// ResolvedReferences records resolved references without mutating the parsed schema.
type ResolvedReferences struct {
	ElementRefs   map[*types.ElementDecl]ElemID
	AttributeRefs map[*types.AttributeDecl]AttrID
	GroupRefs     map[*types.GroupRef]*types.ModelGroup
}

type resolveState uint8

const (
	resolveUnseen resolveState = iota
	resolveResolving
	resolveResolved
)

// ResolveReferences validates and resolves QName references in the parsed schema.
func ResolveReferences(schema *parser.Schema, registry *Registry) (*ResolvedReferences, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if err := RequireResolved(schema); err != nil {
		return nil, err
	}
	if err := validateSchemaInput(schema); err != nil {
		return nil, err
	}

	resolver := &referenceResolver{
		schema:   schema,
		registry: registry,
		refs: &ResolvedReferences{
			ElementRefs:   make(map[*types.ElementDecl]ElemID),
			AttributeRefs: make(map[*types.AttributeDecl]AttrID),
			GroupRefs:     make(map[*types.GroupRef]*types.ModelGroup),
		},
		elementState:     make(map[*types.ElementDecl]resolveState),
		modelGroupState:  make(map[*types.ModelGroup]resolveState),
		simpleTypeState:  make(map[*types.SimpleType]resolveState),
		complexTypeState: make(map[*types.ComplexType]resolveState),
	}

	for _, decl := range schema.GlobalDecls {
		switch decl.Kind {
		case parser.GlobalDeclElement:
			declared := schema.ElementDecls[decl.Name]
			if declared == nil {
				return nil, fmt.Errorf("missing global element %s", decl.Name)
			}
			if err := resolver.resolveGlobalElement(declared); err != nil {
				return nil, err
			}
		case parser.GlobalDeclType:
			typeDef := schema.TypeDefs[decl.Name]
			if typeDef == nil {
				return nil, fmt.Errorf("missing global type %s", decl.Name)
			}
			if err := resolver.resolveType(typeDef); err != nil {
				return nil, fmt.Errorf("type %s: %w", decl.Name, err)
			}
		case parser.GlobalDeclAttribute:
			attr := schema.AttributeDecls[decl.Name]
			if attr == nil {
				return nil, fmt.Errorf("missing global attribute %s", decl.Name)
			}
			if err := resolver.resolveAttribute(attr); err != nil {
				return nil, fmt.Errorf("attribute %s: %w", decl.Name, err)
			}
		case parser.GlobalDeclAttributeGroup:
			group := schema.AttributeGroups[decl.Name]
			if group == nil {
				return nil, fmt.Errorf("missing attributeGroup %s", decl.Name)
			}
			if err := resolver.resolveAttributeGroup(decl.Name, group); err != nil {
				return nil, err
			}
		case parser.GlobalDeclGroup:
			group := schema.Groups[decl.Name]
			if group == nil {
				return nil, fmt.Errorf("missing group %s", decl.Name)
			}
			if err := resolver.resolveModelGroup(group); err != nil {
				return nil, err
			}
		case parser.GlobalDeclNotation:
			continue
		default:
			return nil, fmt.Errorf("unknown global declaration kind %d", decl.Kind)
		}
	}

	return resolver.refs, nil
}

type referenceResolver struct {
	schema           *parser.Schema
	registry         *Registry
	refs             *ResolvedReferences
	elementState     map[*types.ElementDecl]resolveState
	modelGroupState  map[*types.ModelGroup]resolveState
	simpleTypeState  map[*types.SimpleType]resolveState
	complexTypeState map[*types.ComplexType]resolveState
}

func (r *referenceResolver) resolveGlobalElement(decl *types.ElementDecl) error {
	if decl != nil && !decl.SubstitutionGroup.IsZero() {
		if _, ok := r.schema.ElementDecls[decl.SubstitutionGroup]; !ok {
			return fmt.Errorf("element %s substitutionGroup %s not found", decl.Name, decl.SubstitutionGroup)
		}
	}
	return r.resolveElement(decl)
}

func (r *referenceResolver) resolveElement(decl *types.ElementDecl) error {
	if decl == nil {
		return nil
	}
	switch r.elementState[decl] {
	case resolveResolving, resolveResolved:
		return nil
	}
	r.elementState[decl] = resolveResolving
	if decl.IsReference {
		if err := r.resolveElementReference(decl); err != nil {
			delete(r.elementState, decl)
			return err
		}
		r.elementState[decl] = resolveResolved
		return nil
	}
	if decl.Type == nil {
		r.elementState[decl] = resolveResolved
		return nil
	}
	if st, ok := decl.Type.(*types.SimpleType); ok && types.IsPlaceholderSimpleType(st) {
		if err := r.resolveTypeQName(st.QName); err != nil {
			delete(r.elementState, decl)
			return fmt.Errorf("element %s: %w", decl.Name, err)
		}
		r.elementState[decl] = resolveResolved
		return nil
	}
	if err := r.resolveType(decl.Type); err != nil {
		delete(r.elementState, decl)
		return fmt.Errorf("element %s: %w", decl.Name, err)
	}
	r.elementState[decl] = resolveResolved
	return nil
}

func (r *referenceResolver) resolveElementReference(decl *types.ElementDecl) error {
	target := r.schema.ElementDecls[decl.Name]
	if target == nil {
		return fmt.Errorf("element ref %s not found", decl.Name)
	}
	id, ok := r.registry.Elements[decl.Name]
	if !ok {
		return fmt.Errorf("element ref %s missing ID", decl.Name)
	}
	r.refs.ElementRefs[decl] = id
	return nil
}

func (r *referenceResolver) resolveAttribute(attr *types.AttributeDecl) error {
	if attr == nil {
		return nil
	}
	if attr.IsReference {
		return r.resolveAttributeReference(attr)
	}
	if attr.Type == nil {
		return nil
	}
	if st, ok := attr.Type.(*types.SimpleType); ok && types.IsPlaceholderSimpleType(st) {
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

func (r *referenceResolver) resolveAttributeReference(attr *types.AttributeDecl) error {
	target := r.schema.AttributeDecls[attr.Name]
	if target == nil {
		return fmt.Errorf("attribute ref %s not found", attr.Name)
	}
	id, ok := r.registry.Attributes[attr.Name]
	if !ok {
		return fmt.Errorf("attribute ref %s missing ID", attr.Name)
	}
	r.refs.AttributeRefs[attr] = id
	return nil
}

func (r *referenceResolver) resolveAttributeGroup(name types.QName, group *types.AttributeGroup) error {
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

func (r *referenceResolver) resolveModelGroup(group *types.ModelGroup) error {
	if group == nil {
		return nil
	}
	switch r.modelGroupState[group] {
	case resolveResolving, resolveResolved:
		return nil
	}
	r.modelGroupState[group] = resolveResolving
	for _, particle := range group.Particles {
		if err := r.resolveParticle(particle); err != nil {
			delete(r.modelGroupState, group)
			return err
		}
	}
	r.modelGroupState[group] = resolveResolved
	return nil
}

func (r *referenceResolver) resolveParticle(particle types.Particle) error {
	switch typed := particle.(type) {
	case *types.ElementDecl:
		return r.resolveElement(typed)
	case *types.ModelGroup:
		return r.resolveModelGroup(typed)
	case *types.GroupRef:
		return r.resolveGroupRef(typed)
	case *types.AnyElement:
		return nil
	}
	return nil
}

func (r *referenceResolver) resolveGroupRef(ref *types.GroupRef) error {
	group := r.schema.Groups[ref.RefQName]
	if group == nil {
		return fmt.Errorf("group ref %s not found", ref.RefQName)
	}
	r.refs.GroupRefs[ref] = group
	return nil
}

func (r *referenceResolver) resolveType(typ types.Type) error {
	if typ == nil || typ.IsBuiltin() {
		return nil
	}

	switch typed := typ.(type) {
	case *types.SimpleType:
		if types.IsPlaceholderSimpleType(typed) {
			return r.resolveTypeQName(typed.QName)
		}
		return r.resolveSimpleType(typed)
	case *types.ComplexType:
		return r.resolveComplexType(typed)
	default:
		return nil
	}
}

func (r *referenceResolver) resolveSimpleType(st *types.SimpleType) error {
	if st == nil {
		return nil
	}
	switch r.simpleTypeState[st] {
	case resolveResolving, resolveResolved:
		return nil
	}
	r.simpleTypeState[st] = resolveResolving
	if st.Restriction != nil {
		if err := r.resolveTypeQName(st.Restriction.Base); err != nil {
			delete(r.simpleTypeState, st)
			return err
		}
		if st.Restriction.SimpleType != nil {
			if err := r.resolveType(st.Restriction.SimpleType); err != nil {
				delete(r.simpleTypeState, st)
				return err
			}
		}
	}
	if st.List != nil {
		if st.List.InlineItemType != nil {
			if err := r.resolveType(st.List.InlineItemType); err != nil {
				delete(r.simpleTypeState, st)
				return err
			}
		}
		if !st.List.ItemType.IsZero() {
			if err := r.resolveTypeQName(st.List.ItemType); err != nil {
				delete(r.simpleTypeState, st)
				return err
			}
		}
	}
	if st.Union != nil {
		for _, member := range st.Union.MemberTypes {
			if err := r.resolveTypeQName(member); err != nil {
				delete(r.simpleTypeState, st)
				return err
			}
		}
		for _, inline := range st.Union.InlineTypes {
			if err := r.resolveType(inline); err != nil {
				delete(r.simpleTypeState, st)
				return err
			}
		}
	}
	r.simpleTypeState[st] = resolveResolved
	return nil
}

func (r *referenceResolver) resolveComplexType(ct *types.ComplexType) error {
	if ct == nil {
		return nil
	}
	switch r.complexTypeState[ct] {
	case resolveResolving, resolveResolved:
		return nil
	}
	r.complexTypeState[ct] = resolveResolving

	switch content := ct.Content().(type) {
	case *types.ElementContent:
		if err := r.resolveParticle(content.Particle); err != nil {
			delete(r.complexTypeState, ct)
			return err
		}
	case *types.SimpleContent:
		if err := r.resolveSimpleContent(content); err != nil {
			delete(r.complexTypeState, ct)
			return err
		}
	case *types.ComplexContent:
		if err := r.resolveComplexContent(content); err != nil {
			delete(r.complexTypeState, ct)
			return err
		}
	case *types.EmptyContent:
		// no-op
	}

	if err := r.resolveAttributes(ct.Attributes(), ct.AttrGroups); err != nil {
		delete(r.complexTypeState, ct)
		return err
	}
	r.complexTypeState[ct] = resolveResolved
	return nil
}

func (r *referenceResolver) resolveSimpleContent(content *types.SimpleContent) error {
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

func (r *referenceResolver) resolveComplexContent(content *types.ComplexContent) error {
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

func (r *referenceResolver) resolveAttributes(attrs []*types.AttributeDecl, groups []types.QName) error {
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

func (r *referenceResolver) resolveTypeQName(qname types.QName) error {
	if qname.IsZero() {
		return nil
	}
	if qname.Namespace == types.XSDNamespace {
		if types.GetBuiltin(types.TypeName(qname.Local)) == nil {
			return fmt.Errorf("type '%s' not found in XSD namespace", qname.Local)
		}
		return nil
	}
	if _, ok := r.schema.TypeDefs[qname]; ok {
		return nil
	}
	return fmt.Errorf("type %s not found", qname)
}
