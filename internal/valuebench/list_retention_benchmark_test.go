package valuebench

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func BenchmarkProgramListRetention(b *testing.B) {
	for _, size := range []int{64 << 10, 1 << 20, 4 << 20} {
		b.Run(fmt.Sprintf("%dKiB", size>>10), func(b *testing.B) {
			input := strings.Repeat("a ", size/2)
			b.Run("plain", func(b *testing.B) {
				program, id := benchmarkListProgram(b, value.FacetSpec{})
				benchmarkListRetention(b, program, id, input, nil)
			})
			b.Run("direct-singleton-enum", func(b *testing.B) {
				program, id := benchmarkListProgram(b, value.FacetSpec{
					Enumeration: []value.LiteralSpec{{Lexical: "a", Type: value.NoType}},
				})
				benchmarkListRetention(b, program, id, input, value.ErrFacet)
			})
			b.Run("nested-union-enum", func(b *testing.B) {
				program, id := benchmarkNestedListEnumerationProgram(b)
				benchmarkListRetention(b, program, id, input, value.ErrFacet)
			})
		})
	}
}

func benchmarkListProgram(b *testing.B, facets value.FacetSpec) (*value.Program, value.TypeID) {
	b.Helper()
	builder := value.NewBuilder(value.BuilderOptions{})
	spec := value.TypeSpec{
		Variety:           value.List,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.BuiltinType(value.PrimitiveString),
		Facets:            facets,
	}
	id, err := builder.Add(spec)
	if err != nil {
		b.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	return program, id
}

func benchmarkNestedListEnumerationProgram(b *testing.B) (*value.Program, value.TypeID) {
	b.Helper()
	builder := value.NewBuilder(value.BuilderOptions{})
	listID, err := builder.Add(value.TypeSpec{
		Variety:           value.List,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.BuiltinType(value.PrimitiveString),
	})
	if err != nil {
		b.Fatal(err)
	}
	innerID, err := builder.Add(value.TypeSpec{
		Variety:           value.Union,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Union:             []value.TypeID{listID, value.BuiltinType(value.PrimitiveString)},
	})
	if err != nil {
		b.Fatal(err)
	}
	outerID, err := builder.Add(value.TypeSpec{
		Variety:           value.Union,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Union:             []value.TypeID{innerID},
		Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{{
			Lexical: "a",
			Type:    value.NoType,
		}}},
	})
	if err != nil {
		b.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	return program, outerID
}

func benchmarkListRetention(b *testing.B, program *value.Program, id value.TypeID, input string, wantError error) {
	b.Helper()
	if _, err := program.Validate(id, input, value.Resolver{}, 0, 1<<40, nil); !errors.Is(err, wantError) {
		b.Fatalf("Validate() error = %v, want %v", err, wantError)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	failures := 0
	for b.Loop() {
		if _, err := program.Validate(id, input, value.Resolver{}, 0, 1<<40, nil); err != nil {
			failures++
		}
	}
	b.ReportMetric(float64(failures)/float64(b.N), "errors/op")
}
