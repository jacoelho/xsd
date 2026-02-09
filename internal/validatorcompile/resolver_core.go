package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type typeResolver struct {
	schema *parser.Schema
}

func newTypeResolver(schema *parser.Schema) *typeResolver {
	return &typeResolver{schema: schema}
}

func (r *typeResolver) baseType(st *types.SimpleType) types.Type {
	if st == nil {
		return nil
	}
	if st.ResolvedBase != nil {
		if isAnySimpleType(st.ResolvedBase) {
			return nil
		}
		return st.ResolvedBase
	}
	if st.Restriction == nil {
		return nil
	}
	if st.Restriction.SimpleType != nil {
		if isAnySimpleType(st.Restriction.SimpleType) {
			return nil
		}
		return st.Restriction.SimpleType
	}
	if st.Restriction.Base.IsZero() {
		return nil
	}
	base := r.resolveQName(st.Restriction.Base)
	if isAnySimpleType(base) {
		return nil
	}
	return base
}

func (r *typeResolver) resolveQName(name types.QName) types.Type {
	if name.IsZero() {
		return nil
	}
	if builtin := types.GetBuiltinNS(name.Namespace, name.Local); builtin != nil {
		return builtin
	}
	if r.schema == nil {
		return nil
	}
	if def, ok := r.schema.TypeDefs[name]; ok {
		return def
	}
	return nil
}
