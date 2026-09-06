package valuebench

import (
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkProgramPrimitiveNoFacets(b *testing.B) {
	p, id := benchmarkProgramPrimitive(b)
	benchmarkProgramValidate(b, p, id, "value", 0)
}

func BenchmarkProgramDecimalBound(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveDecimal,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
		Facets: valuepkg.FacetSpec{
			MinInclusive: valuepkg.BoundFacet{LiteralSpec: valuepkg.LiteralSpec{Lexical: "1.5", Type: valuepkg.NoType}, Present: true},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkProgramValidate(b, p, id, "2.00", valuepkg.NeedCanonical)
}

func BenchmarkProgramList(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	item, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	id, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.List,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          item,
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkProgramValidate(b, p, id, "one two three", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
}

func BenchmarkProgramUnion(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	stringID, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	decimalID, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveDecimal,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	unionID, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Union,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
		Union:             []valuepkg.TypeID{stringID, decimalID},
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	benchmarkProgramValidate(b, p, unionID, "2", valuepkg.NeedCanonical)
}

func benchmarkProgramPrimitive(b *testing.B) (*valuepkg.Program, valuepkg.TypeID) {
	b.Helper()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespacePreserve,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
	p, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	return p, id
}

func benchmarkProgramValidate(b *testing.B, p *valuepkg.Program, id valuepkg.TypeID, lexical string, needs valuepkg.Needs) {
	b.Helper()
	b.ReportAllocs()
	for range b.N {
		if _, err := p.Validate(id, lexical, valuepkg.Resolver{}, needs, nil); err != nil {
			b.Fatal(err)
		}
	}
}
