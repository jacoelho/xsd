package valuebench

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

// enumerationKind is one restriction shape carrying an enumeration of the
// "short" audit literals. Whether validation takes the raw or the typed path
// follows from the shape (see rawStringEnumerationPlan); it is not observable
// here, so each kind records its expected path only as a comment.
type enumerationKind struct {
	name string
	spec func(literals []valuepkg.LiteralSpec) valuepkg.TypeSpec
	// collapse reports whether a value padded with spaces still matches.
	collapse bool
	// covered lists sizes BenchmarkAuditStringEnumerationValidation measures.
	covered []int
}

func enumerationKinds(tb testing.TB) []enumerationKind {
	tb.Helper()
	builtin := func(local string) valuepkg.TypeID {
		id, ok := valuepkg.BuiltinTypeID(local)
		if !ok {
			tb.Fatalf("BuiltinTypeID(%q) missing", local)
		}
		return id
	}
	token, anyURI := builtin("token"), builtin("anyURI")
	return []enumerationKind{
		{
			// Raw path: preserved string with only the enumeration facet.
			name: "string_preserve",
			spec: func(literals []valuepkg.LiteralSpec) valuepkg.TypeSpec {
				return valuepkg.TypeSpec{
					Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
					Whitespace: valuepkg.WhitespacePreserve, WhitespacePresent: true,
					Base: valuepkg.NoType, ListItem: valuepkg.NoType,
					Facets: valuepkg.FacetSpec{Enumeration: literals},
				}
			},
			covered: []int{8, 32, 1024},
		},
		{
			// Typed path: collapse whitespace.
			name: "token",
			spec: func(literals []valuepkg.LiteralSpec) valuepkg.TypeSpec {
				return valuepkg.TypeSpec{
					Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
					Whitespace: valuepkg.WhitespaceCollapse,
					Base:       token, ListItem: valuepkg.NoType,
					Facets: valuepkg.FacetSpec{Enumeration: literals},
				}
			},
			collapse: true,
		},
		{
			// Typed path: a second facet beside the enumeration.
			name: "string_maxlen",
			spec: func(literals []valuepkg.LiteralSpec) valuepkg.TypeSpec {
				return valuepkg.TypeSpec{
					Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString,
					Whitespace: valuepkg.WhitespacePreserve, WhitespacePresent: true,
					Base: valuepkg.NoType, ListItem: valuepkg.NoType,
					Facets: valuepkg.FacetSpec{
						Enumeration: literals,
						MaxLength:   valuepkg.CardinalityFacet{Value: 64, Present: true},
					},
				}
			},
		},
		{
			// Typed path: a non-string primitive.
			name: "anyuri",
			spec: func(literals []valuepkg.LiteralSpec) valuepkg.TypeSpec {
				return valuepkg.TypeSpec{
					Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveAnyURI,
					Whitespace: valuepkg.WhitespaceCollapse,
					Base:       anyURI, ListItem: valuepkg.NoType,
					Facets: valuepkg.FacetSpec{Enumeration: literals},
				}
			},
			collapse: true,
		},
	}
}

// enumerationKindSizes straddles the eight-literal index threshold.
var enumerationKindSizes = []int{1, 4, 8, 9, 16, 32, 1024}

func BenchmarkEnumerationKindValidation(b *testing.B) {
	for _, kind := range enumerationKinds(b) {
		for _, size := range enumerationKindSizes {
			program, id, values := enumerationKindProgram(b, kind, size)
			requireEnumerationKind(b, kind, program, id, values)
			if slices.Contains(kind.covered, size) {
				continue
			}
			for _, position := range []string{"first", "last", "miss"} {
				raw := []byte(auditEnumerationInput(values, position, "short"))
				wantFacet := position == "miss"
				b.Run(fmt.Sprintf("%s/n_%d/%s", kind.name, size, position), func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(raw)))
					for b.Loop() {
						_, err := program.ValidateBytes(id, raw, valuepkg.Resolver{}, 0, 16<<20, nil)
						if (err != nil) != wantFacet || (wantFacet && !errors.Is(err, valuepkg.ErrFacet)) {
							b.Fatalf("ValidateBytes(%q) error = %v, want facet=%t", raw, err, wantFacet)
						}
					}
				})
			}
		}
	}
}

// BenchmarkEnumerationKindCompile includes Seal, where typed literals are
// parsed and indexed.
func BenchmarkEnumerationKindCompile(b *testing.B) {
	for _, kind := range enumerationKinds(b) {
		for _, size := range enumerationKindSizes {
			spec := kind.spec(auditStringLiteralSpecs(auditStringEnumerationLiterals(size, "short")))
			b.Run(fmt.Sprintf("%s/n_%d", kind.name, size), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
					id, err := builder.Reserve()
					if err != nil {
						b.Fatalf("Reserve() error = %v", err)
					}
					if err := builder.Complete(id, spec); err != nil {
						b.Fatal(err)
					}
					if _, err := builder.Seal(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func enumerationKindProgram(tb testing.TB, kind enumerationKind, size int) (*valuepkg.Program, valuepkg.TypeID, []string) {
	tb.Helper()
	values := auditStringEnumerationLiterals(size, "short")
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		tb.Fatalf("Reserve() error = %v", err)
	}
	if err = builder.Complete(id, kind.spec(auditStringLiteralSpecs(values))); err != nil {
		tb.Fatalf("%s n=%d: %v", kind.name, size, err)
	}
	program, err := builder.Seal()
	if err != nil {
		tb.Fatalf("%s n=%d: %v", kind.name, size, err)
	}
	return program, id, values
}

// requireEnumerationKind checks the enumeration semantics every kind shares:
// each literal matches, a non-literal fails the facet, and a space-padded
// literal matches exactly when the kind collapses whitespace.
func requireEnumerationKind(tb testing.TB, kind enumerationKind, program *valuepkg.Program, id valuepkg.TypeID, values []string) {
	tb.Helper()
	validate := func(input string) error {
		_, err := program.ValidateBytes(id, []byte(input), valuepkg.Resolver{}, 0, 16<<20, nil)
		return err
	}
	for _, value := range values {
		if err := validate(value); err != nil {
			tb.Fatalf("%s n=%d: literal %q error = %v", kind.name, len(values), value, err)
		}
	}
	if err := validate(auditEnumerationInput(values, "miss", "short")); !errors.Is(err, valuepkg.ErrFacet) {
		tb.Fatalf("%s n=%d: miss error = %v, want ErrFacet", kind.name, len(values), err)
	}
	padded := " " + values[0] + " "
	if err := validate(padded); kind.collapse && err != nil {
		tb.Fatalf("%s n=%d: padded %q error = %v, want match", kind.name, len(values), padded, err)
	} else if !kind.collapse && !errors.Is(err, valuepkg.ErrFacet) {
		tb.Fatalf("%s n=%d: padded %q error = %v, want ErrFacet", kind.name, len(values), padded, err)
	}
}
