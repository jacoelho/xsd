package validatorgen

import (
	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
)

func (r *typeResolver) variety(st *model.SimpleType) model.SimpleTypeVariety {
	if st == nil {
		return model.AtomicVariety
	}
	if st.List != nil {
		return model.ListVariety
	}
	if st.Union != nil {
		return model.UnionVariety
	}
	if st.ResolvedBase != nil {
		if baseST, ok := model.AsSimpleType(st.ResolvedBase); ok {
			return r.variety(baseST)
		}
		if bt := builtinForType(st.ResolvedBase); bt != nil && builtins.IsBuiltinListTypeName(bt.Name().Local) {
			return model.ListVariety
		}
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			if baseST, ok := model.AsSimpleType(st.Restriction.SimpleType); ok {
				return r.variety(baseST)
			}
			if bt := builtinForType(st.Restriction.SimpleType); bt != nil && builtins.IsBuiltinListTypeName(bt.Name().Local) {
				return model.ListVariety
			}
		}
		if !st.Restriction.Base.IsZero() {
			if base := r.resolveQName(st.Restriction.Base); base != nil {
				if baseST, ok := model.AsSimpleType(base); ok {
					return r.variety(baseST)
				}
				if bt := builtinForType(base); bt != nil && builtins.IsBuiltinListTypeName(bt.Name().Local) {
					return model.ListVariety
				}
			}
		}
	}
	if st.IsBuiltin() && builtins.IsBuiltinListTypeName(st.Name().Local) {
		return model.ListVariety
	}
	return model.AtomicVariety
}

func (r *typeResolver) varietyForType(typ model.Type) model.SimpleTypeVariety {
	if typ == nil {
		return model.AtomicVariety
	}
	if bt := builtinForType(typ); bt != nil {
		if builtins.IsBuiltinListTypeName(bt.Name().Local) {
			return model.ListVariety
		}
		return model.AtomicVariety
	}
	if st, ok := model.AsSimpleType(typ); ok {
		return r.variety(st)
	}
	return model.AtomicVariety
}

func (r *typeResolver) isListType(typ model.Type) bool {
	return r.varietyForType(typ) == model.ListVariety
}
