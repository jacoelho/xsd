package runtime

func digestNamespaces(h *digestBuilder, table *NamespaceTable) {
	if table == nil {
		return
	}
	h.bytes(table.Blob)
	digestU32Slice(h, table.Off)
	digestU32Slice(h, table.Len)
	digestU64Slice(h, table.Index.Hash)
	digestNamespaceIDs(h, table.Index.ID)
}

func digestSymbols(h *digestBuilder, table *SymbolsTable) {
	if table == nil {
		return
	}
	digestNamespaceIDs(h, table.NS)
	digestU32Slice(h, table.LocalOff)
	digestU32Slice(h, table.LocalLen)
	h.bytes(table.LocalBlob)
	digestU64Slice(h, table.Index.Hash)
	digestSymbolIDs(h, table.Index.ID)
}

func digestPredef(h *digestBuilder, pre PredefinedSymbols, preNS PredefinedNamespaces, builtin BuiltinIDs, policy RootPolicy) {
	h.u32(uint32(pre.XsiType))
	h.u32(uint32(pre.XsiNil))
	h.u32(uint32(pre.XsiSchemaLocation))
	h.u32(uint32(pre.XsiNoNamespaceSchemaLocation))
	h.u32(uint32(pre.XmlLang))
	h.u32(uint32(pre.XmlSpace))
	h.u32(uint32(preNS.Xsi))
	h.u32(uint32(preNS.Xml))
	h.u32(uint32(preNS.Empty))
	h.u32(uint32(builtin.AnyType))
	h.u32(uint32(builtin.AnySimpleType))
	h.u8(uint8(policy))
}

func digestGlobalIndices(h *digestBuilder, types []TypeID, elems []ElemID, attrs []AttrID) {
	digestTypeIDs(h, types)
	digestElemIDs(h, elems)
	digestAttrIDs(h, attrs)
}
