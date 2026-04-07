package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

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
