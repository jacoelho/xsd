package runtimeassemble

import "github.com/jacoelho/xsd/internal/runtime"

func hashAttrIndexRef(h *hashBuilder, ref runtime.AttrIndexRef) {
	h.u32(ref.Off)
	h.u32(ref.Len)
	h.u8(uint8(ref.Mode))
	h.u32(ref.HashTable)
}

func hashModelRef(h *hashBuilder, ref runtime.ModelRef) {
	h.u8(uint8(ref.Kind))
	h.u32(ref.ID)
}

func hashBitsetRef(h *hashBuilder, ref runtime.BitsetRef) {
	h.u32(ref.Off)
	h.u32(ref.Len)
}

func hashValueRef(h *hashBuilder, ref runtime.ValueRef) {
	h.u32(ref.Off)
	h.u32(ref.Len)
	h.u64(ref.Hash)
	h.bool(ref.Present)
}

func hashValueKeyRef(h *hashBuilder, ref runtime.ValueKeyRef) {
	h.u8(uint8(ref.Kind))
	hashValueRef(h, ref.Ref)
}

func hashU32Slice(h *hashBuilder, vals []uint32) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(v)
	}
}

func hashU64Slice(h *hashBuilder, vals []uint64) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u64(v)
	}
}

func hashU8Slice(h *hashBuilder, vals []uint8) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u8(v)
	}
}

func hashNamespaceIDs(h *hashBuilder, vals []runtime.NamespaceID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func hashSymbolIDs(h *hashBuilder, vals []runtime.SymbolID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func hashTypeIDs(h *hashBuilder, vals []runtime.TypeID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func hashElemIDs(h *hashBuilder, vals []runtime.ElemID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func hashAttrIDs(h *hashBuilder, vals []runtime.AttrID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func hashValidatorIDs(h *hashBuilder, vals []runtime.ValidatorID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func hashICIDs(h *hashBuilder, vals []runtime.ICID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func hashPathIDs(h *hashBuilder, vals []runtime.PathID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}
