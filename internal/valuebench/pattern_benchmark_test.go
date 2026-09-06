package valuebench

import (
	"errors"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func BenchmarkSimplePatternVariableNoMatchGroupScratch(b *testing.B) {
	pattern, err := xsdregex.Compile(`[a-z]{0,}[a-z]{0,}x`, xsdregex.CompileOptions{})
	if err != nil {
		b.Fatal(err)
	}
	patterns := make([]*valuepkg.Pattern, 8)
	for i := range patterns {
		patterns[i] = pattern
	}
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespacePreserve,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
		Facets:            valuepkg.FacetSpec{Patterns: [][]*valuepkg.Pattern{patterns}},
	})
	if err != nil {
		b.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	input := strings.Repeat("a", 4096)
	var scratch valuepkg.Scratch
	if _, err := program.Validate(id, input, valuepkg.Resolver{}, 0, &scratch); !errors.Is(err, valuepkg.ErrFacet) {
		b.Fatalf("pattern validation = %v; want %v", err, valuepkg.ErrFacet)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := program.Validate(id, input, valuepkg.Resolver{}, 0, &scratch); !errors.Is(err, valuepkg.ErrFacet) {
			b.Fatalf("pattern validation = %v; want %v", err, valuepkg.ErrFacet)
		}
	}
}
