package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/traversal"
)

// Resolve resolves all references in the schema.
// Returns an error if there are unresolvable references or invalid cycles.
func (r *Resolver) Resolve() error {
	// order matters: resolve in dependency order
	if err := r.resolveSimpleTypesPhase(); err != nil {
		return err
	}
	if err := r.resolveComplexTypesPhase(); err != nil {
		return err
	}
	if err := r.resolveGroupsPhase(); err != nil {
		return err
	}
	if err := r.resolveElementsPhase(); err != nil {
		return err
	}
	if err := r.resolveAttributesPhase(); err != nil {
		return err
	}
	if err := r.resolveAttributeGroupsPhase(); err != nil {
		return err
	}
	return nil
}

func (r *Resolver) resolveSimpleTypesPhase() error {
	// 1. Simple types (only depend on built-ins or other simple types)
	for _, qname := range traversal.SortedQNames(r.schema.TypeDefs) {
		typ := r.schema.TypeDefs[qname]
		if st, ok := typ.(*model.SimpleType); ok {
			if err := r.resolveSimpleType(qname, st); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Resolver) resolveComplexTypesPhase() error {
	// 2. Complex types (may depend on simple types)
	for _, qname := range traversal.SortedQNames(r.schema.TypeDefs) {
		typ := r.schema.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			if err := r.resolveComplexType(qname, ct); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Resolver) resolveGroupsPhase() error {
	// 3. Groups (reference types and other groups)
	for _, qname := range traversal.SortedQNames(r.schema.Groups) {
		grp := r.schema.Groups[qname]
		if r.detector.IsVisited(qname) {
			continue
		}
		if err := r.detector.WithScope(qname, func() error {
			return r.resolveParticles(grp.Particles)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveElementsPhase() error {
	// 4. Elements (reference types and groups)
	for _, qname := range traversal.SortedQNames(r.schema.ElementDecls) {
		elem := r.schema.ElementDecls[qname]
		if elem.Type == nil {
			continue
		}
		if err := r.resolveElementType(elem, qname, elementTypeOptions{
			simpleContext:  "element %s type: %w",
			complexContext: "element %s type: %w",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveAttributesPhase() error {
	// 5. Attributes
	for _, qname := range traversal.SortedQNames(r.schema.AttributeDecls) {
		attr := r.schema.AttributeDecls[qname]
		if err := r.resolveAttributeType(attr); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeGroupsPhase() error {
	// 6. Attribute groups
	for _, qname := range traversal.SortedQNames(r.schema.AttributeGroups) {
		ag := r.schema.AttributeGroups[qname]
		if err := r.resolveAttributeGroup(qname, ag); err != nil {
			return err
		}
	}
	return nil
}
