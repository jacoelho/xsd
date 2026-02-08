package runtimecompile

import "github.com/jacoelho/xsd/internal/types"

func (r *typeResolver) builtinNameForType(typ types.Type) (types.TypeName, bool) {
	return r.builtinNameForTypeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) builtinNameForTypeSeen(typ types.Type, seen map[types.Type]bool) (types.TypeName, bool) {
	if typ == nil {
		return "", false
	}
	if seen[typ] {
		return "", false
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return types.TypeName(bt.Name().Local), true
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return "", false
	}
	if base := r.baseType(st); base != nil {
		return r.builtinNameForTypeSeen(base, seen)
	}
	return "", false
}
