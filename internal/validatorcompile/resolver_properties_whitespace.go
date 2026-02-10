package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
)

func (r *typeResolver) whitespaceMode(typ model.Type) runtime.WhitespaceMode {
	seen := make(map[model.Type]bool)
	var walk func(model.Type) runtime.WhitespaceMode
	walk = func(current model.Type) runtime.WhitespaceMode {
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
		st, ok := model.AsSimpleType(current)
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
