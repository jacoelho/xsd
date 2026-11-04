package valuebench

import (
	"errors"
	"fmt"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

// enumerationValueKind is a non-string enumeration whose group is value-keyed
// once it reaches the index threshold.
type enumerationValueKind struct {
	name    string
	base    string
	list    bool
	literal func(i int) string
	miss    string
}

var enumerationValueKinds = []enumerationValueKind{
	{name: "decimal", base: "decimal", literal: func(i int) string { return fmt.Sprintf("%d.5", i) }, miss: "-1.5"},
	{name: "integer", base: "integer", literal: func(i int) string { return fmt.Sprint(i * 7) }, miss: "-1"},
	{name: "dateTime", base: "dateTime", literal: func(i int) string {
		// One distinct second of the day per literal; sizes stay below 86400.
		return fmt.Sprintf("2000-01-01T%02d:%02d:%02dZ", i/3600, i/60%60, i%60)
	}, miss: "1999-01-01T00:00:00Z"},
	{name: "hexBinary", base: "hexBinary", literal: func(i int) string { return fmt.Sprintf("%08X", i) }, miss: "FFFFFFFF0A"},
	{name: "decimal_list", base: "decimal", list: true, literal: func(i int) string { return fmt.Sprintf("%d %d", i, i+1) }, miss: "-1 -2"},
}

// enumerationValueSizes straddles the index threshold and reaches sizes where
// an unindexed scan dominates.
var enumerationValueSizes = []int{4, 7, 8, 16, 32, 2048, 8192}

func enumerationValueLiterals(kind enumerationValueKind, size int) []valuepkg.LiteralSpec {
	literals := make([]valuepkg.LiteralSpec, size)
	for i := range literals {
		literals[i] = valuepkg.LiteralSpec{Lexical: kind.literal(i), Type: valuepkg.NoType}
	}
	return literals
}

// enumerationValueProgram builds a chain of steps restrictions that each
// enumerate the same literals; every step after the first validates each
// literal against its base's group at construction.
func enumerationValueProgram(tb testing.TB, kind enumerationValueKind, size, steps int) (*valuepkg.Program, valuepkg.TypeID) {
	tb.Helper()
	builtin, ok := valuepkg.BuiltinTypeID(kind.base)
	if !ok {
		tb.Fatalf("BuiltinTypeID(%q) missing", kind.base)
	}
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	complete := func(spec valuepkg.TypeSpec) valuepkg.TypeID {
		id, err := builder.Reserve()
		if err != nil {
			tb.Fatalf("Reserve() error = %v", err)
		}
		if err := builder.Complete(id, spec); err != nil {
			tb.Fatalf("%s n=%d: %v", kind.name, size, err)
		}
		return id
	}
	base := valuepkg.TypeSpec{Variety: valuepkg.Atomic, Primitive: primitiveOf(kind.base), Base: builtin, ListItem: valuepkg.NoType}
	if kind.base == "integer" {
		base.Builtin = valuepkg.BuiltinInteger
	}
	if kind.list {
		base = valuepkg.TypeSpec{
			Variety: valuepkg.List, ListItem: builtin, Base: valuepkg.NoType,
			Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
		}
		listID := complete(base)
		base.Base = listID
	}
	literals := enumerationValueLiterals(kind, size)
	restricted := base
	restricted.Facets = valuepkg.FacetSpec{Enumeration: literals}
	id := complete(restricted)
	for range steps - 1 {
		restricted.Base = id
		id = complete(restricted)
	}
	program, err := builder.Seal()
	if err != nil {
		tb.Fatalf("%s n=%d: %v", kind.name, size, err)
	}
	return program, id
}

func BenchmarkEnumerationValueKindValidation(b *testing.B) {
	for _, kind := range enumerationValueKinds {
		for _, size := range enumerationValueSizes {
			program, id := enumerationValueProgram(b, kind, size, 1)
			for _, position := range []string{"first", "last", "miss"} {
				input := kind.miss
				switch position {
				case "first":
					input = kind.literal(0)
				case "last":
					input = kind.literal(size - 1)
				}
				raw := []byte(input)
				wantFacet := position == "miss"
				b.Run(fmt.Sprintf("%s/n_%d/%s", kind.name, size, position), func(b *testing.B) {
					b.ReportAllocs()
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

// BenchmarkEnumerationValueKindDerivedCompile measures a restriction that
// repeats its base's literals: without an index it is quadratic in size.
func BenchmarkEnumerationValueKindDerivedCompile(b *testing.B) {
	for _, kind := range enumerationValueKinds {
		for _, size := range enumerationValueSizes {
			b.Run(fmt.Sprintf("%s/n_%d", kind.name, size), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					enumerationValueProgram(b, kind, size, 2)
				}
			})
		}
	}
}

func primitiveOf(local string) valuepkg.PrimitiveKind {
	switch local {
	case "dateTime":
		return valuepkg.PrimitiveDateTime
	case "hexBinary":
		return valuepkg.PrimitiveHexBinary
	default:
		return valuepkg.PrimitiveDecimal
	}
}
