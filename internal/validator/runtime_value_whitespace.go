package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
)

func runtimeWhitespaceValueMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	switch mode {
	case runtime.WSReplace:
		return value.WhitespaceReplace
	case runtime.WSCollapse:
		return value.WhitespaceCollapse
	default:
		return value.WhitespacePreserve
	}
}
