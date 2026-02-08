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
	digestSymbolIDs(h, s.Notations)

	digestModels(h, s.Models)
	digestWildcards(h, s.Wildcards, s.WildcardNS)

	digestIdentity(h, s.ICs, s.ElemICs, s.ICSelectors, s.ICFields, s.Paths)

	return h.sum()
}
