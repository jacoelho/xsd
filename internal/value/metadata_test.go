package value

import (
	"errors"
	"reflect"
	"testing"
)

func TestProgramInputRequirementsUnconstrainedString(t *testing.T) {
	program, types := unconstrainedStringQueryProgram(t)
	tests := []struct {
		name string
		id   TypeID
		want bool
	}{
		{name: "anySimpleType", id: AnySimpleTypeID, want: true},
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
			requirements, valid := program.InputRequirements(test.id)
			got := requirements.UnconstrainedString
			if !valid {
				t.Fatal("InputRequirements reported an invalid type")
			}
			if got != test.want {
				t.Fatalf("InputRequirements() = %v, want %v", got, test.want)
			}
		})
	}

	if got, valid := program.InputRequirements(NoType); valid || got.UnconstrainedString {
		t.Fatalf("InputRequirements(NoType) = %v, %v, want zero, false", got, valid)
	}
}

func TestProgramInputRequirementsOwnsScalarAdmissionPolicy(t *testing.T) {
	program, types := unconstrainedStringQueryProgram(t)
	for _, test := range []struct {
		name                string
		id                  TypeID
		needsQName          bool
		identity            IdentityKind
		unconstrainedString bool
	}{
		{name: "string", id: builtinString, unconstrainedString: true},
		{name: "identity", id: types["identity"], identity: IdentityID},
		{name: "list", id: types["list"]},
		{name: "qname", id: builtinQName, needsQName: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, valid := program.InputRequirements(test.id)
			if !valid {
				t.Fatal("InputRequirements reported an invalid type")
			}
			if got.NeedsQName != test.needsQName || got.Identity != test.identity || got.UnconstrainedString != test.unconstrainedString {
				t.Fatalf("InputRequirements() = %#v, want NeedsQName=%v Identity=%d UnconstrainedString=%v", got, test.needsQName, test.identity, test.unconstrainedString)
			}
		})
	}
	if got, valid := program.InputRequirements(NoType); valid || got != (InputRequirements{}) {
		t.Fatalf("InputRequirements(NoType) = %#v, %v, want zero,false", got, valid)
	}
}

func TestBuilderMetadataLifecycle(t *testing.T) {
	assertUnavailable := func(builder *Builder, id TypeID) {
		t.Helper()
		if got, valid := builder.InputRequirements(id); valid || got != (InputRequirements{}) {
			t.Fatalf("builder InputRequirements(%d) = %#v, %v; want zero, false", id, got, valid)
		}
		if got, valid := builder.TypeView(id); valid || !reflect.DeepEqual(got, TypeView{}) {
			t.Fatalf("builder TypeView(%d) = %#v, %v; want zero, false", id, got, valid)
		}
	}
	assertUnavailable(nil, builtinString)
	assertUnavailable(&Builder{}, builtinString)
	builder := NewBuilder(BuilderOptions{})
	assertUnavailable(builder, NoType)
	assertUnavailable(builder, BuiltinTypeCount+99)
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	assertUnavailable(builder, id)
	if got, valid := builder.program.InputRequirements(builtinString); valid || got != (InputRequirements{}) {
		t.Fatalf("unsealed InputRequirements() = %#v, %v; want zero, false", got, valid)
	}
	spec := TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	}
	if completeErr := builder.Complete(id, spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	requirements, requirementsValid := builder.InputRequirements(id)
	if !requirementsValid || !requirements.UnconstrainedString {
		t.Fatalf("builder InputRequirements() = %#v, %v; want unconstrained string, true", requirements, requirementsValid)
	}
	view, viewValid := builder.TypeView(id)
	if !viewValid || view.Variety != Atomic || view.Primitive != PrimitiveString || view.Whitespace != WhitespacePreserve {
		t.Fatalf("completed TypeView() = %#v, %v; want atomic string with preserved whitespace", view, viewValid)
	}
	if _, validateErr := builder.Validate(id, "value", Resolver{}, 0, nil); validateErr != nil {
		t.Fatalf("builder Validate() = %v", validateErr)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	assertUnavailable(builder, id)
	if reserved, reserveErr := builder.Reserve(); reserved != NoType || !errors.Is(reserveErr, ErrMetadata) {
		t.Fatalf("Reserve after Seal = %d, %v; want NoType, ErrMetadata", reserved, reserveErr)
	}
	if capacityErr := builder.ReserveCapacity(2); !errors.Is(capacityErr, ErrMetadata) {
		t.Fatalf("ReserveCapacity after Seal = %v; want ErrMetadata", capacityErr)
	}
	if completionErr := builder.Complete(id, spec); !errors.Is(completionErr, ErrMetadata) {
		t.Fatalf("Complete after Seal = %v; want ErrMetadata", completionErr)
	}
	if _, validationErr := builder.Validate(id, "x", Resolver{}, NeedCanonical, nil); !errors.Is(validationErr, ErrMetadata) {
		t.Fatalf("Builder Validate after Seal = %v; want ErrMetadata", validationErr)
	}
	if repeated, sealErr := builder.Seal(); repeated != nil || !errors.Is(sealErr, ErrMetadata) {
		t.Fatalf("repeated Seal = %v, %v; want nil, ErrMetadata", repeated, sealErr)
	}
	if got, validationErr := program.Validate(id, "x", Resolver{}, NeedCanonical, 16<<20, nil); validationErr != nil || got.CanonicalText() != "x" {
		t.Fatalf("published Validate after rejected Builder calls = %q, %v; want x, nil", got.CanonicalText(), validationErr)
	}
	if got, valid := program.InputRequirements(id); !valid || got != requirements {
		t.Fatalf("sealed InputRequirements() = %#v, %v; want %#v, true", got, valid, requirements)
	}
	for _, test := range []struct {
		name    string
		program *Program
		id      TypeID
	}{
		{name: "nil", id: builtinString},
		{name: "zero", program: &Program{}, id: builtinString},
		{name: "absent type", program: program, id: NoType},
		{name: "out of range", program: program, id: id + 99},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, valid := test.program.InputRequirements(test.id); valid || got != (InputRequirements{}) {
				t.Fatalf("InputRequirements(%d) = %#v, %v; want zero, false", test.id, got, valid)
			}
		})
	}
}

func TestBuilderTypeViewOwnsUnionCopy(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if completeErr := builder.Complete(id, TypeSpec{
		Variety: Union, Primitive: PrimitiveString,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Union: []TypeID{builtinBoolean, builtinDecimal},
	}); completeErr != nil {
		t.Fatal(completeErr)
	}
	view, valid := builder.TypeView(id)
	if !valid || len(view.Union) != 2 {
		t.Fatalf("TypeView() = %#v, %v; want two union members", view, valid)
	}
	view.Union[0] = builtinInt
	again, valid := builder.TypeView(id)
	if !valid || !reflect.DeepEqual(again.Union, []TypeID{builtinBoolean, builtinDecimal}) {
		t.Fatalf("TypeView() after returned-slice mutation = %#v, %v; want boolean, decimal", again, valid)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	validated, err := program.Validate(id, "true", Resolver{}, 0, 16<<20, nil)
	if err != nil || validated.SelectedType() != builtinBoolean {
		t.Fatalf("Validate(true) = type %d, %v; want boolean", validated.SelectedType(), err)
	}
}

func TestBuilderTypeViewReportsEffectiveEnumeration(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	base := NoType
	for _, test := range []struct {
		name   string
		facets FacetSpec
		want   bool
	}{
		{name: "absent"},
		{name: "own", facets: FacetSpec{Enumeration: []LiteralSpec{{Type: NoType, Lexical: "value"}}}, want: true},
		{name: "inherited", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if err := builder.Complete(id, TypeSpec{
				Variety: Atomic, Primitive: PrimitiveString,
				Whitespace: WhitespacePreserve, WhitespacePresent: true,
				Base: base, ListItem: NoType, Facets: test.facets,
			}); err != nil {
				t.Fatal(err)
			}
			view, valid := builder.TypeView(id)
			if !valid || view.HasEnumeration != test.want {
				t.Fatalf("TypeView().HasEnumeration = %v, %v; want %v, true", view.HasEnumeration, valid, test.want)
			}
			base = id
		})
	}
}

func TestProgramInputRequirementsDoesNotProjectMetadata(t *testing.T) {
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
				requirements, ok := program.InputRequirements(test.id)
				got, valid = requirements.UnconstrainedString, ok
			})
			if !valid {
				t.Fatal("InputRequirements reported a valid type as invalid")
			}
			if got {
				t.Fatal("InputRequirements accepted a constrained type")
			}
			if allocs != 0 {
				t.Fatalf("InputRequirements allocations = %v, want 0", allocs)
			}
		})
	}
}

func BenchmarkProgramInputRequirementsUnconstrainedString(b *testing.B) {
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
				requirements, _ := program.InputRequirements(test.id)
				benchmarkUnconstrainedStringResult = requirements.UnconstrainedString
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
		id, err := builder.Reserve()
		if err != nil {
			tb.Fatalf("Reserve() error = %v", err)
		}
		err = builder.Complete(id, spec)
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
