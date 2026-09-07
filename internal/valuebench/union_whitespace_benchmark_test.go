package valuebench

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func BenchmarkProgramUnionWhitespace(b *testing.B) {
	for _, test := range []struct {
		name     string
		members  []value.TypeID
		lexical  string
		pattern  string
		collapse bool
		replace  bool
	}{
		{name: "decimal", members: []value.TypeID{value.BuiltinType(value.PrimitiveDecimal), value.BuiltinType(value.PrimitiveString)}, lexical: " 7 "},
		{name: "string", members: []value.TypeID{value.BuiltinType(value.PrimitiveDecimal), value.BuiltinType(value.PrimitiveString)}, lexical: "  x  "},
		{name: "collapse-pattern", members: []value.TypeID{value.BuiltinType(value.PrimitiveString), value.BuiltinType(value.PrimitiveString)}, lexical: "a   b", pattern: `\S+ \S+`, collapse: true},
		{name: "pattern-rejection", members: []value.TypeID{value.BuiltinType(value.PrimitiveString), value.BuiltinType(value.PrimitiveString)}, lexical: "a  b", pattern: `\S+\s{2}\S+`, collapse: true},
		{name: "replace-pattern", members: []value.TypeID{value.BuiltinType(value.PrimitiveString), value.BuiltinType(value.PrimitiveString)}, lexical: "hello\nworld", pattern: `[a-z ]+`, replace: true},
	} {
		b.Run(test.name, func(b *testing.B) {
			builder := value.NewBuilder(value.BuilderOptions{})
			members := test.members
			if test.collapse || test.replace {
				whitespace := value.WhitespaceCollapse
				if test.replace {
					whitespace = value.WhitespaceReplace
				}
				collapsed, err := builder.Add(value.TypeSpec{
					Variety: value.Atomic, Primitive: value.PrimitiveString,
					Base: value.BuiltinType(value.PrimitiveString), ListItem: value.NoType,
					Whitespace: whitespace, WhitespacePresent: true,
				})
				if err != nil {
					b.Fatal(err)
				}
				members = []value.TypeID{collapsed, value.BuiltinType(value.PrimitiveString)}
			}
			var facets value.FacetSpec
			if test.pattern != "" {
				pattern, err := xsdregex.Compile(test.pattern, xsdregex.CompileOptions{})
				if err != nil {
					b.Fatal(err)
				}
				facets.Patterns = [][]*value.Pattern{{pattern}}
			}
			id, err := builder.Add(value.TypeSpec{
				Variety: value.Union, Primitive: value.PrimitiveString,
				Union: members,
				Base:  value.NoType, ListItem: value.NoType,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
				Facets: facets,
			})
			if err != nil {
				b.Fatal(err)
			}
			program, err := builder.Seal()
			if err != nil {
				b.Fatal(err)
			}
			failures := 0
			b.ReportAllocs()
			for b.Loop() {
				if _, err := program.Validate(id, test.lexical, value.Resolver{}, value.NeedCanonical, 16<<20, nil); err != nil {
					failures++
				}
			}
			b.ReportMetric(float64(failures)/float64(b.N), "errors/op")
		})
	}
}
