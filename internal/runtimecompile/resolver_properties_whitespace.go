package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

func (r *typeResolver) whitespaceMode(typ types.Type) runtime.WhitespaceMode {
	return r.whitespaceModeSeen(typ, make(map[types.Type]bool))
}

func (r *typeResolver) whitespaceModeSeen(typ types.Type, seen map[types.Type]bool) runtime.WhitespaceMode {
	if typ == nil {
		return runtime.WS_Preserve
	}
	if seen[typ] {
		return runtime.WS_Preserve
	}
	seen[typ] = true
	defer delete(seen, typ)

	if bt := builtinForType(typ); bt != nil {
		return toRuntimeWhitespaceMode(bt.WhiteSpace())
	}
	st, ok := types.AsSimpleType(typ)
	if !ok {
		return runtime.WS_Preserve
	}
	if st.WhiteSpaceExplicit() {
		return toRuntimeWhitespaceMode(st.WhiteSpace())
	}
	if st.List != nil || st.Union != nil {
		return toRuntimeWhitespaceMode(st.WhiteSpace())
	}
	if base := r.baseType(st); base != nil {
		return r.whitespaceModeSeen(base, seen)
	}
	return toRuntimeWhitespaceMode(st.WhiteSpace())
}
