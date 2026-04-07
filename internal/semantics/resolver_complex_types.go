package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

func (r *Resolver) resolveComplexType(qname model.QName, ct *model.ComplexType) error {
	if qname.IsZero() {
		return r.anonymousTypeGuard.Resolve(ct, func() error {
			return fmt.Errorf("circular anonymous type definition")
		}, func() error {
			return r.doResolveComplexType(qname, ct)
		})
	}
	return analysis.ResolveNamed[model.QName](r.detector, qname, func() error {
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
