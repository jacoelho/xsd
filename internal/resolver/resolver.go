package resolver

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

// Resolver resolves all QName references in a schema.
// Runs exactly once after parsing. Detects cycles during resolution.
type Resolver struct {
	schema *parser.Schema

	// Cycle detection during resolution (cleared after resolution)
	detector *CycleDetector[types.QName]

	// Pointer-based tracking for anonymous types (which have empty QNames) to
	// avoid false cycle matches while still detecting self-references.
	resolvingPtrs map[types.Type]bool
	resolvedPtrs  map[types.Type]bool
}

// NewResolver creates a new resolver for the given schema.
func NewResolver(schema *parser.Schema) *Resolver {
	return &Resolver{
		schema:        schema,
		detector:      NewCycleDetector[types.QName](),
		resolvingPtrs: make(map[types.Type]bool),
		resolvedPtrs:  make(map[types.Type]bool),
	}
}

// Resolve resolves all references in the schema.
// Returns an error if there are unresolvable references or invalid cycles.
func (r *Resolver) Resolve() error {
	// order matters: resolve in dependency order

	// 1. Simple types (only depend on built-ins or other simple types)
	for qname, typ := range r.schema.TypeDefs {
		if st, ok := typ.(*types.SimpleType); ok {
			if err := r.resolveSimpleType(qname, st); err != nil {
				return err
			}
		}
	}

	// 2. Complex types (may depend on simple types)
	for qname, typ := range r.schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			if err := r.resolveComplexType(qname, ct); err != nil {
				return err
			}
		}
	}

	// 3. Groups (reference types and other groups)
	for qname, grp := range r.schema.Groups {
		if err := r.resolveGroup(qname, grp); err != nil {
			return err
		}
	}

	// 4. Elements (reference types and groups)
	for qname, elem := range r.schema.ElementDecls {
		if err := r.resolveElement(qname, elem); err != nil {
			return err
		}
	}

	// 5. Attributes
	for _, attr := range r.schema.AttributeDecls {
		if err := r.resolveAttribute(attr); err != nil {
			return err
		}
	}

	// 6. Attribute groups
	for qname, ag := range r.schema.AttributeGroups {
		if err := r.resolveAttributeGroup(qname, ag); err != nil {
			return err
		}
	}

	return nil
}

func (r *Resolver) resolveSimpleType(qname types.QName, st *types.SimpleType) error {
	// for anonymous types (empty QName), use pointer-based tracking
	if qname.IsZero() {
		if r.resolvedPtrs[st] {
			return nil
		}
		if r.resolvingPtrs[st] {
			return fmt.Errorf("circular anonymous type definition")
		}
		r.resolvingPtrs[st] = true
		defer func() {
			delete(r.resolvingPtrs, st)
			r.resolvedPtrs[st] = true
		}()
		return r.doResolveSimpleType(qname, st)
	}

	if r.detector.IsVisited(qname) {
		return nil
	}

	return r.detector.WithScope(qname, func() error {
		return r.doResolveSimpleType(qname, st)
	})
}

func (r *Resolver) doResolveSimpleType(qname types.QName, st *types.SimpleType) error {
	if err := r.resolveSimpleTypeRestriction(qname, st); err != nil {
		return err
	}
	if err := r.resolveSimpleTypeList(qname, st); err != nil {
		return err
	}
	if err := r.resolveSimpleTypeUnion(qname, st); err != nil {
		return err
	}
	return nil
}

func (r *Resolver) resolveSimpleTypeRestriction(qname types.QName, st *types.SimpleType) error {
	if st.Restriction == nil || st.Restriction.Base.IsZero() {
		return nil
	}
	base, err := r.lookupType(st.Restriction.Base, st.QName)
	if err != nil {
		return fmt.Errorf("type %s: %w", qname, err)
	}
	st.ResolvedBase = base
	if baseST, ok := base.(*types.SimpleType); ok {
		if baseST.Variety() == types.UnionVariety && len(st.MemberTypes) == 0 {
			if len(baseST.MemberTypes) == 0 {
				baseQName := baseST.QName
				if baseQName.IsZero() {
					baseQName = st.Restriction.Base
				}
				if err := r.resolveUnionNamedMembers(baseQName, baseST); err != nil {
					return err
				}
				if err := r.resolveUnionInlineMembers(baseQName, baseST); err != nil {
					return err
				}
			}
			if len(baseST.MemberTypes) > 0 {
				st.MemberTypes = append([]types.Type(nil), baseST.MemberTypes...)
			}
		}
	}
	// inherit whiteSpace when this type keeps the default preserve behavior
	if st.WhiteSpace() == types.WhiteSpacePreserve && base != nil {
		st.SetWhiteSpace(base.WhiteSpace())
	}
	return nil
}

func (r *Resolver) resolveSimpleTypeList(qname types.QName, st *types.SimpleType) error {
	if st.List == nil {
		return nil
	}
	if st.List.InlineItemType != nil {
		if err := r.resolveSimpleType(st.List.InlineItemType.QName, st.List.InlineItemType); err != nil {
			return fmt.Errorf("type %s list inline item: %w", qname, err)
		}
		st.ItemType = st.List.InlineItemType
		if !st.WhiteSpaceExplicit() {
			st.SetWhiteSpace(types.WhiteSpaceCollapse)
		}
		return nil
	}
	if st.List.ItemType.IsZero() {
		if !st.WhiteSpaceExplicit() {
			st.SetWhiteSpace(types.WhiteSpaceCollapse)
		}
		return nil
	}
	item, err := r.lookupType(st.List.ItemType, st.QName)
	if err != nil {
		if allowMissingTypeReference(r.schema, st.List.ItemType) {
			st.ItemType = types.NewPlaceholderSimpleType(st.List.ItemType)
			if !st.WhiteSpaceExplicit() {
				st.SetWhiteSpace(types.WhiteSpaceCollapse)
			}
			return nil
		}
		return fmt.Errorf("type %s list item: %w", qname, err)
	}
	st.ItemType = item
	if !st.WhiteSpaceExplicit() {
		st.SetWhiteSpace(types.WhiteSpaceCollapse)
	}
	return nil
}

func (r *Resolver) resolveSimpleTypeUnion(qname types.QName, st *types.SimpleType) error {
	if st.Union == nil {
		return nil
	}
	if err := r.resolveUnionNamedMembers(qname, st); err != nil {
		return err
	}
	if err := r.resolveUnionInlineMembers(qname, st); err != nil {
		return err
	}
	return nil
}

func (r *Resolver) resolveUnionNamedMembers(qname types.QName, st *types.SimpleType) error {
	if len(st.Union.MemberTypes) == 0 {
		return nil
	}
	st.MemberTypes = make([]types.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
	for i, memberQName := range st.Union.MemberTypes {
		if r.detector.IsResolving(memberQName) {
			if member, ok := r.schema.TypeDefs[memberQName]; ok {
				st.MemberTypes = append(st.MemberTypes, member)
				continue
			}
		}
		member, err := r.lookupType(memberQName, st.QName)
		if err != nil {
			if allowMissingTypeReference(r.schema, memberQName) {
				st.MemberTypes = append(st.MemberTypes, types.NewPlaceholderSimpleType(memberQName))
				continue
			}
			return fmt.Errorf("type %s union member %d: %w", qname, i, err)
		}
		st.MemberTypes = append(st.MemberTypes, member)
	}
	return nil
}

func (r *Resolver) resolveUnionInlineMembers(qname types.QName, st *types.SimpleType) error {
	if len(st.Union.InlineTypes) == 0 {
		return nil
	}
	if st.MemberTypes == nil {
		st.MemberTypes = make([]types.Type, 0, len(st.Union.InlineTypes))
	}
	for i, inlineType := range st.Union.InlineTypes {
		// resolve inline type if it has restrictions
		if err := r.resolveSimpleType(inlineType.QName, inlineType); err != nil {
			return fmt.Errorf("type %s union inline member %d: %w", qname, i, err)
		}
		st.MemberTypes = append(st.MemberTypes, inlineType)
	}
	return nil
}

func (r *Resolver) resolveComplexType(qname types.QName, ct *types.ComplexType) error {
	// for anonymous types (empty QName), use pointer-based tracking
	if qname.IsZero() {
		if r.resolvedPtrs[ct] {
			return nil
		}
		if r.resolvingPtrs[ct] {
			return fmt.Errorf("circular anonymous type definition")
		}
		r.resolvingPtrs[ct] = true
		defer func() {
			delete(r.resolvingPtrs, ct)
			r.resolvedPtrs[ct] = true
		}()
		return r.doResolveComplexType(qname, ct)
	}

	if r.detector.IsVisited(qname) {
		return nil
	}
	return r.detector.WithScope(qname, func() error {
		return r.doResolveComplexType(qname, ct)
	})
}

func (r *Resolver) doResolveComplexType(qname types.QName, ct *types.ComplexType) error {
	if err := r.resolveComplexTypeBase(qname, ct); err != nil {
		return err
	}
	if err := r.resolveComplexTypeParticles(qname, ct); err != nil {
		return err
	}
	if err := r.resolveComplexTypeAttributes(qname, ct); err != nil {
		return err
	}
	return nil
}

func (r *Resolver) resolveComplexTypeBase(qname types.QName, ct *types.ComplexType) error {
	baseQName := r.getBaseQName(ct)
	if baseQName.IsZero() {
		return nil
	}
	base, err := r.lookupType(baseQName, ct.QName)
	if err != nil {
		return fmt.Errorf("type %s: %w", qname, err)
	}
	ct.ResolvedBase = base
	return nil
}

func (r *Resolver) resolveComplexTypeParticles(qname types.QName, ct *types.ComplexType) error {
	if err := r.resolveContentParticles(ct.Content()); err != nil {
		return fmt.Errorf("type %s content: %w", qname, err)
	}
	return nil
}

func (r *Resolver) resolveComplexTypeAttributes(qname types.QName, ct *types.ComplexType) error {
	if err := r.resolveAttributeGroupRefs(qname, ct.AttrGroups); err != nil {
		return err
	}
	if err := r.resolveAttributeDecls(ct.Attributes()); err != nil {
		return err
	}

	content := ct.Content()
	if content == nil {
		return nil
	}
	switch c := content.(type) {
	case *types.ComplexContent:
		if ext := c.ExtensionDef(); ext != nil {
			if err := r.resolveAttributeGroupRefs(qname, ext.AttrGroups); err != nil {
				return err
			}
			if err := r.resolveAttributeDecls(ext.Attributes); err != nil {
				return err
			}
		}
		if restr := c.RestrictionDef(); restr != nil {
			if err := r.resolveAttributeGroupRefs(qname, restr.AttrGroups); err != nil {
				return err
			}
			if err := r.resolveAttributeDecls(restr.Attributes); err != nil {
				return err
			}
		}
	case *types.SimpleContent:
		if ext := c.ExtensionDef(); ext != nil {
			if err := r.resolveAttributeGroupRefs(qname, ext.AttrGroups); err != nil {
				return err
			}
			if err := r.resolveAttributeDecls(ext.Attributes); err != nil {
				return err
			}
		}
		if restr := c.RestrictionDef(); restr != nil {
			if err := r.resolveAttributeGroupRefs(qname, restr.AttrGroups); err != nil {
				return err
			}
			if err := r.resolveAttributeDecls(restr.Attributes); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Resolver) resolveAttributeGroupRefs(qname types.QName, groups []types.QName) error {
	for _, agRef := range groups {
		ag, err := r.lookupAttributeGroup(agRef)
		if err != nil {
			return fmt.Errorf("type %s attribute group %s: %w", qname, agRef, err)
		}
		if err := r.resolveAttributeGroup(agRef, ag); err != nil {
			return fmt.Errorf("type %s attribute group %s: %w", qname, agRef, err)
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeDecls(attrs []*types.AttributeDecl) error {
	for _, attr := range attrs {
		if err := r.resolveAttributeType(attr); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) lookupType(qname, referrer types.QName) (types.Type, error) {
	if bt := types.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		if qname.Local == "anyType" {
			return types.NewAnyTypeComplexType(), nil
		}
		return bt, nil
	}

	// self-referencing types are circular definitions
	if qname == referrer {
		return nil, fmt.Errorf("circular type definition: %s references itself", qname)
	}

	// look up in schema
	typ, ok := r.schema.TypeDefs[qname]
	if !ok {
		return nil, fmt.Errorf("type %s not found", qname)
	}

	if r.detector.IsResolving(qname) {
		if referrer.IsZero() {
			// allow recursive references from anonymous contexts (content models).
			return typ, nil
		}
		return nil, fmt.Errorf("circular reference detected: %s", qname.String())
	}

	switch t := typ.(type) {
	case *types.SimpleType:
		if err := r.resolveSimpleType(qname, t); err != nil {
			return nil, err
		}
	case *types.ComplexType:
		if err := r.resolveComplexType(qname, t); err != nil {
			return nil, err
		}
	}

	return typ, nil
}

func (r *Resolver) getBaseQName(ct *types.ComplexType) types.QName {
	return ct.Content().BaseTypeQName()
}

func (r *Resolver) resolveGroup(qname types.QName, mg *types.ModelGroup) error {
	if r.detector.IsVisited(qname) {
		return nil
	}

	return r.detector.WithScope(qname, func() error {
		// resolve group particles (expand GroupRefs)
		return r.resolveParticles(mg.Particles)
	})
}

func (r *Resolver) resolveParticles(particles []types.Particle) error {
	// use iterative approach with work queue to avoid stack overflow
	// inline ModelGroups are tree-structured (no pointer cycles)
	// named groups (GroupRef) have cycle detection via r.detector
	queue := slices.Clone(particles)

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]

		switch particle := p.(type) {
		case *types.GroupRef:
			if err := r.resolveGroupRefParticle(particle); err != nil {
				return err
			}
		case *types.ModelGroup:
			queue = append(queue, particle.Particles...)
		case *types.ElementDecl:
			if err := r.resolveElementParticle(particle); err != nil {
				return err
			}
		case *types.AnyElement:
			// wildcards don't need resolution
		}
	}
	return nil
}

func (r *Resolver) resolveGroupRefParticle(ref *types.GroupRef) error {
	group, ok := r.schema.Groups[ref.RefQName]
	if !ok {
		return fmt.Errorf("group %s not found", ref.RefQName)
	}
	return r.resolveGroup(ref.RefQName, group)
}

func (r *Resolver) resolveElementParticle(elem *types.ElementDecl) error {
	if elem.IsReference || elem.Type == nil {
		return nil
	}
	return r.resolveElementType(elem, elem.Name, elementTypeOptions{
		simpleContext:  "element %s type: %w",
		complexContext: "element %s anonymous type: %w",
		allowResolving: true,
	})
}

func (r *Resolver) resolveElement(qname types.QName, elem *types.ElementDecl) error {
	// elements may have inline anonymous types - those are already attached
	// elements with type attribute need lookup
	if elem.Type == nil {
		// this shouldn't happen if parsing is correct
		return nil
	}
	return r.resolveElementType(elem, qname, elementTypeOptions{
		simpleContext:  "element %s type: %w",
		complexContext: "element %s type: %w",
	})
}

type elementTypeOptions struct {
	simpleContext  string
	complexContext string
	allowResolving bool
}

func (r *Resolver) resolveElementType(elem *types.ElementDecl, elemName types.QName, opts elementTypeOptions) error {
	switch t := elem.Type.(type) {
	case *types.SimpleType:
		if types.IsPlaceholderSimpleType(t) {
			// pass empty referrer because element type lookup is not type derivation.
			// self-reference detection is for types referencing themselves as base types,
			// not for elements with the same name as their type (which is valid).
			actualType, err := r.lookupType(t.QName, types.QName{})
			if err != nil {
				if allowMissingTypeReference(r.schema, t.QName) {
					return nil
				}
				return fmt.Errorf(opts.simpleContext, elemName, err)
			}
			elem.Type = actualType
			return nil
		}
		if err := r.resolveSimpleType(t.QName, t); err != nil {
			return fmt.Errorf(opts.simpleContext, elemName, err)
		}
	case *types.ComplexType:
		if opts.allowResolving && !t.QName.IsZero() && r.detector.IsResolving(t.QName) {
			return nil
		}
		if err := r.resolveComplexType(t.QName, t); err != nil {
			return fmt.Errorf(opts.complexContext, elemName, err)
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeGroup(qname types.QName, ag *types.AttributeGroup) error {
	if r.detector.IsVisited(qname) {
		return nil
	}

	return r.detector.WithScope(qname, func() error {
		return r.doResolveAttributeGroup(qname, ag)
	})
}

func (r *Resolver) doResolveAttributeGroup(qname types.QName, ag *types.AttributeGroup) error {

	for _, agRef := range ag.AttrGroups {
		nestedAG, err := r.lookupAttributeGroup(agRef)
		if err != nil {
			return fmt.Errorf("attribute group %s: nested group %s: %w", qname, agRef, err)
		}
		if err := r.resolveAttributeGroup(agRef, nestedAG); err != nil {
			return err
		}
	}

	for _, attr := range ag.Attributes {
		if err := r.resolveAttributeType(attr); err != nil {
			return err
		}
	}

	return nil
}

func (r *Resolver) resolveAttribute(attr *types.AttributeDecl) error {
	if attr == nil {
		return nil
	}
	return r.resolveAttributeType(attr)
}

func (r *Resolver) resolveAttributeType(attr *types.AttributeDecl) error {
	if attr == nil || attr.Type == nil || attr.IsReference {
		return nil
	}

	// re-link to the schema's canonical type definition if available
	if typeQName := attr.Type.Name(); !typeQName.IsZero() {
		if current, ok := r.schema.TypeDefs[typeQName]; ok && current != attr.Type {
			attr.Type = current
		}
	}

	if st, ok := attr.Type.(*types.SimpleType); ok {
		// if it's a placeholder (has QName but no content), resolve it
		if types.IsPlaceholderSimpleType(st) {
			actualType, err := r.lookupType(st.QName, types.QName{})
			if err != nil {
				return fmt.Errorf("attribute %s type: %w", attr.Name, err)
			}
			attr.Type = actualType
			return nil
		}
		if err := r.resolveSimpleType(st.QName, st); err != nil {
			return fmt.Errorf("attribute %s type: %w", attr.Name, err)
		}
	}

	return nil
}

func (r *Resolver) lookupAttributeGroup(qname types.QName) (*types.AttributeGroup, error) {
	ag, ok := r.schema.AttributeGroups[qname]
	if !ok {
		return nil, fmt.Errorf("attribute group %s not found", qname)
	}
	return ag, nil
}

func (r *Resolver) resolveContentParticles(content types.Content) error {
	return schemacheck.WalkContentParticles(content, func(particle types.Particle) error {
		return r.resolveParticles([]types.Particle{particle})
	})
}
