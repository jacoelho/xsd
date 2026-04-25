package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaast"
)

// ResolveStartSymbol applies strict/lax/skip wildcard resolution to one symbol.
func ResolveStartSymbol(
	pc runtime.ProcessContents,
	sym runtime.SymbolID,
	resolve func(runtime.SymbolID) bool,
	strictError func() error,
) (bool, error) {
	resolved, strictUnresolved, err := schemaast.ResolveSymbolByProcessContents(
		runtimeProcessContentsToModel(pc),
		sym != 0,
		func() bool {
			if resolve == nil {
				return false
			}
			return resolve(sym)
		},
	)
	if err != nil {
		return false, err
	}
	if !strictUnresolved {
		return resolved, nil
	}
	if strictError != nil {
		return false, strictError()
	}
	return false, fmt.Errorf("wildcard strict unresolved")
}

func runtimeProcessContentsToModel(pc runtime.ProcessContents) schemaast.ProcessContents {
	switch pc {
	case runtime.PCStrict:
		return schemaast.Strict
	case runtime.PCLax:
		return schemaast.Lax
	case runtime.PCSkip:
		return schemaast.Skip
	default:
		return schemaast.Strict + 255
	}
}
