package valuebench

import (
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func BenchmarkPublishedRawUnionLateMember(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	booleanID, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveBoolean,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
	})
	if err != nil {
		b.Fatal(err)
	}
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
	unionID, err := builder.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Union,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
		Union:             []valuepkg.TypeID{booleanID, stringID},
	})
	if err != nil {
		b.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		b.Fatal(err)
	}
	raw := []byte("value")
	b.ReportAllocs()
	for b.Loop() {
		if _, err := program.ValidateBytes(unionID, raw, valuepkg.Resolver{}, 0, nil); err != nil {
			b.Fatal(err)
		}
	}
}
