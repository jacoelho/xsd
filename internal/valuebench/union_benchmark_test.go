package valuebench

import (
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func BenchmarkPublishedRawUnionLateMember(b *testing.B) {
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	booleanID, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(booleanID, valuepkg.TypeSpec{
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
	stringID, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(stringID, valuepkg.TypeSpec{
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
	unionID, err := builder.Reserve()
	if err != nil {
		b.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(unionID, valuepkg.TypeSpec{
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
		if _, err := program.ValidateBytes(unionID, raw, valuepkg.Resolver{}, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPublishedUnionRejectedMember(b *testing.B) {
	pattern, err := xsdregex.Compile(`[0-9]+`, xsdregex.CompileOptions{})
	if err != nil {
		b.Fatal(err)
	}
	integerID, ok := valuepkg.BuiltinTypeID("integer")
	if !ok {
		b.Fatal("missing integer builtin")
	}
	for _, fixture := range []struct {
		name, input string
		first       valuepkg.TypeSpec
	}{
		{name: "Enumeration", input: "waiting", first: valuepkg.TypeSpec{
			Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
			Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
			Base: valuepkg.NoType, ListItem: valuepkg.NoType,
			Facets: valuepkg.FacetSpec{Enumeration: []valuepkg.LiteralSpec{{Lexical: "ready", Type: valuepkg.NoType}}},
		}},
		{name: "Pattern", input: "other", first: valuepkg.TypeSpec{
			Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
			Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
			Base: valuepkg.NoType, ListItem: valuepkg.NoType,
			Facets: valuepkg.FacetSpec{Patterns: [][]*valuepkg.Pattern{{pattern}}},
		}},
		{name: "IntegerBound", input: "9", first: valuepkg.TypeSpec{
			Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveDecimal, Builtin: valuepkg.BuiltinInteger,
			Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
			Base: valuepkg.NoType, ListItem: valuepkg.NoType,
			Facets: valuepkg.FacetSpec{MinInclusive: valuepkg.BoundFacet{
				LiteralSpec: valuepkg.LiteralSpec{Lexical: "10", Type: valuepkg.NoType}, Present: true,
			}},
		}},
		{name: "ListItem", input: "1 other", first: valuepkg.TypeSpec{
			Variety: valuepkg.List, Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
			Base: valuepkg.NoType, ListItem: integerID,
		}},
		{name: "ListLength", input: "1 2 3", first: valuepkg.TypeSpec{
			Variety: valuepkg.List, Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
			Base: valuepkg.NoType, ListItem: integerID,
			Facets: valuepkg.FacetSpec{Length: valuepkg.CardinalityFacet{Value: 2, Present: true}},
		}},
	} {
		b.Run(fixture.name, func(b *testing.B) {
			builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
			firstID, reserveErr := builder.Reserve()
			if reserveErr != nil {
				b.Fatal(reserveErr)
			}
			if completeErr := builder.Complete(firstID, fixture.first); completeErr != nil {
				b.Fatal(completeErr)
			}
			stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
			unionID, reserveErr := builder.Reserve()
			if reserveErr != nil {
				b.Fatal(reserveErr)
			}
			if completeErr := builder.Complete(unionID, valuepkg.TypeSpec{
				Variety: valuepkg.Union, Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
				Base: valuepkg.NoType, ListItem: valuepkg.NoType,
				Union: []valuepkg.TypeID{firstID, stringID},
			}); completeErr != nil {
				b.Fatal(completeErr)
			}
			program, sealErr := builder.Seal()
			if sealErr != nil {
				b.Fatal(sealErr)
			}
			raw := []byte(fixture.input)
			for _, api := range []string{"string", "bytes", "admitted"} {
				b.Run(api, func(b *testing.B) {
					var scratch valuepkg.Scratch
					validate := func(needs valuepkg.Needs) (valuepkg.Value, error) {
						switch api {
						case "bytes":
							return program.ValidateBytes(unionID, raw, valuepkg.Resolver{}, needs, 16<<20, &scratch)
						case "admitted":
							return program.ValidateAdmitted(unionID, fixture.input, valuepkg.Resolver{}, needs, 16<<20, &scratch)
						default:
							return program.Validate(unionID, fixture.input, valuepkg.Resolver{}, needs, 16<<20, &scratch)
						}
					}
					projected, validationErr := validate(valuepkg.NeedCanonical | valuepkg.NeedIdentity)
					if validationErr != nil || projected.Type() != unionID || projected.SelectedType() != stringID || projected.CanonicalText() != fixture.input || !projected.HasIdentity() {
						b.Fatalf("projected validation: type=%d selected=%d canonical=%q identity=%t error=%v", projected.Type(), projected.SelectedType(), projected.CanonicalText(), projected.HasIdentity(), validationErr)
					}
					unprojected, validationErr := validate(0)
					if validationErr != nil || unprojected.Type() != unionID || unprojected.SelectedType() != stringID || unprojected.CanonicalText() != "" || unprojected.HasIdentity() {
						b.Fatalf("unprojected validation: type=%d selected=%d canonical=%q identity=%t error=%v", unprojected.Type(), unprojected.SelectedType(), unprojected.CanonicalText(), unprojected.HasIdentity(), validationErr)
					}
					b.ReportAllocs()
					for b.Loop() {
						if _, validationErr := validate(0); validationErr != nil {
							b.Fatal(validationErr)
						}
					}
				})
			}
		})
	}
}
