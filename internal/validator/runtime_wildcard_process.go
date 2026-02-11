package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/wildcardpolicy"
)

func resolveWildcardSymbol(
	pc runtime.ProcessContents,
	sym runtime.SymbolID,
	resolve func(runtime.SymbolID) bool,
	strictError func() error,
) (bool, error) {
	resolved, strictUnresolved, err := wildcardpolicy.ResolveSymbolByProcessContents(
		runtimeProcessContentsToPolicy(pc),
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

func runtimeProcessContentsToPolicy(pc runtime.ProcessContents) wildcardpolicy.ProcessContents {
	switch pc {
	case runtime.PCStrict:
		return wildcardpolicy.ProcessStrict
	case runtime.PCLax:
		return wildcardpolicy.ProcessLax
	case runtime.PCSkip:
		return wildcardpolicy.ProcessSkip
	default:
		return wildcardpolicy.ProcessStrict + 255
	}
}
