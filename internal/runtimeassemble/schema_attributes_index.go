package runtimeassemble

import "github.com/jacoelho/xsd/internal/runtime"

const (
	attrIndexLinearLimit = 8
	attrIndexBinaryLimit = 64
)

func buildAttrHashTable(uses []runtime.AttrUse, off uint32) runtime.AttrHashTable {
	size := max(runtime.NextPow2(len(uses)*2), 1)
	table := runtime.AttrHashTable{
		Hash: make([]uint64, size),
		Slot: make([]uint32, size),
	}
	mask := uint64(size - 1)
	for i := range uses {
		use := &uses[i]
		h := uint64(use.Name)
		if h == 0 {
			h = 1
		}
		slot := int(h & mask)
		for {
			if table.Slot[slot] == 0 {
				table.Hash[slot] = h
				table.Slot[slot] = off + uint32(i) + 1
				break
			}
			slot = (slot + 1) & int(mask)
		}
	}
	return table
}
