package runtime

func digestNamespaces(h FingerprintWriter, table *NamespaceTable) {
	if table == nil {
		return
	}
	h.WriteBytes(table.Blob)
	digestU32Slice(h, table.Off)
	digestU32Slice(h, table.Len)
	digestU64Slice(h, table.Index.Hash)
	digestNamespaceIDs(h, table.Index.ID)
}

func digestSymbols(h FingerprintWriter, table *SymbolsTable) {
	if table == nil {
		return
	}
	digestNamespaceIDs(h, table.NS)
	digestU32Slice(h, table.LocalOff)
	digestU32Slice(h, table.LocalLen)
	h.WriteBytes(table.LocalBlob)
	digestU64Slice(h, table.Index.Hash)
	digestSymbolIDs(h, table.Index.ID)
}

func digestPredef(h FingerprintWriter, pre PredefinedSymbols, preNS PredefinedNamespaces, builtin BuiltinIDs, policy RootPolicy) {
	h.WriteU32(uint32(pre.XsiType))
	h.WriteU32(uint32(pre.XsiNil))
	h.WriteU32(uint32(pre.XsiSchemaLocation))
	h.WriteU32(uint32(pre.XsiNoNamespaceSchemaLocation))
	h.WriteU32(uint32(pre.XmlLang))
	h.WriteU32(uint32(pre.XmlSpace))
	h.WriteU32(uint32(preNS.Xsi))
	h.WriteU32(uint32(preNS.Xml))
	h.WriteU32(uint32(preNS.Empty))
	h.WriteU32(uint32(builtin.AnyType))
	h.WriteU32(uint32(builtin.AnySimpleType))
	h.WriteU8(uint8(policy))
}

func digestGlobalIndices(h FingerprintWriter, types []TypeID, elems []ElemID, attrs []AttrID) {
	digestTypeIDs(h, types)
	digestElemIDs(h, elems)
	digestAttrIDs(h, attrs)
}
