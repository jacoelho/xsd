package runtime

func digestAttrIndexRef(h *digestBuilder, ref AttrIndexRef) {
	h.u32(ref.Off)
	h.u32(ref.Len)
	h.u8(uint8(ref.Mode))
	h.u32(ref.HashTable)
}

func digestModelRef(h *digestBuilder, ref ModelRef) {
	h.u8(uint8(ref.Kind))
	h.u32(ref.ID)
}

func digestBitsetRef(h *digestBuilder, ref BitsetRef) {
	h.u32(ref.Off)
	h.u32(ref.Len)
}

func digestValueRef(h *digestBuilder, ref ValueRef) {
	h.u32(ref.Off)
	h.u32(ref.Len)
	h.u64(ref.Hash)
	h.bool(ref.Present)
}

func digestValueKeyRef(h *digestBuilder, ref ValueKeyRef) {
	h.u8(uint8(ref.Kind))
	digestValueRef(h, ref.Ref)
}

func digestU32Slice(h *digestBuilder, vals []uint32) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(v)
	}
}

func digestU64Slice(h *digestBuilder, vals []uint64) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u64(v)
	}
}

func digestU8Slice(h *digestBuilder, vals []uint8) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u8(v)
	}
}

func digestNamespaceIDs(h *digestBuilder, vals []NamespaceID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func digestSymbolIDs(h *digestBuilder, vals []SymbolID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func digestTypeIDs(h *digestBuilder, vals []TypeID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func digestElemIDs(h *digestBuilder, vals []ElemID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func digestAttrIDs(h *digestBuilder, vals []AttrID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func digestValidatorIDs(h *digestBuilder, vals []ValidatorID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func digestICIDs(h *digestBuilder, vals []ICID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}

func digestPathIDs(h *digestBuilder, vals []PathID) {
	h.u32(uint32(len(vals)))
	for _, v := range vals {
		h.u32(uint32(v))
	}
}
