package runtimecompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) variety(st *types.SimpleType) types.SimpleTypeVariety {
	if st == nil {
		return types.AtomicVariety
	}
	if st.List != nil {
		return types.ListVariety
	}
	if st.Union != nil {
		return types.UnionVariety
	}
	if st.ResolvedBase != nil {
		if baseST, ok := types.AsSimpleType(st.ResolvedBase); ok {
			return r.variety(baseST)
		}
		if bt := builtinForType(st.ResolvedBase); bt != nil && isBuiltinListName(bt.Name().Local) {
			return types.ListVariety
		}
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			if baseST, ok := types.AsSimpleType(st.Restriction.SimpleType); ok {
				return r.variety(baseST)
			}
			if bt := builtinForType(st.Restriction.SimpleType); bt != nil && isBuiltinListName(bt.Name().Local) {
				return types.ListVariety
			}
		}
		if !st.Restriction.Base.IsZero() {
			if base := r.resolveQName(st.Restriction.Base); base != nil {
				if baseST, ok := types.AsSimpleType(base); ok {
					return r.variety(baseST)
				}
				if bt := builtinForType(base); bt != nil && isBuiltinListName(bt.Name().Local) {
					return types.ListVariety
				}
			}
		}
	}
	if st.IsBuiltin() && isBuiltinListName(st.Name().Local) {
		return types.ListVariety
	}
	return types.AtomicVariety
}

func (r *typeResolver) varietyForType(typ types.Type) types.SimpleTypeVariety {
	if typ == nil {
		return types.AtomicVariety
	}
	if bt := builtinForType(typ); bt != nil {
		if isBuiltinListName(bt.Name().Local) {
			return types.ListVariety
		}
		return types.AtomicVariety
	}
	if st, ok := types.AsSimpleType(typ); ok {
		return r.variety(st)
	}
	return types.AtomicVariety
}

func (r *typeResolver) isListType(typ types.Type) bool {
	return r.varietyForType(typ) == types.ListVariety
}
