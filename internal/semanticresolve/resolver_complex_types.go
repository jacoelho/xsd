package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/resolveguard"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveComplexType(qname types.QName, ct *types.ComplexType) error {
	if qname.IsZero() {
		return r.anonymousTypeGuard.Resolve(ct, func() error {
			return fmt.Errorf("circular anonymous type definition")
		}, func() error {
			return r.doResolveComplexType(qname, ct)
		})
	}
	return resolveguard.ResolveNamed[types.QName](r.detector, qname, func() error {
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
	if ext := content.ExtensionDef(); ext != nil {
		if err := r.resolveAttributeGroupRefs(qname, ext.AttrGroups); err != nil {
			return err
		}
		if err := r.resolveAttributeDecls(ext.Attributes); err != nil {
			return err
		}
	}
	if restr := content.RestrictionDef(); restr != nil {
		if err := r.resolveAttributeGroupRefs(qname, restr.AttrGroups); err != nil {
			return err
		}
		if err := r.resolveAttributeDecls(restr.Attributes); err != nil {
			return err
		}
	}

	return nil
}
