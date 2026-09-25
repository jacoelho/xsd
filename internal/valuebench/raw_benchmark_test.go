package valuebench

import (
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkPublishedRawStringEnumeration(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Add(valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
		Whitespace: valuepkg.WhitespacePreserve, WhitespacePresent: true,
		Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Facets: valuepkg.FacetSpec{Enumeration: []valuepkg.LiteralSpec{{Lexical: "ready"}, {Lexical: "waiting"}}},
	})
	if err != nil {
		b.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	raw := []byte("ready")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublishedRawNMTokensList(b *testing.B) {
	program, ok := mustBuiltinProgram(b, "NMTOKENS")
	if !ok {
		b.Fatal("missing NMTOKENS builtin")
	}
	id, _ := valuepkg.BuiltinTypeID("NMTOKENS")
	raw := []byte("one two three")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublishedRawIntegerBounds(b *testing.B) {
	program, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		b.Fatal(err)
	}
	id, _ := valuepkg.BuiltinTypeID("unsignedLong")
	raw := []byte("18446744073709551615")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func mustBuiltinProgram(tb testing.TB, name string) (*valuepkg.Program, bool) {
	tb.Helper()
	program, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		tb.Fatal(err)
	}
	_, ok := valuepkg.BuiltinTypeID(name)
	return program, ok
}
