package runtimebuild

import "github.com/jacoelho/xsd/internal/runtime"

func hashTypes(h *hashBuilder, types []runtime.Type) {
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

func hashAncestors(h *hashBuilder, ancestors runtime.TypeAncestors) {
	hashTypeIDs(h, ancestors.IDs)
	h.u32(uint32(len(ancestors.Masks)))
	for _, m := range ancestors.Masks {
		h.u8(uint8(m))
	}
}

func hashComplexTypes(h *hashBuilder, types []runtime.ComplexType) {
	h.u32(uint32(len(types)))
	for _, ct := range types {
		h.u8(uint8(ct.Content))
		hashAttrIndexRef(h, ct.Attrs)
		h.u32(uint32(ct.AnyAttr))
		h.u32(uint32(ct.TextValidator))
		hashValueRef(h, ct.TextFixed)
		hashValueRef(h, ct.TextDefault)
		h.u32(uint32(ct.TextFixedMember))
		h.u32(uint32(ct.TextDefaultMember))
		hashModelRef(h, ct.Model)
		h.bool(ct.Mixed)
	}
}

func hashElements(h *hashBuilder, elems []runtime.Element) {
	h.u32(uint32(len(elems)))
	for i := range elems {
		e := &elems[i]
		h.u32(uint32(e.Name))
		h.u32(uint32(e.Type))
		h.u32(uint32(e.SubstHead))
		hashValueRef(h, e.Default)
		hashValueRef(h, e.Fixed)
		hashValueKeyRef(h, e.DefaultKey)
		hashValueKeyRef(h, e.FixedKey)
		h.u32(uint32(e.DefaultMember))
		h.u32(uint32(e.FixedMember))
		h.u32(uint32(e.Flags))
		h.u8(uint8(e.Block))
		h.u8(uint8(e.Final))
		h.u32(e.ICOff)
		h.u32(e.ICLen)
	}
}

func hashAttributes(h *hashBuilder, attrs []runtime.Attribute) {
	h.u32(uint32(len(attrs)))
	for i := range attrs {
		a := &attrs[i]
		h.u32(uint32(a.Name))
		h.u32(uint32(a.Validator))
		hashValueRef(h, a.Default)
		hashValueRef(h, a.Fixed)
		hashValueKeyRef(h, a.DefaultKey)
		hashValueKeyRef(h, a.FixedKey)
		h.u32(uint32(a.DefaultMember))
		h.u32(uint32(a.FixedMember))
	}
}

func hashAttrIndex(h *hashBuilder, idx runtime.ComplexAttrIndex) {
	h.u32(uint32(len(idx.Uses)))
	for i := range idx.Uses {
		use := &idx.Uses[i]
		h.u32(uint32(use.Name))
		h.u32(uint32(use.Validator))
		h.u8(uint8(use.Use))
		hashValueRef(h, use.Default)
		hashValueRef(h, use.Fixed)
		hashValueKeyRef(h, use.DefaultKey)
		hashValueKeyRef(h, use.FixedKey)
		h.u32(uint32(use.DefaultMember))
		h.u32(uint32(use.FixedMember))
	}
	h.u32(uint32(len(idx.HashTables)))
	for _, table := range idx.HashTables {
		hashU64Slice(h, table.Hash)
		hashU32Slice(h, table.Slot)
	}
}
