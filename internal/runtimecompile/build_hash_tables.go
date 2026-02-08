package runtimecompile

import "github.com/jacoelho/xsd/internal/runtime"

func hashNamespaces(h *hashBuilder, table *runtime.NamespaceTable) {
	if table == nil {
		return
	}
	h.bytes(table.Blob)
	hashU32Slice(h, table.Off)
	hashU32Slice(h, table.Len)
	hashU64Slice(h, table.Index.Hash)
	hashNamespaceIDs(h, table.Index.ID)
}

func hashSymbols(h *hashBuilder, table *runtime.SymbolsTable) {
	if table == nil {
		return
	}
	hashNamespaceIDs(h, table.NS)
	hashU32Slice(h, table.LocalOff)
	hashU32Slice(h, table.LocalLen)
	h.bytes(table.LocalBlob)
	hashU64Slice(h, table.Index.Hash)
	hashSymbolIDs(h, table.Index.ID)
}

func hashPredef(h *hashBuilder, pre runtime.PredefinedSymbols, preNS runtime.PredefinedNamespaces, builtin runtime.BuiltinIDs, policy runtime.RootPolicy) {
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

func hashGlobalIndices(h *hashBuilder, types []runtime.TypeID, elems []runtime.ElemID, attrs []runtime.AttrID) {
	hashTypeIDs(h, types)
	hashElemIDs(h, elems)
	hashAttrIDs(h, attrs)
}
