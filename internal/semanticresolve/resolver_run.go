package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/resolveguard"
)

// Resolve resolves all references in the schema.
// Returns an error if there are unresolvable references or invalid cycles.
func (r *Resolver) Resolve() error {
	index := buildIterationIndex(r.schema)
	// order matters: resolve in dependency order
	if err := r.resolveSimpleTypesPhase(index); err != nil {
		return err
	}
	if err := r.resolveComplexTypesPhase(index); err != nil {
		return err
	}
	if err := r.resolveGroupsPhase(index); err != nil {
		return err
	}
	if err := r.resolveElementsPhase(index); err != nil {
		return err
	}
	if err := r.resolveAttributesPhase(index); err != nil {
		return err
	}
	if err := r.resolveAttributeGroupsPhase(index); err != nil {
		return err
	}
	return nil
}

func (r *Resolver) resolveSimpleTypesPhase(index *iterationIndex) error {
	// 1. Simple types (only depend on built-ins or other simple types)
	for _, qname := range index.typeQNames {
		typ := r.schema.TypeDefs[qname]
		if st, ok := typ.(*model.SimpleType); ok {
			if err := r.resolveSimpleType(qname, st); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Resolver) resolveComplexTypesPhase(index *iterationIndex) error {
	// 2. Complex types (may depend on simple types)
	for _, qname := range index.typeQNames {
		typ := r.schema.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			if err := r.resolveComplexType(qname, ct); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Resolver) resolveGroupsPhase(index *iterationIndex) error {
	// 3. Groups (reference types and other groups)
	for _, qname := range index.groupQNames {
		grp := r.schema.Groups[qname]
		if err := resolveguard.ResolveNamed[model.QName](r.detector, qname, func() error {
			return r.resolveParticles(grp.Particles)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveElementsPhase(index *iterationIndex) error {
	// 4. Elements (reference types and groups)
	for _, qname := range index.elementQNames {
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

func (r *Resolver) resolveAttributesPhase(index *iterationIndex) error {
	// 5. Attributes
	for _, qname := range index.attributeQNames {
		attr := r.schema.AttributeDecls[qname]
		if err := r.resolveAttributeType(attr); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeGroupsPhase(index *iterationIndex) error {
	// 6. Attribute groups
	for _, qname := range index.attributeGroupQNames {
		ag := r.schema.AttributeGroups[qname]
		if err := r.resolveAttributeGroup(qname, ag); err != nil {
			return err
		}
	}
	return nil
}
