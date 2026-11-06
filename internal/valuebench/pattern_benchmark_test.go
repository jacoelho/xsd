package valuebench

import (
	"errors"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func BenchmarkSimplePatternVariableNoMatchGroupScratch(b *testing.B) {
	pattern, err := xsdregex.Compile(`[a-z]{0,}[a-z]{0,}x`)
	if err != nil {
		b.Fatal(err)
	}
	patterns := make([]*valuepkg.Pattern, 8)
	for i := range patterns {
		patterns[i] = pattern
	}
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	id, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(id, valuepkg.TypeSpec{
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
	if _, err := program.Validate(id, input, valuepkg.Resolver{}, 0, valuepkg.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, &scratch); !errors.Is(err, valuepkg.ErrFacet) {
		b.Fatalf("pattern validation = %v; want %v", err, valuepkg.ErrFacet)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := program.Validate(id, input, valuepkg.Resolver{}, 0, valuepkg.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, &scratch); !errors.Is(err, valuepkg.ErrFacet) {
			b.Fatalf("pattern validation = %v; want %v", err, valuepkg.ErrFacet)
		}
	}
}
