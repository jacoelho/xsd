package runtime

import xsdxml "github.com/jacoelho/xsd/internal/xml"

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

func (b *Builder) InternNamespace(uri []byte) NamespaceID {
	b.ensureMutable()
	return b.namespaces.intern(uri)
}

func (b *Builder) InternSymbol(nsID NamespaceID, local []byte) SymbolID {
	b.ensureMutable()
	return b.symbols.intern(nsID, local)
}

func (b *Builder) Build() *Schema {
	b.ensureMutable()
	b.sealed = true
	return &Schema{
		Namespaces: b.namespaces.build(),
		Symbols:    b.symbols.build(),
		Predef:     b.predef,
		PredefNS: PredefinedNamespaces{
			Empty: b.emptyNS,
			Xml:   b.xmlNS,
			Xsi:   b.xsiNS,
		},
	}
}

func (b *Builder) ensureMutable() {
	if b.sealed {
		panic("runtime.Builder used after Build")
	}
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

func (b *namespaceBuilder) build() NamespaceTable {
	out := NamespaceTable{
		Blob: b.blob,
		Off:  b.off,
		Len:  b.len,
	}
	out.Index = buildNamespaceIndex(&out)
	return out
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

func (b *symbolBuilder) build() SymbolsTable {
	out := SymbolsTable{
		NS:        b.ns,
		LocalOff:  b.localOff,
		LocalLen:  b.localLen,
		LocalBlob: b.localBlob,
	}
	out.Index = buildSymbolsIndex(&out)
	return out
}

func buildNamespaceIndex(table *NamespaceTable) NamespaceIndex {
	if table == nil {
		return NamespaceIndex{}
	}
	count := len(table.Off) - 1
	if count <= 0 {
		return NamespaceIndex{}
	}
	size := NextPow2(count * 2)
	index := NamespaceIndex{
		Hash: make([]uint64, size),
		ID:   make([]NamespaceID, size),
	}
	mask := uint64(size - 1)
	for i := 1; i <= count; i++ {
		id := NamespaceID(i)
		off := table.Off[id]
		ln := table.Len[id]
		b := table.Blob[off : off+ln]
		h := hashBytes(b)
		slot := int(h & mask)
		inserted := false
		for probes := 0; probes < size; probes++ {
			if index.ID[slot] == 0 {
				index.ID[slot] = id
				index.Hash[slot] = h
				inserted = true
				break
			}
			slot = int((uint64(slot) + 1) & mask)
		}
		if !inserted {
			panic("namespace index table full")
		}
	}
	return index
}

func buildSymbolsIndex(table *SymbolsTable) SymbolsIndex {
	if table == nil {
		return SymbolsIndex{}
	}
	count := len(table.NS) - 1
	if count <= 0 {
		return SymbolsIndex{}
	}
	size := NextPow2(count * 2)
	index := SymbolsIndex{
		Hash: make([]uint64, size),
		ID:   make([]SymbolID, size),
	}
	mask := uint64(size - 1)
	for i := 1; i <= count; i++ {
		id := SymbolID(i)
		nsID := table.NS[id]
		off := table.LocalOff[id]
		ln := table.LocalLen[id]
		local := table.LocalBlob[off : off+ln]
		h := hashSymbol(nsID, local)
		slot := int(h & mask)
		inserted := false
		for probes := 0; probes < size; probes++ {
			if index.ID[slot] == 0 {
				index.ID[slot] = id
				index.Hash[slot] = h
				inserted = true
				break
			}
			slot = int((uint64(slot) + 1) & mask)
		}
		if !inserted {
			panic("symbol index table full")
		}
	}
	return index
}
