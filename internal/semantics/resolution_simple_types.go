package semantics

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

func (r *Resolver) resolveSimpleType(qname model.QName, st *model.SimpleType) error {
	if qname.IsZero() {
		return r.anonymousTypeGuard.Resolve(st, func() error {
			return fmt.Errorf("circular anonymous type definition")
		}, func() error {
			return r.doResolveSimpleType(qname, st)
		})
	}
	return analysis.ResolveNamed[model.QName](r.detector, qname, func() error {
		return r.doResolveSimpleType(qname, st)
	})
}

func (r *Resolver) doResolveSimpleType(qname model.QName, st *model.SimpleType) error {
	if err := r.resolveSimpleTypeRestriction(qname, st); err != nil {
		return err
	}
	if err := r.resolveSimpleTypeList(qname, st); err != nil {
		return err
	}
	return r.resolveSimpleTypeUnion(qname, st)
}

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

func (r *Resolver) resolveSimpleTypeList(qname model.QName, st *model.SimpleType) error {
	if st.List == nil {
		return nil
	}
	if st.List.InlineItemType != nil {
		if err := r.resolveSimpleType(st.List.InlineItemType.QName, st.List.InlineItemType); err != nil {
			return fmt.Errorf("type %s list inline item: %w", qname, err)
		}
		st.ItemType = st.List.InlineItemType
		if !st.WhiteSpaceExplicit() {
			st.SetWhiteSpace(model.WhiteSpaceCollapse)
		}
		return nil
	}
	if st.List.ItemType.IsZero() {
		if !st.WhiteSpaceExplicit() {
			st.SetWhiteSpace(model.WhiteSpaceCollapse)
		}
		return nil
	}
	item, err := r.lookupType(st.List.ItemType, st.QName)
	if err != nil {
		return fmt.Errorf("type %s list item: %w", qname, err)
	}
	st.ItemType = item
	if !st.WhiteSpaceExplicit() {
		st.SetWhiteSpace(model.WhiteSpaceCollapse)
	}
	return nil
}

func (r *Resolver) resolveSimpleTypeUnion(qname model.QName, st *model.SimpleType) error {
	if st.Union == nil {
		return nil
	}
	if err := r.resolveUnionNamedMembers(qname, st); err != nil {
		return err
	}
	return r.resolveUnionInlineMembers(qname, st)
}

func (r *Resolver) resolveUnionNamedMembers(qname model.QName, st *model.SimpleType) error {
	if len(st.Union.MemberTypes) == 0 {
		return nil
	}

	resetUnionMemberTypesForNamed(st)
	for i, memberQName := range st.Union.MemberTypes {
		member, err := r.resolveUnionNamedMember(memberQName, st.QName)
		if err != nil {
			return fmt.Errorf("type %s union member %d: %w", qname, i, err)
		}
		st.MemberTypes = append(st.MemberTypes, member)
	}
	return nil
}

func (r *Resolver) resolveUnionInlineMembers(qname model.QName, st *model.SimpleType) error {
	if len(st.Union.InlineTypes) == 0 {
		return nil
	}

	ensureUnionMemberTypesForInline(st)
	for i, inlineType := range st.Union.InlineTypes {
		member, err := r.resolveUnionInlineMember(inlineType)
		if err != nil {
			return fmt.Errorf("type %s union inline member %d: %w", qname, i, err)
		}
		st.MemberTypes = append(st.MemberTypes, member)
	}
	return nil
}

func resetUnionMemberTypesForNamed(st *model.SimpleType) {
	st.MemberTypes = make([]model.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
}

func (r *Resolver) resolveUnionNamedMember(memberQName, referrer model.QName) (model.Type, error) {
	if r.detector.IsResolving(memberQName) {
		if member, ok := r.schema.TypeDefs[memberQName]; ok {
			return member, nil
		}
	}
	return r.lookupType(memberQName, referrer)
}

func ensureUnionMemberTypesForInline(st *model.SimpleType) {
	if st.MemberTypes == nil {
		st.MemberTypes = make([]model.Type, 0, len(st.Union.InlineTypes))
	}
}

func (r *Resolver) resolveUnionInlineMember(inlineType *model.SimpleType) (model.Type, error) {
	if err := r.resolveSimpleType(inlineType.QName, inlineType); err != nil {
		return nil, err
	}
	return inlineType, nil
}
