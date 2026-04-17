package semantics

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

func (r *Resolver) resolveAttributeGroupRefs(qname model.QName, groups []model.QName) error {
	for _, agRef := range groups {
		if err := r.resolveAttributeGroupClosure([]model.QName{agRef}); err != nil {
			return fmt.Errorf("type %s attribute group %s: %w", qname, agRef, err)
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeDecls(attrs []*model.AttributeDecl) error {
	for _, attr := range attrs {
		if err := r.resolveAttributeType(attr); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeGroup(qname model.QName, _ *model.AttributeGroup) error {
	if err := r.resolveAttributeGroupClosure([]model.QName{qname}); err != nil {
		return fmt.Errorf("attribute group %s: %w", qname, err)
	}
	return nil
}

func (r *Resolver) resolveAttributeGroupClosure(roots []model.QName) error {
	if len(roots) == 0 {
		return nil
	}
	err := WalkAttributeGroupsWithOptions(r.schema, roots, AttributeGroupWalkOptions{
		Missing: MissingError,
		Cycles:  CyclePolicyError,
	}, func(_ model.QName, ag *model.AttributeGroup) error {
		return r.resolveAttributeDecls(ag.Attributes)
	})
	if err == nil {
		return nil
	}
	var cycleErr AttributeGroupCycleError
	if errors.As(err, &cycleErr) {
		return CycleError[model.QName]{Key: cycleErr.QName}
	}
	return err
}

func (r *Resolver) resolveAttributeType(attr *model.AttributeDecl) error {
	if !shouldResolveAttributeType(attr) {
		return nil
	}

	r.relinkAttributeTypeToSchema(attr)
	return r.resolveAttributeSimpleType(attr)
}

func shouldResolveAttributeType(attr *model.AttributeDecl) bool {
	return attr != nil && attr.Type != nil && !attr.IsReference
}

func (r *Resolver) relinkAttributeTypeToSchema(attr *model.AttributeDecl) {
	typeQName := attr.Type.Name()
	if typeQName.IsZero() {
		return
	}
	current, ok := r.schema.TypeDefs[typeQName]
	if ok && current != attr.Type {
		attr.Type = current
	}
}

func (r *Resolver) resolveAttributeSimpleType(attr *model.AttributeDecl) error {
	st, ok := attr.Type.(*model.SimpleType)
	if !ok {
		return nil
	}
	if model.IsPlaceholderSimpleType(st) {
		return r.resolvePlaceholderAttributeSimpleType(attr, st)
	}
	if err := r.resolveSimpleType(st.QName, st); err != nil {
		return fmt.Errorf("attribute %s type: %w", attr.Name, err)
	}
	return nil
}

func (r *Resolver) resolvePlaceholderAttributeSimpleType(attr *model.AttributeDecl, st *model.SimpleType) error {
	actualType, err := r.lookupType(st.QName, model.QName{})
	if err != nil {
		return fmt.Errorf("attribute %s type: %w", attr.Name, err)
	}
	attr.Type = actualType
	return nil
}

func (r *Resolver) resolveComplexType(qname model.QName, ct *model.ComplexType) error {
	if qname.IsZero() {
		return r.anonymousTypeGuard.Resolve(ct, func() error {
			return fmt.Errorf("circular anonymous type definition")
		}, func() error {
			return r.doResolveComplexType(qname, ct)
		})
	}
	return ResolveNamed[model.QName](r.detector, qname, func() error {
		return r.doResolveComplexType(qname, ct)
	})
}

func (r *Resolver) doResolveComplexType(qname model.QName, ct *model.ComplexType) error {
	if err := r.resolveComplexTypeBase(qname, ct); err != nil {
		return err
	}
	if err := r.resolveComplexTypeParticles(qname, ct); err != nil {
		return err
	}
	return r.resolveComplexTypeAttributes(qname, ct)
}

func (r *Resolver) resolveComplexTypeBase(qname model.QName, ct *model.ComplexType) error {
	baseQName := ct.Content().BaseTypeQName()
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

func (r *Resolver) resolveComplexTypeParticles(qname model.QName, ct *model.ComplexType) error {
	if err := r.resolveContentParticles(ct.Content()); err != nil {
		return fmt.Errorf("type %s content: %w", qname, err)
	}
	return nil
}

func (r *Resolver) resolveComplexTypeAttributes(qname model.QName, ct *model.ComplexType) error {
	if err := r.resolveAttributeSet(qname, ct.AttrGroups, ct.Attributes()); err != nil {
		return err
	}
	return r.resolveComplexTypeContentAttributes(qname, ct.Content())
}

func (r *Resolver) resolveComplexTypeContentAttributes(qname model.QName, content model.Content) error {
	if content == nil {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil {
		if err := r.resolveAttributeSet(qname, ext.AttrGroups, ext.Attributes); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := r.resolveAttributeSet(qname, restr.AttrGroups, restr.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeSet(qname model.QName, attrGroups []model.QName, attrs []*model.AttributeDecl) error {
	if err := r.resolveAttributeGroupRefs(qname, attrGroups); err != nil {
		return err
	}
	return r.resolveAttributeDecls(attrs)
}
