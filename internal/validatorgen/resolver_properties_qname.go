package validatorgen

import "github.com/jacoelho/xsd/internal/model"

func (r *typeResolver) isQNameOrNotation(typ model.Type) bool {
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
			return model.IsQNameOrNotation(bt.Name())
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			return false
		}
		if r.variety(st) != model.AtomicVariety {
			return false
		}
		if model.IsQNameOrNotation(st.Name()) {
			return true
		}
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			base := st.Restriction.Base
			if (base.Namespace == model.XSDNamespace || base.Namespace == "") &&
				(base.Local == string(model.TypeNameQName) || base.Local == string(model.TypeNameNOTATION)) {
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
