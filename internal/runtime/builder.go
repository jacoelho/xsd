package runtime

import (
	"fmt"

	xsdxml "github.com/jacoelho/xsd/internal/xml"
)

type Builder struct {
	namespaces namespaceBuilder
	symbols    symbolBuilder
	sealed     bool

	emptyNS NamespaceID
	xmlNS   NamespaceID
	xsiNS   NamespaceID

	predef PredefinedSymbols
}

func NewBuilder() *Builder {
	b := &Builder{
		namespaces: newNamespaceBuilder(),
		symbols:    newSymbolBuilder(),
	}
	b.emptyNS = b.namespaces.intern(nil)
	b.xmlNS = b.namespaces.intern([]byte(xsdxml.XMLNamespace))
	b.xsiNS = b.namespaces.intern([]byte(xsdxml.XSINamespace))

	b.predef = PredefinedSymbols{
		XsiType:                      b.symbols.intern(b.xsiNS, []byte("type")),
		XsiNil:                       b.symbols.intern(b.xsiNS, []byte("nil")),
		XsiSchemaLocation:            b.symbols.intern(b.xsiNS, []byte("schemaLocation")),
		XsiNoNamespaceSchemaLocation: b.symbols.intern(b.xsiNS, []byte("noNamespaceSchemaLocation")),
		XmlLang:                      b.symbols.intern(b.xmlNS, []byte("lang")),
		XmlSpace:                     b.symbols.intern(b.xmlNS, []byte("space")),
	}

	return b
}

func (b *Builder) InternNamespace(uri []byte) (NamespaceID, error) {
	if err := b.ensureMutable(); err != nil {
		return 0, err
	}
	return b.namespaces.intern(uri), nil
}

func (b *Builder) InternSymbol(nsID NamespaceID, local []byte) (SymbolID, error) {
	if err := b.ensureMutable(); err != nil {
		return 0, err
	}
	return b.symbols.intern(nsID, local), nil
}

func (b *Builder) Build() (*Schema, error) {
	if err := b.ensureMutable(); err != nil {
		return nil, err
	}
	b.sealed = true
	namespaces, err := b.namespaces.build()
	if err != nil {
		return nil, fmt.Errorf("runtime build: namespaces: %w", err)
	}
	symbols, err := b.symbols.build()
	if err != nil {
		return nil, fmt.Errorf("runtime build: symbols: %w", err)
	}
	return &Schema{
		Namespaces: namespaces,
		Symbols:    symbols,
		Predef:     b.predef,
		PredefNS: PredefinedNamespaces{
			Empty: b.emptyNS,
			Xml:   b.xmlNS,
			Xsi:   b.xsiNS,
		},
	}, nil
}

func (b *Builder) ensureMutable() error {
	if b.sealed {
		return fmt.Errorf("runtime.Builder used after Build")
	}
	return nil
}

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

func buildNamespaceIndex(table *NamespaceTable) (NamespaceIndex, error) {
	if table == nil {
		return NamespaceIndex{}, nil
	}
	count := len(table.Off) - 1
	if count <= 0 {
		return NamespaceIndex{}, nil
	}
	size := NextPow2(count * 2)
	index := NamespaceIndex{
		Hash: make([]uint64, size),
		ID:   make([]NamespaceID, size),
	}
	mask := uint64(size - 1)
	for i := 1; i <= count; i++ {
		id := NamespaceID(i)
		if int(id) >= len(table.Off) || int(id) >= len(table.Len) {
			return NamespaceIndex{}, fmt.Errorf("namespace table id out of range")
		}
		off := table.Off[id]
		ln := table.Len[id]
		end := uint64(off) + uint64(ln)
		if end > uint64(len(table.Blob)) {
			return NamespaceIndex{}, fmt.Errorf("namespace blob bounds exceeded")
		}
		b := table.Blob[int(off):int(end)]
		h := hashBytes(b)
		slot := int(h & mask)
		inserted := false
		for range size {
			if index.ID[slot] == 0 {
				index.ID[slot] = id
				index.Hash[slot] = h
				inserted = true
				break
			}
			slot = int((uint64(slot) + 1) & mask)
		}
		if !inserted {
			return NamespaceIndex{}, fmt.Errorf("namespace index table full")
		}
	}
	return index, nil
}

func buildSymbolsIndex(table *SymbolsTable) (SymbolsIndex, error) {
	if table == nil {
		return SymbolsIndex{}, nil
	}
	count := len(table.NS) - 1
	if count <= 0 {
		return SymbolsIndex{}, nil
	}
	size := NextPow2(count * 2)
	index := SymbolsIndex{
		Hash: make([]uint64, size),
		ID:   make([]SymbolID, size),
	}
	mask := uint64(size - 1)
	for i := 1; i <= count; i++ {
		id := SymbolID(i)
		if int(id) >= len(table.NS) || int(id) >= len(table.LocalOff) || int(id) >= len(table.LocalLen) {
			return SymbolsIndex{}, fmt.Errorf("symbol table id out of range")
		}
		nsID := table.NS[id]
		off := table.LocalOff[id]
		ln := table.LocalLen[id]
		end := uint64(off) + uint64(ln)
		if end > uint64(len(table.LocalBlob)) {
			return SymbolsIndex{}, fmt.Errorf("symbol blob bounds exceeded")
		}
		local := table.LocalBlob[int(off):int(end)]
		h := hashSymbol(nsID, local)
		slot := int(h & mask)
		inserted := false
		for range size {
			if index.ID[slot] == 0 {
				index.ID[slot] = id
				index.Hash[slot] = h
				inserted = true
				break
			}
			slot = int((uint64(slot) + 1) & mask)
		}
		if !inserted {
			return SymbolsIndex{}, fmt.Errorf("symbol index table full")
		}
	}
	return index, nil
}
