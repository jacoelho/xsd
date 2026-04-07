package semantics

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/model"
)

func (r *Resolver) resolveSimpleTypeRestriction(qname model.QName, st *model.SimpleType) error {
	if st.Restriction == nil {
		return nil
	}

	base, err := r.resolveRestrictionBaseType(qname, st)
	if err != nil {
		return err
	}
	if base == nil {
		return nil
	}

	st.ResolvedBase = base
	if err := r.inheritRestrictionUnionMembers(st, base); err != nil {
		return err
	}
	inheritRestrictionWhitespace(st, base)
	return nil
}

func (r *Resolver) resolveRestrictionBaseType(qname model.QName, st *model.SimpleType) (model.Type, error) {
	base := st.ResolvedBase
	if st.Restriction.SimpleType != nil {
		if err := r.resolveSimpleType(st.Restriction.SimpleType.QName, st.Restriction.SimpleType); err != nil {
			return nil, fmt.Errorf("type %s: inline base: %w", qname, err)
		}
		base = st.Restriction.SimpleType
	}
	if st.Restriction.Base.IsZero() {
		return base, nil
	}

	resolvedBase, err := r.lookupType(st.Restriction.Base, st.QName)
	if err != nil {
		return nil, fmt.Errorf("type %s: %w", qname, err)
	}
	return resolvedBase, nil
}

func (r *Resolver) inheritRestrictionUnionMembers(st *model.SimpleType, base model.Type) error {
	baseST, ok := base.(*model.SimpleType)
	if !ok || baseST.Variety() != model.UnionVariety || len(st.MemberTypes) > 0 {
		return nil
	}

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
		st.MemberTypes = slices.Clone(baseST.MemberTypes)
	}
	return nil
}

func inheritRestrictionWhitespace(st *model.SimpleType, base model.Type) {
	if st.WhiteSpace() == model.WhiteSpacePreserve {
		st.SetWhiteSpace(base.WhiteSpace())
	}
}
