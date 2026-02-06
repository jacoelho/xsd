package runtimecompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/value"
	wsmode "github.com/jacoelho/xsd/internal/whitespace"
)

func toRuntimeWhitespaceMode(ws types.WhiteSpace) runtime.WhitespaceMode {
	return wsmode.ToRuntime(ws)
}

func toValueWhitespaceMode(mode runtime.WhitespaceMode) value.WhitespaceMode {
	return wsmode.ToValue(mode)
}
