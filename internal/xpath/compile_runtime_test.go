package xpath

import (
	"reflect"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

type runtimeIDs struct {
	ns        runtime.NamespaceID
	emptyNS   runtime.NamespaceID
	symA      runtime.SymbolID
	symB      runtime.SymbolID
	symAttr   runtime.SymbolID
	symID     runtime.SymbolID
	symAEmpty runtime.SymbolID
}

func buildRuntimeXPathFixture() (*runtime.Schema, runtimeIDs) {
	builder := runtime.NewBuilder()
	emptyNS := builder.InternNamespace(nil)
	ns := builder.InternNamespace([]byte("urn:test"))
	symA := builder.InternSymbol(ns, []byte("a"))
	symB := builder.InternSymbol(ns, []byte("b"))
	symAttr := builder.InternSymbol(ns, []byte("attr"))
	symID := builder.InternSymbol(emptyNS, []byte("id"))
	symAEmpty := builder.InternSymbol(emptyNS, []byte("a"))
	schema := builder.Build()
	return schema, runtimeIDs{
		ns:        ns,
		emptyNS:   emptyNS,
		symA:      symA,
		symB:      symB,
		symAttr:   symAttr,
		symID:     symID,
		symAEmpty: symAEmpty,
	}
}

func TestCompileProgramsDescend(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture()
	nsContext := map[string]string{"t": "urn:test"}

	programs, err := CompilePrograms(".//t:a/t:b", nsContext, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	if len(programs) != 1 {
		t.Fatalf("programs = %d, want 1", len(programs))
	}

	want := []runtime.PathOp{
		{Op: runtime.OpDescend},
		{Op: runtime.OpChildName, Sym: ids.symA, NS: ids.ns},
		{Op: runtime.OpChildName, Sym: ids.symB, NS: ids.ns},
	}
	if !reflect.DeepEqual(programs[0].Ops, want) {
		t.Fatalf("ops = %#v, want %#v", programs[0].Ops, want)
	}
}

func TestCompileProgramsRootSelf(t *testing.T) {
	schema, _ := buildRuntimeXPathFixture()

	programs, err := CompilePrograms(".", nil, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	want := []runtime.PathOp{{Op: runtime.OpRootSelf}}
	if !reflect.DeepEqual(programs[0].Ops, want) {
		t.Fatalf("ops = %#v, want %#v", programs[0].Ops, want)
	}
}

func TestCompileProgramsSelfStep(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture()
	nsContext := map[string]string{"t": "urn:test"}

	programs, err := CompilePrograms("t:a/.", nsContext, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	want := []runtime.PathOp{
		{Op: runtime.OpChildName, Sym: ids.symA, NS: ids.ns},
		{Op: runtime.OpSelf},
	}
	if !reflect.DeepEqual(programs[0].Ops, want) {
		t.Fatalf("ops = %#v, want %#v", programs[0].Ops, want)
	}
}

func TestCompileProgramsAttribute(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture()
	nsContext := map[string]string{"t": "urn:test"}

	programs, err := CompilePrograms("t:a/@id", nsContext, AttributesAllowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	want := []runtime.PathOp{
		{Op: runtime.OpChildName, Sym: ids.symA, NS: ids.ns},
		{Op: runtime.OpAttrName, Sym: ids.symID, NS: ids.emptyNS},
	}
	if !reflect.DeepEqual(programs[0].Ops, want) {
		t.Fatalf("ops = %#v, want %#v", programs[0].Ops, want)
	}
}

func TestCompileProgramsNamespaceWildcard(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture()
	nsContext := map[string]string{"t": "urn:test"}

	programs, err := CompilePrograms("t:*", nsContext, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	want := []runtime.PathOp{{Op: runtime.OpChildNSAny, NS: ids.ns}}
	if !reflect.DeepEqual(programs[0].Ops, want) {
		t.Fatalf("ops = %#v, want %#v", programs[0].Ops, want)
	}
}

func TestCompileProgramsUnprefixedNameUsesEmptyNamespace(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture()

	programs, err := CompilePrograms("a", nil, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	want := []runtime.PathOp{{Op: runtime.OpChildName, Sym: ids.symAEmpty, NS: ids.emptyNS}}
	if !reflect.DeepEqual(programs[0].Ops, want) {
		t.Fatalf("ops = %#v, want %#v", programs[0].Ops, want)
	}
}

func TestCompileProgramsUnion(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture()
	nsContext := map[string]string{"t": "urn:test"}

	programs, err := CompilePrograms("t:a|t:b", nsContext, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	if len(programs) != 2 {
		t.Fatalf("programs = %d, want 2", len(programs))
	}
	want0 := []runtime.PathOp{{Op: runtime.OpChildName, Sym: ids.symA, NS: ids.ns}}
	want1 := []runtime.PathOp{{Op: runtime.OpChildName, Sym: ids.symB, NS: ids.ns}}
	if !reflect.DeepEqual(programs[0].Ops, want0) || !reflect.DeepEqual(programs[1].Ops, want1) {
		t.Fatalf("ops = %#v, %#v, want %#v, %#v", programs[0].Ops, programs[1].Ops, want0, want1)
	}
}

func TestCompileProgramsMissingSymbol(t *testing.T) {
	schema, _ := buildRuntimeXPathFixture()
	nsContext := map[string]string{"t": "urn:test"}

	if _, err := CompilePrograms("t:missing", nsContext, AttributesDisallowed, schema); err == nil {
		t.Fatalf("expected missing symbol error")
	}
}
