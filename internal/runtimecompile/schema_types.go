package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaops"
	"github.com/jacoelho/xsd/internal/types"
)

func (b *schemaBuilder) runtimeTypeID(typ types.Type) (runtime.TypeID, bool) {
	if typ == nil {
		return 0, false
	}
	if bt, ok := types.AsBuiltinType(typ); ok {
		return b.builtinIDs[types.TypeName(bt.Name().Local)], true
	}
	if st, ok := types.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
			return b.builtinIDs[types.TypeName(builtin.Name().Local)], true
		}
	}
	if !typ.Name().IsZero() {
		if id, ok := b.registry.Types[typ.Name()]; ok {
			return b.typeIDs[id], true
		}
	}
	if id, ok := b.registry.AnonymousTypes[typ]; ok {
		return b.typeIDs[id], true
	}
	return 0, false
}

func (b *schemaBuilder) runtimeElemID(decl *types.ElementDecl) (runtime.ElemID, bool) {
	if decl == nil {
		return 0, false
	}
	if decl.IsReference {
		if id, ok := b.refs.ElementRefs[decl]; ok {
			return b.elemIDs[id], true
		}
		return 0, false
	}
	if id, ok := b.registry.LocalElements[decl]; ok {
		return b.elemIDs[id], true
	}
	if id, ok := b.registry.Elements[decl.Name]; ok {
		return b.elemIDs[id], true
	}
	return 0, false
}

func (b *schemaBuilder) resolveTypeQName(qname types.QName) types.Type {
	if qname.IsZero() {
		return nil
	}
	if qname.Namespace == types.XSDNamespace {
		return types.GetBuiltin(types.TypeName(qname.Local))
	}
	return b.schema.TypeDefs[qname]
}

func (b *schemaBuilder) simpleContentTextType(ct *types.ComplexType) (types.Type, error) {
	res := newTypeResolver(b.schema)
	return schemaops.ResolveSimpleContentTextType(ct, schemaops.SimpleContentTextTypeOptions{
		ResolveQName: res.resolveQName,
	})
}

func (b *schemaBuilder) baseForSimpleType(st *types.SimpleType) (types.Type, runtime.DerivationMethod) {
	if st == nil {
		return nil, runtime.DerNone
	}
	if st.List != nil {
		return types.GetBuiltin(types.TypeNameAnySimpleType), runtime.DerList
	}
	if st.Union != nil {
		return types.GetBuiltin(types.TypeNameAnySimpleType), runtime.DerUnion
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			return st.Restriction.SimpleType, runtime.DerRestriction
		}
		if !st.Restriction.Base.IsZero() {
			return b.resolveTypeQName(st.Restriction.Base), runtime.DerRestriction
		}
	}
	if st.ResolvedBase != nil {
		return st.ResolvedBase, runtime.DerRestriction
	}
	return types.GetBuiltin(types.TypeNameAnySimpleType), runtime.DerRestriction
}

func (b *schemaBuilder) validatorForBuiltin(name types.TypeName) runtime.ValidatorID {
	if b.validators == nil {
		return 0
	}
	bt := types.GetBuiltin(name)
	if bt == nil {
		return 0
	}
	if id, ok := b.validators.ValidatorForType(bt); ok {
		return id
	}
	return 0
}
