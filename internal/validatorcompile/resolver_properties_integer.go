package validatorcompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) isIntegerDerived(typ types.Type) bool {
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
			return isIntegerTypeName(bt.Name().Local)
		}
		st, ok := types.AsSimpleType(current)
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
		return walk(base)
	}
	return walk(typ)
}
