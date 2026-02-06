package runtimebuild

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
)

func toRuntimeWhitespaceMode(ws types.WhiteSpace) runtime.WhitespaceMode {
	switch ws {
	case types.WhiteSpaceReplace:
		return runtime.WS_Replace
	case types.WhiteSpaceCollapse:
		return runtime.WS_Collapse
	default:
		return runtime.WS_Preserve
	}
}

func toValueWhitespaceMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WS_Replace:
		return value.WhitespaceReplace
	case runtime.WS_Collapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}
