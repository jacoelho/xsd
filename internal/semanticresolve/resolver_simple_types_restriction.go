package semanticresolve

import (
	"fmt"

	model "github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveSimpleTypeRestriction(qname model.QName, st *model.SimpleType) error {
	if st.Restriction == nil {
		return nil
	}

	base := st.ResolvedBase
	if st.Restriction.SimpleType != nil {
		if err := r.resolveSimpleType(st.Restriction.SimpleType.QName, st.Restriction.SimpleType); err != nil {
			return fmt.Errorf("type %s: inline base: %w", qname, err)
		}
		base = st.Restriction.SimpleType
	}

	if !st.Restriction.Base.IsZero() {
		resolvedBase, err := r.lookupType(st.Restriction.Base, st.QName)
		if err != nil {
			return fmt.Errorf("type %s: %w", qname, err)
		}
		base = resolvedBase
	}

	if base != nil {
		st.ResolvedBase = base
		if baseST, ok := base.(*model.SimpleType); ok {
			if baseST.Variety() == model.UnionVariety && len(st.MemberTypes) == 0 {
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
					st.MemberTypes = append([]model.Type(nil), baseST.MemberTypes...)
				}
			}
		}
	}

	if st.WhiteSpace() == model.WhiteSpacePreserve && base != nil {
		st.SetWhiteSpace(base.WhiteSpace())
	}
	return nil
}
