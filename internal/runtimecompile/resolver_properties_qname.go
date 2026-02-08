package runtimecompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) isQNameOrNotation(typ types.Type) bool {
	return r.isQNameOrNotationSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) isQNameOrNotationSeen(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil {
		return false
	}
	if seen[typ] {
		return false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return bt.IsQNameOrNotationType()
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return false
	}
	if r.variety(st) != types.AtomicVariety {
		return false
	}
	if types.IsQNameOrNotation(st.Name()) {
		return true
	}
	if st.Restriction != nil && !st.Restriction.Base.IsZero() {
		base := st.Restriction.Base
		if (base.Namespace == types.XSDNamespace || base.Namespace.IsEmpty()) &&
			(base.Local == string(types.TypeNameQName) || base.Local == string(types.TypeNameNOTATION)) {
			return true
		}
	}
	if base := r.baseType(st); base != nil {
		return r.isQNameOrNotationSeen(base, seen)
	}
	return false
}
