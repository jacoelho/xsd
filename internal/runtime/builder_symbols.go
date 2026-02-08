package runtime

import "fmt"

type symbolKey struct {
	local string
	ns    NamespaceID
}

type symbolBuilder struct {
	index     map[symbolKey]SymbolID
	ns        []NamespaceID
	localOff  []uint32
	localLen  []uint32
	localBlob []byte
}

func newSymbolBuilder() symbolBuilder {
	return symbolBuilder{
		ns:       make([]NamespaceID, 1),
		localOff: make([]uint32, 1),
		localLen: make([]uint32, 1),
		index:    make(map[symbolKey]SymbolID),
	}
}

func (b *symbolBuilder) intern(nsID NamespaceID, local []byte) SymbolID {
	key := symbolKey{ns: nsID, local: string(local)}
	if id, ok := b.index[key]; ok {
		return id
	}
	id := SymbolID(len(b.ns))
	b.index[key] = id
	b.ns = append(b.ns, nsID)
	b.localOff = append(b.localOff, uint32(len(b.localBlob)))
	b.localLen = append(b.localLen, uint32(len(local)))
	b.localBlob = append(b.localBlob, local...)
	return id
}

func (b *symbolBuilder) build() (SymbolsTable, error) {
	out := SymbolsTable{
		NS:        b.ns,
		LocalOff:  b.localOff,
		LocalLen:  b.localLen,
		LocalBlob: b.localBlob,
	}
	index, err := buildSymbolsIndex(&out)
	if err != nil {
		return SymbolsTable{}, err
	}
	out.Index = index
	return out, nil
}

func buildSymbolsIndex(table *SymbolsTable) (SymbolsIndex, error) {
	if table == nil {
		return SymbolsIndex{}, nil
	}
	count := len(table.NS) - 1
	if count <= 0 {
		return SymbolsIndex{}, nil
	}
	hashes, ids, err := buildOpenAddressIndex(count, "symbol", func(id SymbolID) (uint64, error) {
		if int(id) >= len(table.NS) || int(id) >= len(table.LocalOff) || int(id) >= len(table.LocalLen) {
			return 0, fmt.Errorf("symbol table id out of range")
		}
		nsID := table.NS[id]
		off := table.LocalOff[id]
		ln := table.LocalLen[id]
		end := uint64(off) + uint64(ln)
		if end > uint64(len(table.LocalBlob)) {
			return 0, fmt.Errorf("symbol blob bounds exceeded")
		}
		return hashSymbol(nsID, table.LocalBlob[int(off):int(end)]), nil
	})
	if err != nil {
		return SymbolsIndex{}, err
	}
	return SymbolsIndex{Hash: hashes, ID: ids}, nil
}
