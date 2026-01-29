package runtime

import "bytes"

type NamespaceTable struct {
	Blob  []byte
	Off   []uint32
	Len   []uint32
	Index NamespaceIndex
}

type NamespaceIndex struct {
	Hash []uint64
	ID   []NamespaceID
}

func (t *NamespaceTable) Count() int {
	if len(t.Off) == 0 {
		return 0
	}
	return len(t.Off) - 1
}

func (t *NamespaceTable) Bytes(id NamespaceID) []byte {
	if id == 0 || int(id) >= len(t.Off) {
		return nil
	}
	off := t.Off[id]
	ln := t.Len[id]
	if int(off+ln) > len(t.Blob) {
		return nil
	}
	return t.Blob[off : off+ln]
}

func (t *NamespaceTable) Lookup(ns []byte) NamespaceID {
	if len(t.Index.ID) == 0 {
		return 0
	}
	h := hashBytes(ns)
	mask := uint64(len(t.Index.ID) - 1)
	slot := int(h & mask)
	for i := 0; i < len(t.Index.ID); i++ {
		id := t.Index.ID[slot]
		if id == 0 {
			return 0
		}
		if t.Index.Hash[slot] == h && t.equalNamespace(id, ns) {
			return id
		}
		slot = int((uint64(slot) + 1) & mask)
	}
	return 0
}

func (t *NamespaceTable) equalNamespace(id NamespaceID, ns []byte) bool {
	if id == 0 || int(id) >= len(t.Off) || int(id) >= len(t.Len) {
		return false
	}
	off := t.Off[id]
	ln := t.Len[id]
	end := uint64(off) + uint64(ln)
	if end > uint64(len(t.Blob)) {
		return false
	}
	if int(ln) != len(ns) {
		return false
	}
	return bytes.Equal(t.Blob[int(off):int(end)], ns)
}

type SymbolsTable struct {
	NS        []NamespaceID
	LocalOff  []uint32
	LocalLen  []uint32
	LocalBlob []byte

	Index SymbolsIndex
}

type SymbolsIndex struct {
	Hash []uint64
	ID   []SymbolID
}

func (t *SymbolsTable) Count() int {
	if len(t.NS) == 0 {
		return 0
	}
	return len(t.NS) - 1
}

func (t *SymbolsTable) LocalBytes(id SymbolID) []byte {
	if id == 0 || int(id) >= len(t.LocalOff) {
		return nil
	}
	off := t.LocalOff[id]
	ln := t.LocalLen[id]
	if int(off+ln) > len(t.LocalBlob) {
		return nil
	}
	return t.LocalBlob[off : off+ln]
}

func (t *SymbolsTable) Lookup(nsID NamespaceID, local []byte) SymbolID {
	if len(t.Index.ID) == 0 {
		return 0
	}
	h := hashSymbol(nsID, local)
	mask := uint64(len(t.Index.ID) - 1)
	slot := int(h & mask)
	for i := 0; i < len(t.Index.ID); i++ {
		id := t.Index.ID[slot]
		if id == 0 {
			return 0
		}
		if t.Index.Hash[slot] == h && t.equalSymbol(id, nsID, local) {
			return id
		}
		slot = int((uint64(slot) + 1) & mask)
	}
	return 0
}

func (t *SymbolsTable) equalSymbol(id SymbolID, nsID NamespaceID, local []byte) bool {
	if t.NS[id] != nsID {
		return false
	}
	localStored := t.LocalBytes(id)
	return bytes.Equal(localStored, local)
}
