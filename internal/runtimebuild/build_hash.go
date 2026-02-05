package runtimebuild

import (
	"encoding/binary"
	"hash"
	"hash/fnv"

	"github.com/jacoelho/xsd/internal/runtime"
)

type hashBuilder struct {
	h   hash.Hash64
	buf [8]byte
}

func newHashBuilder() *hashBuilder {
	return &hashBuilder{h: fnv.New64a()}
}

func (b *hashBuilder) write(p []byte) {
	// hash.Hash.Write never returns an error for standard library hashes.
	_, _ = b.h.Write(p)
}

func (b *hashBuilder) u8(v uint8) {
	b.buf[0] = v
	b.write(b.buf[:1])
}

func (b *hashBuilder) u32(v uint32) {
	binary.LittleEndian.PutUint32(b.buf[:4], v)
	b.write(b.buf[:4])
}

func (b *hashBuilder) u64(v uint64) {
	binary.LittleEndian.PutUint64(b.buf[:8], v)
	b.write(b.buf[:8])
}

func (b *hashBuilder) bool(v bool) {
	if v {
		b.u8(1)
		return
	}
	b.u8(0)
}

func (b *hashBuilder) bytes(data []byte) {
	b.u32(uint32(len(data)))
	if len(data) == 0 {
		return
	}
	b.write(data)
}

func (b *hashBuilder) sum64() uint64 {
	sum := b.h.Sum64()
	if sum == 0 {
		return 1
	}
	return sum
}

func computeBuildHash(rt *runtime.Schema) uint64 {
	if rt == nil {
		return 0
	}
	h := newHashBuilder()

	hashNamespaces(h, &rt.Namespaces)
	hashSymbols(h, &rt.Symbols)

	hashPredef(h, rt.Predef, rt.PredefNS, rt.Builtin, rt.RootPolicy)
	hashGlobalIndices(h, rt.GlobalTypes, rt.GlobalElements, rt.GlobalAttributes)

	hashTypes(h, rt.Types)
	hashAncestors(h, rt.Ancestors)
	hashComplexTypes(h, rt.ComplexTypes)
	hashElements(h, rt.Elements)
	hashAttributes(h, rt.Attributes)
	hashAttrIndex(h, rt.AttrIndex)

	hashValidators(h, &rt.Validators)
	hashFacets(h, rt.Facets)
	hashPatterns(h, rt.Patterns)
	hashEnums(h, &rt.Enums)
	hashValues(h, rt.Values)
	hashSymbolIDs(h, rt.Notations)

	hashModels(h, rt.Models)
	hashWildcards(h, rt.Wildcards, rt.WildcardNS)

	hashIdentity(h, rt.ICs, rt.ElemICs, rt.ICSelectors, rt.ICFields, rt.Paths)

	return h.sum64()
}

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
	for _, e := range elems {
		h.u32(uint32(e.Name))
		h.u32(uint32(e.Type))
		h.u32(uint32(e.SubstHead))
		hashValueRef(h, e.Default)
		hashValueRef(h, e.Fixed)
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
	for _, a := range attrs {
		h.u32(uint32(a.Name))
		h.u32(uint32(a.Validator))
		hashValueRef(h, a.Default)
		hashValueRef(h, a.Fixed)
		h.u32(uint32(a.DefaultMember))
		h.u32(uint32(a.FixedMember))
	}
}

func hashAttrIndex(h *hashBuilder, idx runtime.ComplexAttrIndex) {
	h.u32(uint32(len(idx.Uses)))
	for _, use := range idx.Uses {
		h.u32(uint32(use.Name))
		h.u32(uint32(use.Validator))
		h.u8(uint8(use.Use))
		hashValueRef(h, use.Default)
		hashValueRef(h, use.Fixed)
		h.u32(uint32(use.DefaultMember))
		h.u32(uint32(use.FixedMember))
	}
	h.u32(uint32(len(idx.HashTables)))
	for _, table := range idx.HashTables {
		hashU64Slice(h, table.Hash)
		hashU32Slice(h, table.Slot)
	}
}

func hashValidators(h *hashBuilder, bundle *runtime.ValidatorsBundle) {
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
	hashValidatorIDs(h, bundle.UnionMembers)
	hashTypeIDs(h, bundle.UnionMemberTypes)
	hashU8Slice(h, bundle.UnionMemberSameWS)
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

func hashFacets(h *hashBuilder, facets []runtime.FacetInstr) {
	h.u32(uint32(len(facets)))
	for _, f := range facets {
		h.u8(uint8(f.Op))
		h.u32(f.Arg0)
		h.u32(f.Arg1)
	}
}

func hashPatterns(h *hashBuilder, patterns []runtime.Pattern) {
	h.u32(uint32(len(patterns)))
	for _, p := range patterns {
		if len(p.Source) > 0 {
			h.bytes(p.Source)
			continue
		}
		if p.Re != nil {
			h.bytes([]byte(p.Re.String()))
			continue
		}
		h.bytes(nil)
	}
}

func hashEnums(h *hashBuilder, enums *runtime.EnumTable) {
	if enums == nil {
		return
	}
	hashU32Slice(h, enums.Off)
	hashU32Slice(h, enums.Len)
	h.u32(uint32(len(enums.Keys)))
	for _, key := range enums.Keys {
		h.u8(uint8(key.Kind))
		h.u64(key.Hash)
		h.bytes(key.Bytes)
	}
	hashU32Slice(h, enums.HashOff)
	hashU32Slice(h, enums.HashLen)
	hashU64Slice(h, enums.Hashes)
	hashU32Slice(h, enums.Slots)
}

func hashValues(h *hashBuilder, values runtime.ValueBlob) {
	h.bytes(values.Blob)
}

func hashModels(h *hashBuilder, models runtime.ModelsBundle) {
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
		hashU64Slice(h, m.Bitsets.Words)
		hashBitsetRef(h, m.Start)
		hashBitsetRef(h, m.Accept)
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
			hashBitsetRef(h, ref)
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
	hashElemIDs(h, models.AllSubst)
}

func hashWildcards(h *hashBuilder, wildcards []runtime.WildcardRule, nsList []runtime.NamespaceID) {
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
	hashNamespaceIDs(h, nsList)
}

func hashIdentity(h *hashBuilder, ics []runtime.IdentityConstraint, elemICs []runtime.ICID, selectors, fields []runtime.PathID, paths []runtime.PathProgram) {
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
	hashICIDs(h, elemICs)
	hashPathIDs(h, selectors)
	hashPathIDs(h, fields)
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
