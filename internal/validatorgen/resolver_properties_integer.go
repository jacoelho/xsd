package validatorgen

import "github.com/jacoelho/xsd/internal/model"

func (r *typeResolver) isIntegerDerived(typ model.Type) bool {
	seen := make(map[model.Type]bool)
	var walk func(model.Type) bool
	walk = func(current model.Type) bool {
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
		st, ok := model.AsSimpleType(current)
		if !ok {
			return false
		}
		if r.variety(st) != model.AtomicVariety {
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
