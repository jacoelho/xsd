package runtimecompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) isIntegerDerived(typ types.Type) bool {
	return r.isIntegerDerivedSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) isIntegerDerivedSeen(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil {
		return false
	}
	if seen[typ] {
		return false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return isIntegerTypeName(bt.Name().Local)
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return false
	}
	if r.variety(st) != types.AtomicVariety {
		return false
	}
	if isIntegerTypeName(st.Name().Local) {
		return true
	}
	base := r.baseType(st)
	if base == nil {
		return false
	}
	return r.isIntegerDerivedSeen(base, seen)
}
