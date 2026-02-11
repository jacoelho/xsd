package runtime

func digestAttrIndexRef(h FingerprintWriter, ref AttrIndexRef) {
	h.WriteU32(ref.Off)
	h.WriteU32(ref.Len)
	h.WriteU8(uint8(ref.Mode))
	h.WriteU32(ref.HashTable)
}

func digestModelRef(h FingerprintWriter, ref ModelRef) {
	h.WriteU8(uint8(ref.Kind))
	h.WriteU32(ref.ID)
}

func digestBitsetRef(h FingerprintWriter, ref BitsetRef) {
	h.WriteU32(ref.Off)
	h.WriteU32(ref.Len)
}

func digestValueRef(h FingerprintWriter, ref ValueRef) {
	h.WriteU32(ref.Off)
	h.WriteU32(ref.Len)
	h.WriteU64(ref.Hash)
	h.WriteBool(ref.Present)
}

func digestValueKeyRef(h FingerprintWriter, ref ValueKeyRef) {
	h.WriteU8(uint8(ref.Kind))
	digestValueRef(h, ref.Ref)
}

func digestU32Slice(h FingerprintWriter, vals []uint32) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(v)
	}
}

func digestU64Slice(h FingerprintWriter, vals []uint64) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU64(v)
	}
}

func digestU8Slice(h FingerprintWriter, vals []uint8) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU8(v)
	}
}

func digestNamespaceIDs(h FingerprintWriter, vals []NamespaceID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}

func digestSymbolIDs(h FingerprintWriter, vals []SymbolID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}

func digestTypeIDs(h FingerprintWriter, vals []TypeID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}

func digestElemIDs(h FingerprintWriter, vals []ElemID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}

func digestAttrIDs(h FingerprintWriter, vals []AttrID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}

func digestValidatorIDs(h FingerprintWriter, vals []ValidatorID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}

func digestICIDs(h FingerprintWriter, vals []ICID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}

func digestPathIDs(h FingerprintWriter, vals []PathID) {
	h.WriteU32(uint32(len(vals)))
	for _, v := range vals {
		h.WriteU32(uint32(v))
	}
}
