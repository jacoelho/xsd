package validator

import "github.com/jacoelho/xsd/internal/runtime"

type attrDupState struct {
	table   []attrSeenEntry
	mask    uint64
	useHash bool
}

func (s *Session) prepareAttrDupState(attrCount int, checkDuplicates bool) attrDupState {
	if !checkDuplicates || attrCount <= smallAttrDupThreshold {
		return attrDupState{}
	}

	size := runtime.NextPow2(attrCount * 2)
	table := s.attrSeenTable
	if cap(table) < size {
		table = make([]attrSeenEntry, size)
	} else {
		table = table[:size]
		clear(table)
	}
	return attrDupState{
		table:   table,
		mask:    uint64(size - 1),
		useHash: true,
	}
}

func (s *Session) finalizeAttrDupState(state attrDupState) {
	if state.useHash {
		s.attrSeenTable = state.table
	}
}

func (s *Session) hasDuplicateAttrAt(attrs []StartAttr, i int, state *attrDupState) bool {
	if state == nil || !state.useHash {
		return s.hasDuplicateAttrAtLinear(attrs, i)
	}
	return s.hasDuplicateAttrAtHash(attrs, i, state)
}

func (s *Session) hasDuplicateAttrAtLinear(attrs []StartAttr, i int) bool {
	for j := range i {
		if s.attrNamesEqual(&attrs[j], &attrs[i]) {
			return true
		}
	}
	return false
}

func (s *Session) hasDuplicateAttrAtHash(attrs []StartAttr, i int, state *attrDupState) bool {
	if state == nil || !state.useHash {
		return false
	}

	nsBytes := attrNSBytes(s.rt, &attrs[i])
	hash := attrNameHash(nsBytes, attrs[i].Local)
	slot := int(hash & state.mask)
	for {
		entry := state.table[slot]
		if entry.hash == 0 {
			state.table[slot] = attrSeenEntry{hash: hash, idx: uint32(i)}
			return false
		}
		if entry.hash == hash && s.attrNamesEqual(&attrs[int(entry.idx)], &attrs[i]) {
			return true
		}
		slot = (slot + 1) & int(state.mask)
	}
}
