package validatorcompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) builtinNameForType(typ types.Type) (types.TypeName, bool) {
	seen := make(map[types.Type]bool)
	var walk func(types.Type) (types.TypeName, bool)
	walk = func(current types.Type) (types.TypeName, bool) {
		if current == nil {
			return "", false
		}
		if seen[current] {
			return "", false
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return types.TypeName(bt.Name().Local), true
		}
		st, ok := types.AsSimpleType(current)
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
