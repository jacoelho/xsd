package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

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

func (r *Resolver) resolveAttributeGroup(qname types.QName, ag *types.AttributeGroup) error {
	if r.detector.IsVisited(qname) {
		return nil
	}

	return r.detector.WithScope(qname, func() error {
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
	})
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
