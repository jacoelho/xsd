package value

import (
	"testing"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestUnionUsesSelectedMemberWhitespace(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	union, err := b.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Union:             []TypeID{builtinInt, builtinString},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, lexical, canonical string
		selected                 TypeID
	}{
		{name: "integer", lexical: " 7 ", canonical: "7", selected: builtinInt},
		{name: "string fallback", lexical: "  x  ", canonical: "  x  ", selected: builtinString},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Validate(union, tc.lexical, Resolver{}, NeedCanonical, 16<<20, nil)
			if err != nil {
				t.Fatalf("Validate(%q): %v", tc.lexical, err)
			}
			if got.SelectedType() != tc.selected || got.CanonicalText() != tc.canonical {
				t.Fatalf("Validate(%q) = selected %d, canonical %q; want %d, %q", tc.lexical, got.SelectedType(), got.CanonicalText(), tc.selected, tc.canonical)
			}

			got, err = p.ValidateBytes(union, []byte(tc.lexical), Resolver{}, NeedCanonical, 16<<20, nil)
			if err != nil {
				t.Fatalf("ValidateBytes(%q): %v", tc.lexical, err)
			}
			if got.SelectedType() != tc.selected || got.CanonicalText() != tc.canonical {
				t.Fatalf("ValidateBytes(%q) = selected %d, canonical %q; want %d, %q", tc.lexical, got.SelectedType(), got.CanonicalText(), tc.selected, tc.canonical)
			}
		})
	}
}

func TestUnionPatternUsesSelectedMemberWhitespaceWithoutProjection(t *testing.T) {
	pattern, err := xsdregex.Compile(`\S+ \S+`, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(BuilderOptions{})
	union, err := b.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Union:             []TypeID{builtinToken, builtinString},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	value, err := p.Validate(union, "a   b", Resolver{}, 0, 16<<20, nil)
	if err != nil {
		t.Fatalf("Validate without projection: %v", err)
	}
	if value.SelectedType() != builtinToken {
		t.Fatalf("SelectedType() = %d, want token %d", value.SelectedType(), builtinToken)
	}
	if value.CanonicalText() != "" {
		t.Fatalf("CanonicalText() = %q, want empty without projection", value.CanonicalText())
	}
}

func TestUnionEnumerationUsesSelectedMemberValue(t *testing.T) {
	b := NewBuilder(BuilderOptions{})
	union, err := b.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Union:             []TypeID{builtinString, builtinInt},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{
			Lexical: "  x  ",
			Type:    NoType,
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, validate := range []struct {
		name string
		run  func(string) (Value, error)
	}{
		{name: "string", run: func(lexical string) (Value, error) {
			return p.Validate(union, lexical, Resolver{}, NeedCanonical, 16<<20, nil)
		}},
		{name: "bytes", run: func(lexical string) (Value, error) {
			return p.ValidateBytes(union, []byte(lexical), Resolver{}, NeedCanonical, 16<<20, nil)
		}},
	} {
		t.Run(validate.name, func(t *testing.T) {
			got, err := validate.run("  x  ")
			if err != nil {
				t.Fatalf("spaced literal: %v", err)
			}
			if got.SelectedType() != builtinString || got.CanonicalText() != "  x  " {
				t.Fatalf("spaced literal = selected %d, canonical %q; want string %d and %q", got.SelectedType(), got.CanonicalText(), builtinString, "  x  ")
			}
			if _, err := validate.run("x"); err == nil {
				t.Fatal("unspaced literal matched the spaced string enumeration")
			}
		})
	}
}

func TestUnionPatternUsesSelectedMemberWhitespace(t *testing.T) {
	tests := []struct {
		name      string
		members   []TypeID
		pattern   string
		lexical   string
		wantError bool
		list      bool
		selected  TypeID
		canonical string
	}{
		{
			name:      "collapse before pattern",
			members:   []TypeID{builtinToken, builtinString},
			pattern:   `\S+ \S+`,
			lexical:   "a   b",
			selected:  builtinToken,
			canonical: "a b",
		},
		{
			name:      "pattern failure does not try later member",
			members:   []TypeID{builtinToken, builtinString},
			pattern:   `\S+\s{2}\S+`,
			lexical:   "a  b",
			wantError: true,
		},
		{
			name:      "replace before pattern",
			members:   []TypeID{builtinNormalizedString, builtinString},
			pattern:   `[a-z ]+`,
			lexical:   "hello\nworld",
			selected:  builtinNormalizedString,
			canonical: "hello world",
		},
		{
			name:      "preserve before pattern",
			members:   []TypeID{builtinString},
			pattern:   `\S+`,
			lexical:   "  a  ",
			wantError: true,
		},
		{
			name:      "list lexical text rather than last item",
			list:      true,
			pattern:   `\S+ \S+`,
			lexical:   " a  b ",
			canonical: "a b",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pattern, err := xsdregex.Compile(test.pattern, xsdregex.CompileOptions{})
			if err != nil {
				t.Fatal(err)
			}
			b := NewBuilder(BuilderOptions{})
			if test.list {
				list, listErr := b.Add(TypeSpec{
					Variety: List, Primitive: PrimitiveString, Base: NoType,
					ListItem: builtinString, Whitespace: WhitespaceCollapse, WhitespacePresent: true,
				})
				if listErr != nil {
					t.Fatal(listErr)
				}
				test.members = []TypeID{list}
				test.selected = list
			}
			union, err := b.Add(TypeSpec{
				Variety:           Union,
				Primitive:         PrimitiveString,
				Union:             test.members,
				Whitespace:        WhitespaceCollapse,
				WhitespacePresent: true,
				Base:              NoType,
				ListItem:          NoType,
				Facets:            FacetSpec{Patterns: [][]*Pattern{{pattern}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			p, err := b.Seal()
			if err != nil {
				t.Fatal(err)
			}

			for _, validate := range []struct {
				name string
				run  func(string) (Value, error)
			}{
				{name: "string", run: func(lexical string) (Value, error) {
					return p.Validate(union, lexical, Resolver{}, NeedCanonical, 16<<20, nil)
				}},
				{name: "bytes", run: func(lexical string) (Value, error) {
					return p.ValidateBytes(union, []byte(lexical), Resolver{}, NeedCanonical, 16<<20, nil)
				}},
			} {
				t.Run(validate.name, func(t *testing.T) {
					got, err := validate.run(test.lexical)
					if test.wantError {
						if err == nil {
							t.Fatalf("Validate(%q) succeeded; want pattern failure", test.lexical)
						}
						return
					}
					if err != nil {
						t.Fatalf("Validate(%q): %v", test.lexical, err)
					}
					if got.SelectedType() != test.selected || got.CanonicalText() != test.canonical {
						t.Fatalf("Validate(%q) = selected %d, canonical %q; want %d, %q", test.lexical, got.SelectedType(), got.CanonicalText(), test.selected, test.canonical)
					}
				})
			}
		})
	}
}

func TestNestedUnionPatternUsesTerminalMemberWhitespace(t *testing.T) {
	pattern, err := xsdregex.Compile(`\S+\s{2}\S+`, xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(BuilderOptions{})
	inner, err := b.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Union:             []TypeID{builtinToken, builtinString},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	outer, err := b.Add(TypeSpec{
		Variety:           Union,
		Primitive:         PrimitiveString,
		Union:             []TypeID{inner, builtinString},
		Whitespace:        WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              NoType,
		ListItem:          NoType,
		Facets:            FacetSpec{Patterns: [][]*Pattern{{pattern}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, validate := range []struct {
		name string
		run  func(string) error
	}{
		{name: "string", run: func(lexical string) error {
			_, err := p.Validate(outer, lexical, Resolver{}, NeedCanonical, 16<<20, nil)
			return err
		}},
		{name: "bytes", run: func(lexical string) error {
			_, err := p.ValidateBytes(outer, []byte(lexical), Resolver{}, NeedCanonical, 16<<20, nil)
			return err
		}},
	} {
		t.Run(validate.name, func(t *testing.T) {
			if err := validate.run("a  b"); err == nil {
				t.Fatal("nested union accepted outer pattern using raw lexical form")
			}
		})
	}
}
