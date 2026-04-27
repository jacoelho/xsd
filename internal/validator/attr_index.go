package validator

import "github.com/jacoelho/xsd/internal/runtime"

// Uses returns the attribute-use slice referenced by ref.
func Uses(uses []runtime.AttrUse, ref runtime.AttrIndexRef) []runtime.AttrUse {
	if ref.Len == 0 {
		return nil
	}
	start, end, ok := checkedSpan(ref.Off, ref.Len, len(uses))
	if !ok || start == len(uses) {
		return nil
	}
	return uses[start:end]
}

// LookupUse resolves one attribute use by symbol within the referenced index.
func LookupUse(rt *runtime.Schema, ref runtime.AttrIndexRef, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	if rt == nil {
		return runtime.AttrUse{}, -1, false
	}
	uses := rt.AttributeUses(ref)
	switch ref.Mode {
	case runtime.AttrIndexSortedBinary:
		return lookupUseBinary(uses, sym)
	case runtime.AttrIndexHash:
		return lookupUseHash(rt, ref, sym)
	default:
		return lookupUseLinear(uses, sym)
	}
}

// GlobalAttributeBySymbol resolves one global attribute identifier by symbol.
func GlobalAttributeBySymbol(rt *runtime.Schema, sym runtime.SymbolID) (runtime.AttrID, bool) {
	return rt.GlobalAttribute(sym)
}

func lookupUseBinary(uses []runtime.AttrUse, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
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

func lookupUseHash(rt *runtime.Schema, ref runtime.AttrIndexRef, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	table, ok := rt.AttributeHashTable(ref.HashTable)
	if !ok {
		return runtime.AttrUse{}, -1, false
	}
	attrIndex := rt.AttributeIndex()
	if len(table.Hash) == 0 || len(table.Slot) == 0 {
		return runtime.AttrUse{}, -1, false
	}
	hash := uint64(sym)
	if hash == 0 {
		hash = 1
	}
	mask := uint64(len(table.Hash) - 1)
	slot := int(hash & mask)
	for range len(table.Hash) {
		idx := table.Slot[slot]
		if idx == 0 {
			return runtime.AttrUse{}, -1, false
		}
		if table.Hash[slot] == hash {
			useIndex := int(idx - 1)
			if useIndex >= int(ref.Off) && useIndex < int(ref.Off+ref.Len) && useIndex < len(attrIndex.Uses) {
				use := attrIndex.Uses[useIndex]
				if use.Name == sym {
					return use, useIndex - int(ref.Off), true
				}
			}
		}
		slot = (slot + 1) & int(mask)
	}
	return runtime.AttrUse{}, -1, false
}

func lookupUseLinear(uses []runtime.AttrUse, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	for i := range uses {
		use := &uses[i]
		if use.Name == sym {
			return *use, i, true
		}
	}
	return runtime.AttrUse{}, -1, false
}
