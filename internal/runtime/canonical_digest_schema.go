package runtime

func digestTypes(h FingerprintWriter, types []Type) {
	h.WriteU32(uint32(len(types)))
	for _, t := range types {
		h.WriteU8(uint8(t.Kind))
		h.WriteU32(uint32(t.Name))
		h.WriteU32(uint32(t.Flags))
		h.WriteU32(uint32(t.Base))
		h.WriteU8(uint8(t.Derivation))
		h.WriteU8(uint8(t.Final))
		h.WriteU8(uint8(t.Block))
		h.WriteU32(t.AncOff)
		h.WriteU32(t.AncLen)
		h.WriteU32(t.AncMaskOff)
		h.WriteU32(uint32(t.Validator))
		h.WriteU32(t.Complex.ID)
	}
}

func digestAncestors(h FingerprintWriter, ancestors TypeAncestors) {
	digestTypeIDs(h, ancestors.IDs)
	h.WriteU32(uint32(len(ancestors.Masks)))
	for _, m := range ancestors.Masks {
		h.WriteU8(uint8(m))
	}
}

func digestComplexTypes(h FingerprintWriter, types []ComplexType) {
	h.WriteU32(uint32(len(types)))
	for _, ct := range types {
		h.WriteU8(uint8(ct.Content))
		digestAttrIndexRef(h, ct.Attrs)
		h.WriteU32(uint32(ct.AnyAttr))
		h.WriteU32(uint32(ct.TextValidator))
		digestValueRef(h, ct.TextFixed)
		digestValueRef(h, ct.TextDefault)
		h.WriteU32(uint32(ct.TextFixedMember))
		h.WriteU32(uint32(ct.TextDefaultMember))
		digestModelRef(h, ct.Model)
		h.WriteBool(ct.Mixed)
	}
}

func digestElements(h FingerprintWriter, elems []Element) {
	h.WriteU32(uint32(len(elems)))
	for i := range elems {
		e := &elems[i]
		h.WriteU32(uint32(e.Name))
		h.WriteU32(uint32(e.Type))
		h.WriteU32(uint32(e.SubstHead))
		digestValueRef(h, e.Default)
		digestValueRef(h, e.Fixed)
		digestValueKeyRef(h, e.DefaultKey)
		digestValueKeyRef(h, e.FixedKey)
		h.WriteU32(uint32(e.DefaultMember))
		h.WriteU32(uint32(e.FixedMember))
		h.WriteU32(uint32(e.Flags))
		h.WriteU8(uint8(e.Block))
		h.WriteU8(uint8(e.Final))
		h.WriteU32(e.ICOff)
		h.WriteU32(e.ICLen)
	}
}

func digestAttributes(h FingerprintWriter, attrs []Attribute) {
	h.WriteU32(uint32(len(attrs)))
	for i := range attrs {
		a := &attrs[i]
		h.WriteU32(uint32(a.Name))
		h.WriteU32(uint32(a.Validator))
		digestValueRef(h, a.Default)
		digestValueRef(h, a.Fixed)
		digestValueKeyRef(h, a.DefaultKey)
		digestValueKeyRef(h, a.FixedKey)
		h.WriteU32(uint32(a.DefaultMember))
		h.WriteU32(uint32(a.FixedMember))
	}
}

func digestAttrIndex(h FingerprintWriter, idx ComplexAttrIndex) {
	h.WriteU32(uint32(len(idx.Uses)))
	for i := range idx.Uses {
		use := &idx.Uses[i]
		h.WriteU32(uint32(use.Name))
		h.WriteU32(uint32(use.Validator))
		h.WriteU8(uint8(use.Use))
		digestValueRef(h, use.Default)
		digestValueRef(h, use.Fixed)
		digestValueKeyRef(h, use.DefaultKey)
		digestValueKeyRef(h, use.FixedKey)
		h.WriteU32(uint32(use.DefaultMember))
		h.WriteU32(uint32(use.FixedMember))
	}
	h.WriteU32(uint32(len(idx.HashTables)))
	for _, table := range idx.HashTables {
		digestU64Slice(h, table.Hash)
		digestU32Slice(h, table.Slot)
	}
}
