package runtime

func digestValidators(h FingerprintWriter, bundle *ValidatorsBundle) {
	if bundle == nil {
		return
	}
	h.WriteU32(uint32(len(bundle.String)))
	for _, v := range bundle.String {
		h.WriteU8(uint8(v.Kind))
	}
	h.WriteU32(uint32(len(bundle.Boolean)))
	h.WriteU32(uint32(len(bundle.Decimal)))
	h.WriteU32(uint32(len(bundle.Integer)))
	for _, v := range bundle.Integer {
		h.WriteU8(uint8(v.Kind))
	}
	h.WriteU32(uint32(len(bundle.Float)))
	h.WriteU32(uint32(len(bundle.Double)))
	h.WriteU32(uint32(len(bundle.Duration)))
	h.WriteU32(uint32(len(bundle.DateTime)))
	h.WriteU32(uint32(len(bundle.Time)))
	h.WriteU32(uint32(len(bundle.Date)))
	h.WriteU32(uint32(len(bundle.GYearMonth)))
	h.WriteU32(uint32(len(bundle.GYear)))
	h.WriteU32(uint32(len(bundle.GMonthDay)))
	h.WriteU32(uint32(len(bundle.GDay)))
	h.WriteU32(uint32(len(bundle.GMonth)))
	h.WriteU32(uint32(len(bundle.AnyURI)))
	h.WriteU32(uint32(len(bundle.QName)))
	h.WriteU32(uint32(len(bundle.Notation)))
	h.WriteU32(uint32(len(bundle.HexBinary)))
	h.WriteU32(uint32(len(bundle.Base64Binary)))
	h.WriteU32(uint32(len(bundle.List)))
	for _, v := range bundle.List {
		h.WriteU32(uint32(v.Item))
	}
	h.WriteU32(uint32(len(bundle.Union)))
	for _, v := range bundle.Union {
		h.WriteU32(v.MemberOff)
		h.WriteU32(v.MemberLen)
	}
	digestValidatorIDs(h, bundle.UnionMembers)
	digestTypeIDs(h, bundle.UnionMemberTypes)
	digestU8Slice(h, bundle.UnionMemberSameWS)
	h.WriteU32(uint32(len(bundle.Meta)))
	for _, meta := range bundle.Meta {
		h.WriteU8(uint8(meta.Kind))
		h.WriteU32(meta.Index)
		h.WriteU8(uint8(meta.WhiteSpace))
		h.WriteU8(uint8(meta.Flags))
		h.WriteU32(meta.Facets.Off)
		h.WriteU32(meta.Facets.Len)
	}
}

func digestFacets(h FingerprintWriter, facets []FacetInstr) {
	h.WriteU32(uint32(len(facets)))
	for _, f := range facets {
		h.WriteU8(uint8(f.Op))
		h.WriteU32(f.Arg0)
		h.WriteU32(f.Arg1)
	}
}

func digestPatterns(h FingerprintWriter, patterns []Pattern) {
	h.WriteU32(uint32(len(patterns)))
	for _, p := range patterns {
		h.WriteBytes(p.Source)
	}
}

func digestEnums(h FingerprintWriter, enums *EnumTable) {
	if enums == nil {
		return
	}
	digestU32Slice(h, enums.Off)
	digestU32Slice(h, enums.Len)
	h.WriteU32(uint32(len(enums.Keys)))
	for _, key := range enums.Keys {
		h.WriteU8(uint8(key.Kind))
		h.WriteU64(key.Hash)
		h.WriteBytes(key.Bytes)
	}
	digestU32Slice(h, enums.HashOff)
	digestU32Slice(h, enums.HashLen)
	digestU64Slice(h, enums.Hashes)
	digestU32Slice(h, enums.Slots)
}

func digestValues(h FingerprintWriter, values ValueBlob) {
	h.WriteBytes(values.Blob)
}
