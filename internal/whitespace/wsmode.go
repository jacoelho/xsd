package whitespace

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

// ToRuntime converts a schema whitespace facet to runtime mode.
func ToRuntime(ws model.WhiteSpace) runtime.WhitespaceMode {
	switch ws {
	case model.WhiteSpaceReplace:
		return runtime.WS_Replace
	case model.WhiteSpaceCollapse:
		return runtime.WS_Collapse
	default:
		return runtime.WS_Preserve
	}
}

// ToValue converts runtime whitespace mode to lexical normalization mode.
func ToValue(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WS_Replace:
		return value.WhitespaceReplace
	case runtime.WS_Collapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}
