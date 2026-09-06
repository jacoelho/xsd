package value

import "testing"

func TestProgramIsUnconstrainedString(t *testing.T) {
	program, types := unconstrainedStringQueryProgram(t)
	tests := []struct {
		name string
		id   TypeID
		want bool
	}{
		{name: "anySimpleType", id: builtinAnySimpleType, want: true},
		{name: "string", id: builtinString, want: true},
		{name: "derived string", id: types["derived"], want: true},
		{name: "normalized string", id: builtinNormalizedString},
		{name: "identity string", id: types["identity"]},
		{name: "facet string", id: types["facet"]},
		{name: "union", id: types["union"]},
		{name: "list", id: types["list"]},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := program.IsUnconstrainedString(test.id)
			if !valid {
				t.Fatal("IsUnconstrainedString reported an invalid type")
			}
			if got != test.want {
				t.Fatalf("IsUnconstrainedString() = %v, want %v", got, test.want)
			}
		})
	}

	if got, valid := program.IsUnconstrainedString(NoType); valid || got {
		t.Fatalf("IsUnconstrainedString(NoType) = %v, %v, want false, false", got, valid)
	}
}

func TestProgramIsUnconstrainedStringDoesNotProjectMetadata(t *testing.T) {
	program, types := unconstrainedStringQueryProgram(t)
	for _, test := range []struct {
		name string
		id   TypeID
	}{
		{name: "union", id: types["union"]},
		{name: "facet", id: types["facet"]},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got bool
			var valid bool
			allocs := testing.AllocsPerRun(100, func() {
				got, valid = program.IsUnconstrainedString(test.id)
			})
			if !valid {
				t.Fatal("IsUnconstrainedString reported a valid type as invalid")
			}
			if got {
				t.Fatal("IsUnconstrainedString accepted a constrained type")
			}
			if allocs != 0 {
				t.Fatalf("IsUnconstrainedString allocations = %v, want 0", allocs)
			}
		})
	}
}

func BenchmarkProgramIsUnconstrainedString(b *testing.B) {
	program, types := unconstrainedStringQueryProgram(b)
	for _, test := range []struct {
		name string
		id   TypeID
	}{
		{name: "builtin", id: builtinString},
		{name: "union", id: types["union"]},
		{name: "facet", id: types["facet"]},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				benchmarkUnconstrainedStringResult, _ = program.IsUnconstrainedString(test.id)
			}
		})
	}
}

var benchmarkUnconstrainedStringResult bool

func unconstrainedStringQueryProgram(tb testing.TB) (*Program, map[string]TypeID) {
	tb.Helper()
	builder := NewBuilder(BuilderOptions{})
	types := make(map[string]TypeID)
	for name, spec := range map[string]TypeSpec{
		"derived": {
			Variety:           Atomic,
			Primitive:         PrimitiveString,
			Base:              builtinString,
			ListItem:          NoType,
			WhitespacePresent: false,
		},
		"identity": {
			Variety:           Atomic,
			Primitive:         PrimitiveString,
			Whitespace:        WhitespacePreserve,
			WhitespacePresent: true,
			Identity:          IdentityID,
			Base:              NoType,
			ListItem:          NoType,
		},
		"facet": {
			Variety:           Atomic,
			Primitive:         PrimitiveString,
			Whitespace:        WhitespacePreserve,
			WhitespacePresent: true,
			Base:              NoType,
			ListItem:          NoType,
			Facets: FacetSpec{Enumeration: []LiteralSpec{{
				Type:    NoType,
				Lexical: "value",
			}}},
		},
		"union": {
			Variety:           Union,
			Union:             []TypeID{builtinString, builtinInt},
			Whitespace:        WhitespaceCollapse,
			WhitespacePresent: true,
			Base:              NoType,
			ListItem:          NoType,
		},
		"list": {
			Variety:           List,
			Whitespace:        WhitespaceCollapse,
			WhitespacePresent: true,
			Base:              NoType,
			ListItem:          builtinString,
		},
	} {
		id, err := builder.Add(spec)
		if err != nil {
			tb.Fatalf("Add(%s) error = %v", name, err)
		}
		types[name] = id
	}
	program, err := builder.Seal()
	if err != nil {
		tb.Fatalf("Seal() error = %v", err)
	}
	return program, types
}
