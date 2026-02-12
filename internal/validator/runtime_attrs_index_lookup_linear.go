package validator

import "github.com/jacoelho/xsd/internal/runtime"

func lookupAttrUseLinear(uses []runtime.AttrUse, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	for i := range uses {
		use := &uses[i]
		if use.Name == sym {
			return *use, i, true
		}
	}
	return runtime.AttrUse{}, -1, false
}
