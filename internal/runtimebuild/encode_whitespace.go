package runtimebuild

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/wsmode"
)

func toRuntimeWhitespaceMode(ws types.WhiteSpace) runtime.WhitespaceMode {
	return wsmode.ToRuntime(ws)
}

func toValueWhitespaceMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	return wsmode.ToValue(mode)
}
