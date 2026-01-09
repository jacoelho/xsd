package resolver

import (
	"fmt"
	"slices"

	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validation"
)

// Resolver resolves all QName references in a schema.
// Runs exactly once after parsing. Detects cycles during resolution.
type Resolver struct {
	schema *schema.Schema

	// Cycle detection during resolution (cleared after resolution)
	detector *CycleDetector[types.QName]

	// Pointer-based tracking for anonymous types (which have empty QNames)
	resolvingPtrs map[types.Type]bool
	resolvedPtrs  map[types.Type]bool
}

// NewResolver creates a new resolver for the given schema.
func NewResolver(schema *schema.Schema) *Resolver {
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
	if r.detector.IsVisited(qname) {
		return nil
	}

	return r.detector.WithScope(qname, func() error {
		return r.doResolveSimpleType(qname, st)
	})
}

func (r *Resolver) doResolveSimpleType(qname types.QName, st *types.SimpleType) error {

	if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		base, err := r.lookupType(st.Restriction.Base, st.QName)
		if err != nil {
			return fmt.Errorf("type %s: %w", qname, err)
		}
		st.ResolvedBase = base
		if baseST, ok := base.(*types.SimpleType); ok {
			if baseST.Variety() == types.ListVariety || baseST.Variety() == types.UnionVariety {
				st.SetVariety(baseST.Variety())
			}
		}
		// inherit whiteSpace from base type if not explicitly set
		if st.WhiteSpace() == types.WhiteSpacePreserve && base != nil {
			st.SetWhiteSpace(base.WhiteSpace())
		}
	}

	if st.List != nil && !st.List.ItemType.IsZero() {
		item, err := r.lookupType(st.List.ItemType, st.QName)
		if err != nil {
			if allowMissingTypeReference(r.schema, st.List.ItemType) {
				st.ItemType = &types.SimpleType{QName: st.List.ItemType}
			} else {
				return fmt.Errorf("type %s list item: %w", qname, err)
			}
		} else {
			st.ItemType = item
		}
	}

	if st.Union != nil {
		// first add QName-referenced member types
		if len(st.Union.MemberTypes) > 0 {
			st.MemberTypes = make([]types.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
			for i, memberQName := range st.Union.MemberTypes {
				member, err := r.lookupType(memberQName, st.QName)
				if err != nil {
					if allowMissingTypeReference(r.schema, memberQName) {
						st.MemberTypes = append(st.MemberTypes, &types.SimpleType{QName: memberQName})
						continue
					}
					return fmt.Errorf("type %s union member %d: %w", qname, i, err)
				}
				st.MemberTypes = append(st.MemberTypes, member)
			}
		}

		// then add inline member types
		if len(st.Union.InlineTypes) > 0 {
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
		}
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

	baseQName := r.getBaseQName(ct)
	if !baseQName.IsZero() {
		base, err := r.lookupType(baseQName, ct.QName)
		if err != nil {
			return fmt.Errorf("type %s: %w", qname, err)
		}
		ct.ResolvedBase = base
	}

	if err := r.resolveContentParticles(ct.Content()); err != nil {
		return fmt.Errorf("type %s content: %w", qname, err)
	}

	for i, agRef := range ct.AttrGroups {
		ag, err := r.lookupAttributeGroup(agRef)
		if err != nil {
			return fmt.Errorf("type %s attribute group %s: %w", qname, agRef, err)
		}
		if err := r.resolveAttributeGroup(agRef, ag); err != nil {
			return fmt.Errorf("type %s attribute group %s: %w", qname, agRef, err)
		}
		// store resolved reference (we'll expand it during compilation)
		_ = i // TODO: store resolved attribute group for compilation phase
	}

	for _, attr := range ct.Attributes() {
		if err := r.resolveAttributeType(attr); err != nil {
			return err
		}
	}

	return nil
}

func (r *Resolver) lookupType(qname types.QName, referrer types.QName) (types.Type, error) {
	if bt := types.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
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
		return nil, fmt.Errorf("circular reference detected: %v", qname)
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
			// verify the group exists
			group, ok := r.schema.Groups[particle.RefQName]
			if !ok {
				return fmt.Errorf("group %s not found", particle.RefQName)
			}
			// resolve the referenced group (cycle detection via r.detector)
			if err := r.resolveGroup(particle.RefQName, group); err != nil {
				return err
			}
			// GroupRef stays in place - Compiler phase will flatten

		case *types.ModelGroup:
			queue = append(queue, particle.Particles...)

		case *types.ElementDecl:
			// resolve local element types (not top-level elements which are handled separately)
			if !particle.IsReference && particle.Type != nil {
				// check if type is a placeholder SimpleType (created during parsing for unresolved types)
				if st, ok := particle.Type.(*types.SimpleType); ok {
					// if it's a placeholder (has QName but no content), resolve it
					if st.Restriction == nil && st.List == nil && st.Union == nil && !st.QName.IsZero() {
						if r.detector.IsResolving(st.QName) {
							actualType, ok := r.schema.TypeDefs[st.QName]
							if !ok {
								return fmt.Errorf("element %s type: type %s not found", particle.Name, st.QName)
							}
							particle.Type = actualType
							continue
						}
						actualType, err := r.lookupType(st.QName, types.QName{})
						if err != nil {
							if allowMissingTypeReference(r.schema, st.QName) {
								continue
							}
							return fmt.Errorf("element %s type: %w", particle.Name, err)
						}
						particle.Type = actualType
					}
				} else if ct, ok := particle.Type.(*types.ComplexType); ok {
					if !ct.QName.IsZero() && r.detector.IsResolving(ct.QName) {
						continue
					}
					// anonymous complex type - resolve it (includes attributeGroup refs, nested types, etc.)
					if !ct.QName.IsZero() && r.detector.IsResolving(ct.QName) {
						continue
					}
					if err := r.resolveComplexType(ct.QName, ct); err != nil {
						return fmt.Errorf("element %s anonymous type: %w", particle.Name, err)
					}
				}
			}

		case *types.AnyElement:
			// wildcards don't need resolution
		}
	}
	return nil
}

func (r *Resolver) resolveElement(qname types.QName, elem *types.ElementDecl) error {
	// elements may have inline anonymous types - those are already attached
	// elements with type attribute need lookup
	if elem.Type == nil {
		// this shouldn't happen if parsing is correct
		return nil
	}

	// check if type is a placeholder SimpleType (created during parsing for unresolved types)
	if st, ok := elem.Type.(*types.SimpleType); ok {
		// if it's a placeholder (has QName but no content), resolve it
		if st.Restriction == nil && st.List == nil && st.Union == nil && !st.QName.IsZero() {
			// this is a placeholder - resolve the actual type
			// pass empty referrer because element type lookup is not type derivation.
			// self-reference detection is for types referencing themselves as base types,
			// not for elements with the same name as their type (which is valid).
			actualType, err := r.lookupType(st.QName, types.QName{})
			if err != nil {
				if allowMissingTypeReference(r.schema, st.QName) {
					return nil
				}
				return fmt.Errorf("element %s type: %w", qname, err)
			}
			elem.Type = actualType
		} else {
			// it's a real SimpleType - resolve it
			if err := r.resolveSimpleType(st.QName, st); err != nil {
				return fmt.Errorf("element %s type: %w", qname, err)
			}
		}
	} else if ct, ok := elem.Type.(*types.ComplexType); ok {
		// complex type - resolve it (handles anonymous types via pointer tracking)
		if err := r.resolveComplexType(ct.QName, ct); err != nil {
			return fmt.Errorf("element %s type: %w", qname, err)
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

	if st, ok := attr.Type.(*types.SimpleType); ok {
		// if it's a placeholder (has QName but no content), resolve it
		if st.Restriction == nil && st.List == nil && st.Union == nil && !st.QName.IsZero() {
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
	return validation.WalkContentParticles(content, func(particle types.Particle) error {
		return r.resolveParticles([]types.Particle{particle})
	})
}
