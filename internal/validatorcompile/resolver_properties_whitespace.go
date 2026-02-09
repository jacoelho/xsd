package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
)

func (r *typeResolver) whitespaceMode(typ types.Type) runtime.WhitespaceMode {
	seen := make(map[types.Type]bool)
	var walk func(types.Type) runtime.WhitespaceMode
	walk = func(current types.Type) runtime.WhitespaceMode {
		if current == nil {
			return runtime.WS_Preserve
		}
		if seen[current] {
			return runtime.WS_Preserve
		}
		seen[current] = true
		defer delete(seen, current)

		if bt := builtinForType(current); bt != nil {
			return wsmode.ToRuntime(bt.WhiteSpace())
		}
		st, ok := types.AsSimpleType(current)
		if !ok {
			return runtime.WS_Preserve
		}
		if st.WhiteSpaceExplicit() {
			return wsmode.ToRuntime(st.WhiteSpace())
		}
		if st.List != nil || st.Union != nil {
			return wsmode.ToRuntime(st.WhiteSpace())
		}
		if base := r.baseType(st); base != nil {
			return walk(base)
		}
		return wsmode.ToRuntime(st.WhiteSpace())
	}
	return walk(typ)
}
