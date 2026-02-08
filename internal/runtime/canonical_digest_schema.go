package runtime

func digestTypes(h *digestBuilder, types []Type) {
	h.u32(uint32(len(types)))
	for _, t := range types {
		h.u8(uint8(t.Kind))
		h.u32(uint32(t.Name))
		h.u32(uint32(t.Flags))
		h.u32(uint32(t.Base))
		h.u8(uint8(t.Derivation))
		h.u8(uint8(t.Final))
		h.u8(uint8(t.Block))
		h.u32(t.AncOff)
		h.u32(t.AncLen)
		h.u32(t.AncMaskOff)
		h.u32(uint32(t.Validator))
		h.u32(t.Complex.ID)
	}
}

func digestAncestors(h *digestBuilder, ancestors TypeAncestors) {
	digestTypeIDs(h, ancestors.IDs)
	h.u32(uint32(len(ancestors.Masks)))
	for _, m := range ancestors.Masks {
		h.u8(uint8(m))
	}
}

func digestComplexTypes(h *digestBuilder, types []ComplexType) {
	h.u32(uint32(len(types)))
	for _, ct := range types {
		h.u8(uint8(ct.Content))
		digestAttrIndexRef(h, ct.Attrs)
		h.u32(uint32(ct.AnyAttr))
		h.u32(uint32(ct.TextValidator))
		digestValueRef(h, ct.TextFixed)
		digestValueRef(h, ct.TextDefault)
		h.u32(uint32(ct.TextFixedMember))
		h.u32(uint32(ct.TextDefaultMember))
		digestModelRef(h, ct.Model)
		h.bool(ct.Mixed)
	}
}

func digestElements(h *digestBuilder, elems []Element) {
	h.u32(uint32(len(elems)))
	for i := range elems {
		e := &elems[i]
		h.u32(uint32(e.Name))
		h.u32(uint32(e.Type))
		h.u32(uint32(e.SubstHead))
		digestValueRef(h, e.Default)
		digestValueRef(h, e.Fixed)
		digestValueKeyRef(h, e.DefaultKey)
		digestValueKeyRef(h, e.FixedKey)
		h.u32(uint32(e.DefaultMember))
		h.u32(uint32(e.FixedMember))
		h.u32(uint32(e.Flags))
		h.u8(uint8(e.Block))
		h.u8(uint8(e.Final))
		h.u32(e.ICOff)
		h.u32(e.ICLen)
	}
}

func digestAttributes(h *digestBuilder, attrs []Attribute) {
	h.u32(uint32(len(attrs)))
	for i := range attrs {
		a := &attrs[i]
		h.u32(uint32(a.Name))
		h.u32(uint32(a.Validator))
		digestValueRef(h, a.Default)
		digestValueRef(h, a.Fixed)
		digestValueKeyRef(h, a.DefaultKey)
		digestValueKeyRef(h, a.FixedKey)
		h.u32(uint32(a.DefaultMember))
		h.u32(uint32(a.FixedMember))
	}
}

func digestAttrIndex(h *digestBuilder, idx ComplexAttrIndex) {
	h.u32(uint32(len(idx.Uses)))
	for i := range idx.Uses {
		use := &idx.Uses[i]
		h.u32(uint32(use.Name))
		h.u32(uint32(use.Validator))
		h.u8(uint8(use.Use))
		digestValueRef(h, use.Default)
		digestValueRef(h, use.Fixed)
		digestValueKeyRef(h, use.DefaultKey)
		digestValueKeyRef(h, use.FixedKey)
		h.u32(uint32(use.DefaultMember))
		h.u32(uint32(use.FixedMember))
	}
	h.u32(uint32(len(idx.HashTables)))
	for _, table := range idx.HashTables {
		digestU64Slice(h, table.Hash)
		digestU32Slice(h, table.Slot)
	}
}
