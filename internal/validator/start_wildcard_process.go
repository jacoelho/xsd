package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// ResolveStartSymbol applies strict/lax/skip wildcard resolution to one symbol.
func ResolveStartSymbol(
	pc runtime.ProcessContents,
	sym runtime.SymbolID,
	resolve func(runtime.SymbolID) bool,
	strictError func() error,
) (bool, error) {
	switch pc {
	case runtime.PCSkip:
		return false, nil
	case runtime.PCLax, runtime.PCStrict:
		if sym != 0 && resolve != nil && resolve(sym) {
			return true, nil
		}
		if pc == runtime.PCLax {
			return false, nil
		}
		if strictError != nil {
			return false, strictError()
		}
		return false, fmt.Errorf("wildcard strict unresolved")
	default:
		return false, fmt.Errorf("unknown wildcard processContents %d", pc)
	}
}
