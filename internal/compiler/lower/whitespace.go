package lower

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func runtimeWhitespaceMode(ws model.WhiteSpace) runtime.WhitespaceMode {
	switch ws {
	case model.WhiteSpaceReplace:
		return runtime.WSReplace
	case model.WhiteSpaceCollapse:
		return runtime.WSCollapse
	default:
		return runtime.WSPreserve
	}
}

func valueWhitespaceMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WSReplace:
		return value.WhitespaceReplace
	case runtime.WSCollapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}
