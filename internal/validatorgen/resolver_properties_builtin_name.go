package validatorgen

import "github.com/jacoelho/xsd/internal/model"

func (r *typeResolver) builtinNameForType(typ model.Type) (model.TypeName, bool) {
	seen := make(map[model.Type]bool)
	var walk func(model.Type) (model.TypeName, bool)
	walk = func(current model.Type) (model.TypeName, bool) {
		if current == nil {
			return "", false
		}
		if seen[current] {
			return "", false
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return model.TypeName(bt.Name().Local), true
		}
		st, ok := model.AsSimpleType(current)
		if !ok {
			return "", false
		}
		if base := r.baseType(st); base != nil {
			return walk(base)
		}
		return "", false
	}
	return walk(typ)
}
