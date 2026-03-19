package lower

import (
	"github.com/jacoelho/xsd/internal/grouprefs"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (b *schemaBuilder) runtimeTypeID(typ model.Type) (runtime.TypeID, bool) {
	if typ == nil {
		return 0, false
	}
	if bt, ok := model.AsBuiltinType(typ); ok {
		return b.builtinIDs[model.TypeName(bt.Name().Local)], true
	}
	if st, ok := model.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := model.GetBuiltin(model.TypeName(st.Name().Local)); builtin != nil {
			return b.builtinIDs[model.TypeName(builtin.Name().Local)], true
		}
	}
	if !typ.Name().IsZero() {
		if id, ok := b.registry.Types[typ.Name()]; ok {
			return b.typeIDs[id], true
		}
	}
	if id, ok := b.registry.LookupAnonymousTypeID(typ); ok {
		return b.typeIDs[id], true
	}
	return 0, false
}

func (b *schemaBuilder) runtimeElemID(decl *model.ElementDecl) (runtime.ElemID, bool) {
	if decl == nil {
		return 0, false
	}
	if decl.IsReference {
		if id, ok := b.refs.ElementRefs[decl.Name]; ok {
			return b.elemIDs[id], true
		}
		return 0, false
	}
	if id, ok := b.registry.LookupLocalElementID(decl); ok {
		return b.elemIDs[id], true
	}
	if id, ok := b.registry.Elements[decl.Name]; ok {
		return b.elemIDs[id], true
	}
	return 0, false
}

func (b *schemaBuilder) resolveTypeQName(qname model.QName) model.Type {
	if qname.IsZero() {
		return nil
	}
	if qname.Namespace == model.XSDNamespace {
		return model.GetBuiltin(model.TypeName(qname.Local))
	}
	return b.schema.TypeDefs[qname]
}

func (b *schemaBuilder) simpleContentTextType(ct *model.ComplexType) (model.Type, error) {
	return grouprefs.ResolveSimpleContentTextType(ct, grouprefs.SimpleContentTextTypeOptions{
		ResolveQName: b.resolveTypeQName,
	})
}

func (b *schemaBuilder) baseForSimpleType(st *model.SimpleType) (model.Type, runtime.DerivationMethod) {
	if st == nil {
		return nil, runtime.DerNone
	}
	if st.List != nil {
		return model.GetBuiltin(model.TypeNameAnySimpleType), runtime.DerList
	}
	if st.Union != nil {
		return model.GetBuiltin(model.TypeNameAnySimpleType), runtime.DerUnion
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
	return model.GetBuiltin(model.TypeNameAnySimpleType), runtime.DerRestriction
}

func (b *schemaBuilder) validatorForBuiltin(name model.TypeName) runtime.ValidatorID {
	if b.validators == nil {
		return 0
	}
	bt := model.GetBuiltin(name)
	if bt == nil {
		return 0
	}
	if id, ok := b.validators.ValidatorForType(bt); ok {
		return id
	}
	return 0
}
