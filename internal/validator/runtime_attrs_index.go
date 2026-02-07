package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) attrUses(ref runtime.AttrIndexRef) []runtime.AttrUse {
	return sliceAttrUses(s.rt.AttrIndex.Uses, ref)
}

func sliceAttrUses(uses []runtime.AttrUse, ref runtime.AttrIndexRef) []runtime.AttrUse {
	if ref.Len == 0 {
		return nil
	}
	off := ref.Off
	end := off + ref.Len
	if int(off) >= len(uses) || int(end) > len(uses) {
		return nil
	}
	return uses[off:end]
}

func lookupAttrUse(rt *runtime.Schema, ref runtime.AttrIndexRef, sym runtime.SymbolID) (runtime.AttrUse, int, bool) {
	if rt == nil {
		return runtime.AttrUse{}, -1, false
	}
	uses := sliceAttrUses(rt.AttrIndex.Uses, ref)
	switch ref.Mode {
	case runtime.AttrIndexSortedBinary:
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
			} else {
				hi = mid - 1
			}
		}
		return runtime.AttrUse{}, -1, false
	case runtime.AttrIndexHash:
		if int(ref.HashTable) >= len(rt.AttrIndex.HashTables) {
			return runtime.AttrUse{}, -1, false
		}
		table := rt.AttrIndex.HashTables[ref.HashTable]
		if len(table.Hash) == 0 || len(table.Slot) == 0 {
			return runtime.AttrUse{}, -1, false
		}
		h := uint64(sym)
		if h == 0 {
			h = 1
		}
		mask := uint64(len(table.Hash) - 1)
		slot := int(h & mask)
		for i := 0; i < len(table.Hash); i++ {
			idx := table.Slot[slot]
			if idx == 0 {
				return runtime.AttrUse{}, -1, false
			}
			if table.Hash[slot] == h {
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
	default:
		for i := range uses {
			use := &uses[i]
			if use.Name == sym {
				return *use, i, true
			}
		}
		return runtime.AttrUse{}, -1, false
	}
}

func (s *Session) globalAttributeBySymbol(sym runtime.SymbolID) (runtime.AttrID, bool) {
	if sym == 0 {
		return 0, false
	}
	if s.rt == nil || int(sym) >= len(s.rt.GlobalAttributes) {
		return 0, false
	}
	id := s.rt.GlobalAttributes[sym]
	return id, id != 0
}
