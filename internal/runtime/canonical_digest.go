package runtime

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

type digestBuilder struct {
	h   hash.Hash
	buf [8]byte
}

func newDigestBuilder() *digestBuilder {
	return &digestBuilder{h: sha256.New()}
}

func (b *digestBuilder) write(p []byte) {
	// hash.Hash.Write never returns an error for standard library hashes.
	_, _ = b.h.Write(p)
}

func (b *digestBuilder) u8(v uint8) {
	b.buf[0] = v
	b.write(b.buf[:1])
}

func (b *digestBuilder) u32(v uint32) {
	binary.LittleEndian.PutUint32(b.buf[:4], v)
	b.write(b.buf[:4])
}

func (b *digestBuilder) u64(v uint64) {
	binary.LittleEndian.PutUint64(b.buf[:8], v)
	b.write(b.buf[:8])
}

func (b *digestBuilder) bool(v bool) {
	if v {
		b.u8(1)
		return
	}
	b.u8(0)
}

func (b *digestBuilder) bytes(data []byte) {
	b.u32(uint32(len(data)))
	if len(data) == 0 {
		return
	}
	b.write(data)
}

func (b *digestBuilder) sum() [32]byte {
	var out [32]byte
	sum := b.h.Sum(nil)
	copy(out[:], sum)
	return out
}

// CanonicalDigest returns a deterministic digest of canonical schema artifacts.
func (s *Schema) CanonicalDigest() [32]byte {
	if s == nil {
		return [32]byte{}
	}
	h := newDigestBuilder()

	digestNamespaces(h, &s.Namespaces)
	digestSymbols(h, &s.Symbols)

	digestPredef(h, s.Predef, s.PredefNS, s.Builtin, s.RootPolicy)
	digestGlobalIndices(h, s.GlobalTypes, s.GlobalElements, s.GlobalAttributes)

	digestTypes(h, s.Types)
	digestAncestors(h, s.Ancestors)
	digestComplexTypes(h, s.ComplexTypes)
	digestElements(h, s.Elements)
	digestAttributes(h, s.Attributes)
	digestAttrIndex(h, s.AttrIndex)

	digestValidators(h, &s.Validators)
	digestFacets(h, s.Facets)
	digestPatterns(h, s.Patterns)
	digestEnums(h, &s.Enums)
	digestValues(h, s.Values)

	digestModels(h, s.Models)
	digestWildcards(h, s.Wildcards, s.WildcardNS)

	digestIdentity(h, s.ICs, s.ElemICs, s.ICSelectors, s.ICFields, s.Paths)

	return h.sum()
}

func digestNamespaces(h *digestBuilder, table *NamespaceTable) {
	if table == nil {
		return
	}
	h.bytes(table.Blob)
	digestU32Slice(h, table.Off)
	digestU32Slice(h, table.Len)
	digestU64Slice(h, table.Index.Hash)
	digestNamespaceIDs(h, table.Index.ID)
}

func digestSymbols(h *digestBuilder, table *SymbolsTable) {
	if table == nil {
		return
	}
	digestNamespaceIDs(h, table.NS)
	digestU32Slice(h, table.LocalOff)
	digestU32Slice(h, table.LocalLen)
	h.bytes(table.LocalBlob)
	digestU64Slice(h, table.Index.Hash)
	digestSymbolIDs(h, table.Index.ID)
}

func digestPredef(h *digestBuilder, pre PredefinedSymbols, preNS PredefinedNamespaces, builtin BuiltinIDs, policy RootPolicy) {
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

func digestGlobalIndices(h *digestBuilder, types []TypeID, elems []ElemID, attrs []AttrID) {
	digestTypeIDs(h, types)
	digestElemIDs(h, elems)
	digestAttrIDs(h, attrs)
}

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
		digestModelRef(h, ct.Model)
		h.bool(ct.Mixed)
	}
}

func digestElements(h *digestBuilder, elems []Element) {
	h.u32(uint32(len(elems)))
	for _, e := range elems {
		h.u32(uint32(e.Name))
		h.u32(uint32(e.Type))
		h.u32(uint32(e.SubstHead))
		digestValueRef(h, e.Default)
		digestValueRef(h, e.Fixed)
		h.u32(uint32(e.Flags))
		h.u8(uint8(e.Block))
		h.u8(uint8(e.Final))
		h.u32(e.ICOff)
		h.u32(e.ICLen)
	}
}

func digestAttributes(h *digestBuilder, attrs []Attribute) {
	h.u32(uint32(len(attrs)))
	for _, a := range attrs {
		h.u32(uint32(a.Name))
		h.u32(uint32(a.Validator))
		digestValueRef(h, a.Default)
		digestValueRef(h, a.Fixed)
	}
}

func digestAttrIndex(h *digestBuilder, idx ComplexAttrIndex) {
	h.u32(uint32(len(idx.Uses)))
	for _, use := range idx.Uses {
		h.u32(uint32(use.Name))
		h.u32(uint32(use.Validator))
		h.u8(uint8(use.Use))
		digestValueRef(h, use.Default)
		digestValueRef(h, use.Fixed)
	}
	h.u32(uint32(len(idx.HashTables)))
	for _, table := range idx.HashTables {
		digestU64Slice(h, table.Hash)
		digestU32Slice(h, table.Slot)
	}
}

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

func digestModels(h *digestBuilder, models ModelsBundle) {
	h.u32(uint32(len(models.DFA)))
	for _, m := range models.DFA {
		h.u32(m.Start)
		h.u32(uint32(len(m.States)))
		for _, s := range m.States {
			h.bool(s.Accept)
			h.u32(s.TransOff)
			h.u32(s.TransLen)
			h.u32(s.WildOff)
			h.u32(s.WildLen)
		}
		h.u32(uint32(len(m.Transitions)))
		for _, tr := range m.Transitions {
			h.u32(uint32(tr.Sym))
			h.u32(tr.Next)
			h.u32(uint32(tr.Elem))
		}
		h.u32(uint32(len(m.Wildcards)))
		for _, w := range m.Wildcards {
			h.u32(uint32(w.Rule))
			h.u32(w.Next)
		}
	}
	h.u32(uint32(len(models.NFA)))
	for _, m := range models.NFA {
		digestU64Slice(h, m.Bitsets.Words)
		digestBitsetRef(h, m.Start)
		digestBitsetRef(h, m.Accept)
		h.bool(m.Nullable)
		h.u32(m.FollowOff)
		h.u32(m.FollowLen)
		h.u32(uint32(len(m.Matchers)))
		for _, match := range m.Matchers {
			h.u8(uint8(match.Kind))
			h.u32(uint32(match.Sym))
			h.u32(uint32(match.Elem))
			h.u32(uint32(match.Rule))
		}
		h.u32(uint32(len(m.Follow)))
		for _, ref := range m.Follow {
			digestBitsetRef(h, ref)
		}
	}
	h.u32(uint32(len(models.All)))
	for _, m := range models.All {
		h.u32(m.MinOccurs)
		h.bool(m.Mixed)
		h.u32(uint32(len(m.Members)))
		for _, member := range m.Members {
			h.u32(uint32(member.Elem))
			h.bool(member.Optional)
			h.bool(member.AllowsSubst)
			h.u32(member.SubstOff)
			h.u32(member.SubstLen)
		}
	}
	digestElemIDs(h, models.AllSubst)
}

func digestWildcards(h *digestBuilder, wildcards []WildcardRule, nsList []NamespaceID) {
	h.u32(uint32(len(wildcards)))
	for _, w := range wildcards {
		h.u8(uint8(w.NS.Kind))
		h.bool(w.NS.HasTarget)
		h.bool(w.NS.HasLocal)
		h.u32(w.NS.Off)
		h.u32(w.NS.Len)
		h.u8(uint8(w.PC))
		h.u32(uint32(w.TargetNS))
	}
	digestNamespaceIDs(h, nsList)
}

func digestIdentity(h *digestBuilder, ics []IdentityConstraint, elemICs []ICID, selectors, fields []PathID, paths []PathProgram) {
	h.u32(uint32(len(ics)))
	for _, ic := range ics {
		h.u32(uint32(ic.Name))
		h.u8(uint8(ic.Category))
		h.u32(ic.SelectorOff)
		h.u32(ic.SelectorLen)
		h.u32(ic.FieldOff)
		h.u32(ic.FieldLen)
		h.u32(uint32(ic.Referenced))
	}
	digestICIDs(h, elemICs)
	digestPathIDs(h, selectors)
	digestPathIDs(h, fields)
	h.u32(uint32(len(paths)))
	for _, p := range paths {
		h.u32(uint32(len(p.Ops)))
		for _, op := range p.Ops {
			h.u8(uint8(op.Op))
			h.u32(uint32(op.Sym))
			h.u32(uint32(op.NS))
		}
	}
}

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
