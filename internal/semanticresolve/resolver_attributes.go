package semanticresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveAttributeGroupRefs(qname types.QName, groups []types.QName) error {
	for _, agRef := range groups {
		if err := r.resolveAttributeGroupClosure([]types.QName{agRef}); err != nil {
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

func (r *Resolver) resolveAttributeGroup(qname types.QName, _ *types.AttributeGroup) error {
	if err := r.resolveAttributeGroupClosure([]types.QName{qname}); err != nil {
		return fmt.Errorf("attribute group %s: %w", qname, err)
	}
	return nil
}

func (r *Resolver) resolveAttributeGroupClosure(roots []types.QName) error {
	if len(roots) == 0 {
		return nil
	}
	err := attrgroupwalk.WalkWithOptions(r.schema, roots, attrgroupwalk.Options{
		Missing: attrgroupwalk.MissingError,
		Cycles:  attrgroupwalk.CycleError,
	}, func(_ types.QName, ag *types.AttributeGroup) error {
		return r.resolveAttributeDecls(ag.Attributes)
	})
	if err == nil {
		return nil
	}
	var cycleErr attrgroupwalk.AttrGroupCycleError
	if errors.As(err, &cycleErr) {
		return CycleError[types.QName]{Key: cycleErr.QName}
	}
	return err
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
