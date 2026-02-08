package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

func (r *Resolver) resolveSimpleTypeUnion(qname types.QName, st *types.SimpleType) error {
	if st.Union == nil {
		return nil
	}
	if err := r.resolveUnionNamedMembers(qname, st); err != nil {
		return err
	}
	if err := r.resolveUnionInlineMembers(qname, st); err != nil {
		return err
	}
	return nil
}

func (r *Resolver) resolveUnionNamedMembers(qname types.QName, st *types.SimpleType) error {
	if len(st.Union.MemberTypes) == 0 {
		return nil
	}
	st.MemberTypes = make([]types.Type, 0, len(st.Union.MemberTypes)+len(st.Union.InlineTypes))
	for i, memberQName := range st.Union.MemberTypes {
		if r.detector.IsResolving(memberQName) {
			if member, ok := r.schema.TypeDefs[memberQName]; ok {
				st.MemberTypes = append(st.MemberTypes, member)
				continue
			}
		}
		member, err := r.lookupType(memberQName, st.QName)
		if err != nil {
			return fmt.Errorf("type %s union member %d: %w", qname, i, err)
		}
		st.MemberTypes = append(st.MemberTypes, member)
	}
	return nil
}

func (r *Resolver) resolveUnionInlineMembers(qname types.QName, st *types.SimpleType) error {
	if len(st.Union.InlineTypes) == 0 {
		return nil
	}
	if st.MemberTypes == nil {
		st.MemberTypes = make([]types.Type, 0, len(st.Union.InlineTypes))
	}
	for i, inlineType := range st.Union.InlineTypes {
		if err := r.resolveSimpleType(inlineType.QName, inlineType); err != nil {
			return fmt.Errorf("type %s union inline member %d: %w", qname, i, err)
		}
		st.MemberTypes = append(st.MemberTypes, inlineType)
	}
	return nil
}
