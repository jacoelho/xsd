package validator

import "github.com/jacoelho/xsd/internal/runtime"

func lookupAttrUseBinary(uses []runtime.AttrUse, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	lo := 0
	hi := len(uses) - 1
	for lo <= hi {
		mid := (lo + hi) / 2
		name := uses[mid].Name
		if name == sym {
			return uses[mid], mid, true
		}
		if name < sym {
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	return runtime.AttrUse{}, -1, false
}
