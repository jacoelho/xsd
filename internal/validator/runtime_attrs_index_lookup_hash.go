package validator

import "github.com/jacoelho/xsd/internal/runtime"

func lookupAttrUseHash(rt *runtime.Schema, ref runtime.AttrIndexRef, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	if int(ref.HashTable) >= len(rt.AttrIndex.HashTables) {
		return runtime.AttrUse{}, -1, false
	}
	table := rt.AttrIndex.HashTables[ref.HashTable]
	if len(table.Hash) == 0 || len(table.Slot) == 0 {
		return runtime.AttrUse{}, -1, false
	}
	hash := uint64(sym)
	if hash == 0 {
		hash = 1
	}
	mask := uint64(len(table.Hash) - 1)
	slot := int(hash & mask)
	for i := 0; i < len(table.Hash); i++ {
		idx := table.Slot[slot]
		if idx == 0 {
			return runtime.AttrUse{}, -1, false
		}
		if table.Hash[slot] == hash {
			useIndex := int(idx - 1)
			if useIndex >= int(ref.Off) && useIndex < int(ref.Off+ref.Len) && useIndex < len(rt.AttrIndex.Uses) {
				use := rt.AttrIndex.Uses[useIndex]
				if use.Name == sym {
					return use, useIndex - int(ref.Off), true
				}
			}
		}
		slot = (slot + 1) & int(mask)
	}
	return runtime.AttrUse{}, -1, false
}
