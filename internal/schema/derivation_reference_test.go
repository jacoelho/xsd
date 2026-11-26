package schema

import (
	"fmt"
	"math/rand/v2"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

const referenceAnyType ComplexTypeID = 0

var referenceDerivationKinds = map[DerivationMask]derivationKind{
	0:                     derivationKindNone,
	DerivationExtension:   derivationKindExtension,
	DerivationRestriction: derivationKindRestriction,
}

// referenceTypeGraph is a type hierarchy kept in the shape of the
// specification: every type names its base, and a union names its members.
// Simple type 0 is the root of the simple hierarchy and complex type 0 is
// anyType, whose base is the zero TypeID.
type referenceTypeGraph struct {
	simple  []referenceSimpleType
	complex []referenceComplexType
	types   []TypeID
}

type referenceSimpleType struct {
	variety   valuepkg.Variety
	base      SimpleTypeID
	members   []SimpleTypeID
	listItem  SimpleTypeID
	hasFacets bool
	declares  bool
}

type referenceComplexType struct {
	base   TypeID
	method DerivationMask
	block  DerivationMask
}

func generatedReferenceTypeGraph(rng *rand.Rand) referenceTypeGraph {
	g := referenceTypeGraph{
		simple:  []referenceSimpleType{{variety: valuepkg.Atomic, base: NoSimpleType}},
		complex: []referenceComplexType{{}},
		types:   []TypeID{SimpleRef(0), ComplexRef(referenceAnyType)},
	}
	simpleCount := 1 + rng.Uint32N(8)
	for i := uint32(1); i < simpleCount; i++ {
		j := SimpleTypeID(rng.Uint32N(i))
		switch rng.IntN(4) {
		case 0:
			members := make([]SimpleTypeID, 1+rng.IntN(3))
			for k := range members {
				members[k] = SimpleTypeID(rng.Uint32N(i))
			}
			g.simple = append(g.simple, referenceSimpleType{variety: valuepkg.Union, base: 0, members: members, hasFacets: rng.IntN(4) == 0, declares: false})
		case 1:
			item := j
			if g.simple[item].variety == valuepkg.List {
				item = 0
			}
			g.simple = append(g.simple, referenceSimpleType{variety: valuepkg.List, base: 0, listItem: item})
		default:
			parent := g.simple[j]
			declares := parent.variety == valuepkg.Union && !parent.hasFacets && rng.IntN(2) == 0
			g.simple = append(g.simple, referenceSimpleType{
				variety:   parent.variety,
				base:      j,
				members:   parent.members,
				listItem:  parent.listItem,
				hasFacets: parent.variety == valuepkg.Union && (parent.hasFacets || declares),
				declares:  declares,
			})
		}
		g.types = append(g.types, SimpleRef(SimpleTypeID(i)))
	}
	blocks := []DerivationMask{0, DerivationExtension, DerivationRestriction, DerivationExtension | DerivationRestriction}
	for i, n := uint32(1), 1+rng.Uint32N(8); i < n; i++ {
		typ := referenceComplexType{block: blocks[rng.IntN(len(blocks))]}
		if rng.IntN(3) == 0 {
			typ.base = SimpleRef(SimpleTypeID(rng.Uint32N(simpleCount)))
			typ.method = DerivationExtension
		} else {
			typ.base = ComplexRef(ComplexTypeID(rng.Uint32N(i)))
			typ.method = []DerivationMask{DerivationExtension, DerivationRestriction}[rng.IntN(2)]
		}
		g.complex = append(g.complex, typ)
		g.types = append(g.types, ComplexRef(ComplexTypeID(i)))
	}
	return g
}

// realSimple maps a reference simple type to its ID in a builder: reference
// type 0 is xs:anySimpleType and the others follow the builtin types.
func realSimple(id SimpleTypeID) SimpleTypeID {
	if id == 0 {
		return valuepkg.AnySimpleTypeID
	}
	return valuepkg.BuiltinTypeCount + id - 1
}

func realType(t TypeID) TypeID {
	if id, ok := t.Simple(); ok {
		return SimpleRef(realSimple(id))
	}
	return t
}

func (referenceTypeGraph) spec(typ referenceSimpleType) valuepkg.TypeSpec {
	spec := valuepkg.TypeSpec{
		Variety:           typ.variety,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              realSimple(typ.base),
		ListItem:          valuepkg.NoType,
	}
	switch typ.variety {
	case valuepkg.Atomic:
		spec.Whitespace = valuepkg.WhitespacePreserve
	case valuepkg.List:
		spec.ListItem = realSimple(typ.listItem)
	case valuepkg.Union:
		for _, member := range typ.members {
			spec.Union = append(spec.Union, realSimple(member))
		}
	}
	if typ.declares || (typ.variety == valuepkg.Union && typ.hasFacets && typ.base == 0) {
		spec.Facets.Enumeration = []valuepkg.LiteralSpec{{Lexical: "x", Type: valuepkg.NoType}}
	}
	return spec
}

// build completes every generated simple type through a value.Builder, the
// owner of simple-type structure, and records builtin and user types at their
// builder IDs.
func (g referenceTypeGraph) build(t *testing.T) *schemaBuild {
	t.Helper()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	b := &schemaBuild{AnyType: referenceAnyType, valueBuilder: builder}
	b.SimpleTypes = make([]simpleType, valuepkg.BuiltinTypeCount)
	for i, typ := range g.simple {
		if i == 0 {
			continue
		}
		id, err := builder.Reserve()
		if err != nil {
			t.Fatal(err)
		}
		if id != realSimple(SimpleTypeID(i)) {
			t.Fatalf("reserved ID %d, want %d", id, realSimple(SimpleTypeID(i)))
		}
		if err := builder.Complete(id, g.spec(typ)); err != nil {
			t.Fatalf("graph %v: Complete(%d) = %v", g, id, err)
		}
		b.SimpleTypes = append(b.SimpleTypes, simpleType{})
	}
	for _, typ := range g.complex {
		base := typ.base
		b.ComplexTypes = append(b.ComplexTypes, complexTypeMetadata{Base: realType(base), Block: typ.block, Derivation: referenceDerivationKinds[typ.method]})
	}
	return b
}

// simpleDerives applies Type Derivation OK (Simple) clause 2, passing through
// the members of a union only when it has no facets (I-010 of
// docs/spec/INTERPRETATIONS.md).
func (g referenceTypeGraph) simpleDerives(derived, base SimpleTypeID) bool {
	for cur := derived; cur != NoSimpleType; cur = g.simple[cur].base {
		if cur == base {
			return true
		}
	}
	b := g.simple[base]
	if b.variety != valuepkg.Union || b.hasFacets {
		return false
	}
	for _, m := range b.members {
		if g.simpleDerives(derived, m) {
			return true
		}
	}
	return false
}

// referenceChain is the walk from a complex type through its bases: the
// complex types passed, starting with the derived type, the method of each
// one's step to its base, and the simple type the walk ends in, or
// NoSimpleType when it ends at anyType.
type referenceChain struct {
	types   []ComplexTypeID
	methods []DerivationMask
	core    SimpleTypeID
}

func (g referenceTypeGraph) complexChain(derived ComplexTypeID) referenceChain {
	chain := referenceChain{core: NoSimpleType}
	for cur := derived; ; {
		chain.types = append(chain.types, cur)
		typ := g.complex[cur]
		chain.methods = append(chain.methods, typ.method)
		if next, ok := typ.base.Complex(); ok {
			cur = next
			continue
		}
		if core, ok := typ.base.Simple(); ok {
			chain.core = core
		}
		return chain
	}
}

func (g referenceTypeGraph) derivation(derived, base TypeID) (DerivationMask, bool) {
	if derived == base {
		return 0, true
	}
	if d, ok := derived.Simple(); ok {
		if b, ok := base.Simple(); ok && g.simpleDerives(d, b) {
			return DerivationRestriction, true
		}
		if base == ComplexRef(referenceAnyType) {
			return DerivationRestriction, true
		}
		return 0, false
	}
	d, _ := derived.Complex()
	chain := g.complexChain(d)
	var steps DerivationMask
	for i, cur := range chain.types {
		if base == ComplexRef(cur) {
			return steps, true
		}
		steps |= chain.methods[i]
	}
	if chain.core == NoSimpleType {
		return 0, false
	}
	if base == ComplexRef(referenceAnyType) {
		return steps | DerivationRestriction, true
	}
	b, ok := base.Simple()
	if !ok || !g.simpleDerives(chain.core, b) {
		return 0, false
	}
	if chain.core == b {
		return steps, true
	}
	return steps | DerivationRestriction, true
}

// substitutionBlocks is the union Substitution Group OK (Transitive) clause
// 2.3 forms from the {prohibited substitutions} of base and of every complex
// type strictly between derived and base.
func (g referenceTypeGraph) substitutionBlocks(derived, base TypeID) DerivationMask {
	if derived == base {
		return 0
	}
	var blocks DerivationMask
	if b, ok := base.Complex(); ok {
		blocks = g.complex[b].block
	}
	d, ok := derived.Complex()
	if !ok {
		return blocks
	}
	for _, cur := range g.complexChain(d).types[1:] {
		if base == ComplexRef(cur) {
			break
		}
		blocks |= g.complex[cur].block
	}
	return blocks
}

func (g referenceTypeGraph) String() string {
	return fmt.Sprintf("simple %+v complex %+v", g.simple, g.complex)
}

func TestTypeDerivationAgreesWithReferenceWalkOnGeneratedGraphs(t *testing.T) {
	t.Parallel()

	const seed = 0x5eed
	rng := rand.New(rand.NewPCG(seed, 1)) //nolint:gosec // Reproducible generated graphs, not security-sensitive randomness.
	for range 2_000 {
		g := generatedReferenceTypeGraph(rng)
		rt := g.build(t)
		read, err := newTypeDerivationReadForTypes(referenceAnyType, rt.valueBuilder, rt.SimpleTypes, rt.ComplexTypes)
		if err != nil {
			t.Fatalf("seed %#x, graph %v: newTypeDerivationReadForTypes() error = %v", seed, g, err)
		}
		var scratch TypeDerivationScratch
		for _, refDerived := range g.types {
			for _, refBase := range g.types {
				derived, base := realType(refDerived), realType(refBase)
				wantMask, wantOK := g.derivation(refDerived, refBase)
				walkMask, walkOK, err := deriveTypeMask(rt, derived, base, unboundedTypeDerivationWork)
				if err != nil || walkMask != wantMask || walkOK != wantOK {
					t.Fatalf("seed %#x, graph %v: deriveTypeMask(%v, %v) = %02b, %v, %v; want %02b, %v", seed, g, derived, base, walkMask, walkOK, err, wantMask, wantOK)
				}
				indexMask, indexOK := read.derivation(derived, base, &scratch)
				if indexMask != wantMask || indexOK != wantOK {
					t.Fatalf("seed %#x, graph %v: index derivation(%v, %v) = %02b, %v; want %02b, %v", seed, g, derived, base, indexMask, indexOK, wantMask, wantOK)
				}
				if !wantOK {
					continue
				}
				blocks, err := substitutionTypeBlocks(rt, derived, base, unboundedTypeDerivationWork)
				if want := g.substitutionBlocks(refDerived, refBase); err != nil || blocks != want {
					t.Fatalf("seed %#x, graph %v: substitutionTypeBlocks(%v, %v) = %02b, %v; want %02b", seed, g, derived, base, blocks, err, want)
				}
			}
		}
	}
}

func TestMissingSimpleTypeDerivesFromNothingButAnyType(t *testing.T) {
	t.Parallel()

	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	sentinel := missingSimpleType(QName{})
	if completeErr := builder.Complete(id, sentinel.spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	simple := make([]simpleType, int(id)+1)
	simple[id] = sentinel.decl
	read, err := newTypeDerivationReadForTypes(referenceAnyType, builder, simple, []complexTypeMetadata{{}})
	if err != nil {
		t.Fatal(err)
	}
	var scratch TypeDerivationScratch
	missing := SimpleRef(id)
	for _, base := range []TypeID{SimpleRef(valuepkg.BuiltinType(valuepkg.PrimitiveString)), SimpleRef(valuepkg.AnySimpleTypeID)} {
		if mask, ok := read.derivation(missing, base, &scratch); ok {
			t.Errorf("missing type derives from %v with mask %02b", base, mask)
		}
	}
	if _, ok := read.derivation(missing, ComplexRef(referenceAnyType), &scratch); !ok {
		t.Error("missing type does not derive from anyType")
	}
}
