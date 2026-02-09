package validatorcompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) isQNameOrNotation(typ types.Type) bool {
	seen := make(map[types.Type]bool)
	var walk func(types.Type) bool
	walk = func(current types.Type) bool {
		if current == nil {
			return false
		}
		if seen[current] {
			return false
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return types.IsQNameOrNotation(bt.Name())
		}
		st, ok := types.AsSimpleType(current)
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
			return walk(base)
		}
		return false
	}
	return walk(typ)
}
