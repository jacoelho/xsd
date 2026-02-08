package runtimecompile

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
