package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveComplexType(qname types.QName, ct *types.ComplexType) error {
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

func (r *Resolver) getBaseQName(ct *types.ComplexType) types.QName {
	return ct.Content().BaseTypeQName()
}
