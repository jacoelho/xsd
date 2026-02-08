package runtime

import "fmt"

type namespaceBuilder struct {
	index map[string]NamespaceID
	blob  []byte
	off   []uint32
	len   []uint32
}

func newNamespaceBuilder() namespaceBuilder {
	return namespaceBuilder{
		off:   make([]uint32, 1),
		len:   make([]uint32, 1),
		index: make(map[string]NamespaceID),
	}
}

func (b *namespaceBuilder) intern(uri []byte) NamespaceID {
	key := string(uri)
	if id, ok := b.index[key]; ok {
		return id
	}
	id := NamespaceID(len(b.off))
	b.index[key] = id
	b.off = append(b.off, uint32(len(b.blob)))
	b.len = append(b.len, uint32(len(uri)))
	b.blob = append(b.blob, uri...)
	return id
}

func (b *namespaceBuilder) build() (NamespaceTable, error) {
	out := NamespaceTable{
		Blob: b.blob,
		Off:  b.off,
		Len:  b.len,
	}
	index, err := buildNamespaceIndex(&out)
	if err != nil {
		return NamespaceTable{}, err
	}
	out.Index = index
	return out, nil
}

func buildNamespaceIndex(table *NamespaceTable) (NamespaceIndex, error) {
	if table == nil {
		return NamespaceIndex{}, nil
	}
	count := len(table.Off) - 1
	if count <= 0 {
		return NamespaceIndex{}, nil
	}
	hashes, ids, err := buildOpenAddressIndex(count, "namespace", func(id NamespaceID) (uint64, error) {
		if int(id) >= len(table.Off) || int(id) >= len(table.Len) {
			return 0, fmt.Errorf("namespace table id out of range")
		}
		off := table.Off[id]
		ln := table.Len[id]
		end := uint64(off) + uint64(ln)
		if end > uint64(len(table.Blob)) {
			return 0, fmt.Errorf("namespace blob bounds exceeded")
		}
		return hashBytes(table.Blob[int(off):int(end)]), nil
	})
	if err != nil {
		return NamespaceIndex{}, err
	}
	return NamespaceIndex{Hash: hashes, ID: ids}, nil
}
