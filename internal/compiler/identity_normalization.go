package compiler

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

type identityNormalizationBuilder struct {
	cache map[types.Type]*grammar.IdentityNormalizationPlan
	stack map[types.Type]bool
}

func newIdentityNormalizationBuilder(cache map[types.Type]*grammar.IdentityNormalizationPlan) *identityNormalizationBuilder {
	return &identityNormalizationBuilder{
		cache: cache,
		stack: make(map[types.Type]bool),
	}
}

func (b *identityNormalizationBuilder) ensure(typ types.Type) *grammar.IdentityNormalizationPlan {
	if typ == nil {
		typ = types.GetBuiltin(types.TypeNameString)
	}
	if plan := b.cache[typ]; plan != nil {
		return plan
	}
	if b.stack[typ] {
		plan := &grammar.IdentityNormalizationPlan{
			Kind: grammar.IdentityNormalizationAtomic,
			Type: typ,
		}
		b.cache[typ] = plan
		return plan
	}

	b.stack[typ] = true
	defer delete(b.stack, typ)

	if itemType, ok := types.ListItemType(typ); ok {
		plan := &grammar.IdentityNormalizationPlan{
			Kind: grammar.IdentityNormalizationList,
			Type: typ,
			Item: b.ensure(itemType),
		}
		b.cache[typ] = plan
		return plan
	}

	if st, ok := types.AsSimpleType(typ); ok && st.Variety() == types.UnionVariety {
		members := make([]*grammar.IdentityNormalizationPlan, 0, len(st.MemberTypes))
		for _, member := range st.MemberTypes {
			if member == nil {
				continue
			}
			members = append(members, b.ensure(member))
		}
		plan := &grammar.IdentityNormalizationPlan{
			Kind:    grammar.IdentityNormalizationUnion,
			Type:    typ,
			Members: members,
		}
		b.cache[typ] = plan
		return plan
	}

	plan := &grammar.IdentityNormalizationPlan{
		Kind: grammar.IdentityNormalizationAtomic,
		Type: typ,
	}
	b.cache[typ] = plan
	return plan
}

func (c *Compiler) buildIdentityNormalizationPlans() {
	if c == nil || c.grammar == nil {
		return
	}
	if c.grammar.IdentityNormalization == nil {
		c.grammar.IdentityNormalization = make(map[types.Type]*grammar.IdentityNormalizationPlan)
	}
	builder := newIdentityNormalizationBuilder(c.grammar.IdentityNormalization)
	builder.ensure(types.GetBuiltin(types.TypeNameString))
	for typ := range c.typesByPtr {
		builder.ensure(typ)
	}
}
