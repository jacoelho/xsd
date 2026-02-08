package runtime

func digestValidators(h *digestBuilder, bundle *ValidatorsBundle) {
	if bundle == nil {
		return
	}
	h.u32(uint32(len(bundle.String)))
	for _, v := range bundle.String {
		h.u8(uint8(v.Kind))
	}
	h.u32(uint32(len(bundle.Boolean)))
	h.u32(uint32(len(bundle.Decimal)))
	h.u32(uint32(len(bundle.Integer)))
	for _, v := range bundle.Integer {
		h.u8(uint8(v.Kind))
	}
	h.u32(uint32(len(bundle.Float)))
	h.u32(uint32(len(bundle.Double)))
	h.u32(uint32(len(bundle.Duration)))
	h.u32(uint32(len(bundle.DateTime)))
	h.u32(uint32(len(bundle.Time)))
	h.u32(uint32(len(bundle.Date)))
	h.u32(uint32(len(bundle.GYearMonth)))
	h.u32(uint32(len(bundle.GYear)))
	h.u32(uint32(len(bundle.GMonthDay)))
	h.u32(uint32(len(bundle.GDay)))
	h.u32(uint32(len(bundle.GMonth)))
	h.u32(uint32(len(bundle.AnyURI)))
	h.u32(uint32(len(bundle.QName)))
	h.u32(uint32(len(bundle.Notation)))
	h.u32(uint32(len(bundle.HexBinary)))
	h.u32(uint32(len(bundle.Base64Binary)))
	h.u32(uint32(len(bundle.List)))
	for _, v := range bundle.List {
		h.u32(uint32(v.Item))
	}
	h.u32(uint32(len(bundle.Union)))
	for _, v := range bundle.Union {
		h.u32(v.MemberOff)
		h.u32(v.MemberLen)
	}
	digestValidatorIDs(h, bundle.UnionMembers)
	digestTypeIDs(h, bundle.UnionMemberTypes)
	digestU8Slice(h, bundle.UnionMemberSameWS)
	h.u32(uint32(len(bundle.Meta)))
	for _, meta := range bundle.Meta {
		h.u8(uint8(meta.Kind))
		h.u32(meta.Index)
		h.u8(uint8(meta.WhiteSpace))
		h.u8(uint8(meta.Flags))
		h.u32(meta.Facets.Off)
		h.u32(meta.Facets.Len)
	}
}

func digestFacets(h *digestBuilder, facets []FacetInstr) {
	h.u32(uint32(len(facets)))
	for _, f := range facets {
		h.u8(uint8(f.Op))
		h.u32(f.Arg0)
		h.u32(f.Arg1)
	}
}

func digestPatterns(h *digestBuilder, patterns []Pattern) {
	h.u32(uint32(len(patterns)))
	for _, p := range patterns {
		h.bytes(p.Source)
	}
}

func digestEnums(h *digestBuilder, enums *EnumTable) {
	if enums == nil {
		return
	}
	digestU32Slice(h, enums.Off)
	digestU32Slice(h, enums.Len)
	h.u32(uint32(len(enums.Keys)))
	for _, key := range enums.Keys {
		h.u8(uint8(key.Kind))
		h.u64(key.Hash)
		h.bytes(key.Bytes)
	}
	digestU32Slice(h, enums.HashOff)
	digestU32Slice(h, enums.HashLen)
	digestU64Slice(h, enums.Hashes)
	digestU32Slice(h, enums.Slots)
}

func digestValues(h *digestBuilder, values ValueBlob) {
	h.bytes(values.Blob)
}
