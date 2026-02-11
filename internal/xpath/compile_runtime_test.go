package xpath

import (
	"errors"
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

func buildRuntimeXPathFixture(tb testing.TB) (*runtime.Schema, runtimeIDs) {
	tb.Helper()
	builder := runtime.NewBuilder()
	emptyNS, err := builder.InternNamespace(nil)
	if err != nil {
		tb.Fatalf("InternNamespace: %v", err)
	}
	ns, err := builder.InternNamespace([]byte("urn:test"))
	if err != nil {
		tb.Fatalf("InternNamespace: %v", err)
	}
	symA, err := builder.InternSymbol(ns, []byte("a"))
	if err != nil {
		tb.Fatalf("InternSymbol: %v", err)
	}
	symB, err := builder.InternSymbol(ns, []byte("b"))
	if err != nil {
		tb.Fatalf("InternSymbol: %v", err)
	}
	symAttr, err := builder.InternSymbol(ns, []byte("attr"))
	if err != nil {
		tb.Fatalf("InternSymbol: %v", err)
	}
	symID, err := builder.InternSymbol(emptyNS, []byte("id"))
	if err != nil {
		tb.Fatalf("InternSymbol: %v", err)
	}
	symAEmpty, err := builder.InternSymbol(emptyNS, []byte("a"))
	if err != nil {
		tb.Fatalf("InternSymbol: %v", err)
	}
	schema, err := builder.Build()
	if err != nil {
		tb.Fatalf("Build() error = %v", err)
	}
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
	schema, ids := buildRuntimeXPathFixture(t)
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

func TestCompileProgramsDescendMidPath(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture(t)
	nsContext := map[string]string{"t": "urn:test"}

	programs, err := CompilePrograms("t:a//t:b", nsContext, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}
	if len(programs) != 1 {
		t.Fatalf("programs = %d, want 1", len(programs))
	}

	want := []runtime.PathOp{
		{Op: runtime.OpChildName, Sym: ids.symA, NS: ids.ns},
		{Op: runtime.OpDescend},
		{Op: runtime.OpChildName, Sym: ids.symB, NS: ids.ns},
	}
	if !reflect.DeepEqual(programs[0].Ops, want) {
		t.Fatalf("ops = %#v, want %#v", programs[0].Ops, want)
	}
}

func TestCompileProgramsRootSelf(t *testing.T) {
	schema, _ := buildRuntimeXPathFixture(t)

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
	schema, ids := buildRuntimeXPathFixture(t)
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
	schema, ids := buildRuntimeXPathFixture(t)
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

func TestCompileProgramsErrorsAreWrapped(t *testing.T) {
	if _, err := CompilePrograms(".", nil, AttributesDisallowed, nil); err == nil {
		t.Fatal("expected error for nil schema")
	} else if !errors.Is(err, ErrInvalidXPath) {
		t.Fatalf("nil schema error = %v, want ErrInvalidXPath", err)
	}

	schema := &runtime.Schema{}
	if _, err := CompilePrograms("[invalid", nil, AttributesDisallowed, schema); err == nil {
		t.Fatal("expected error for invalid xpath")
	} else if !errors.Is(err, ErrInvalidXPath) {
		t.Fatalf("invalid xpath error = %v, want ErrInvalidXPath", err)
	}
}

func TestCompileProgramsNamespaceWildcard(t *testing.T) {
	schema, ids := buildRuntimeXPathFixture(t)
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
	schema, ids := buildRuntimeXPathFixture(t)

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
	schema, ids := buildRuntimeXPathFixture(t)
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
	schema, _ := buildRuntimeXPathFixture(t)
	nsContext := map[string]string{"t": "urn:test"}

	if _, err := CompilePrograms("t:missing", nsContext, AttributesDisallowed, schema); err == nil {
		t.Fatalf("expected missing symbol error")
	}
}

func TestCompileExpressionParity(t *testing.T) {
	schema, _ := buildRuntimeXPathFixture(t)
	nsContext := map[string]string{"t": "urn:test"}
	expr := "t:a|t:b"

	parsed, err := Parse(expr, nsContext, AttributesDisallowed)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	fromParsed, err := CompileExpression(parsed, schema)
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	direct, err := CompilePrograms(expr, nsContext, AttributesDisallowed, schema)
	if err != nil {
		t.Fatalf("CompilePrograms: %v", err)
	}

	if !reflect.DeepEqual(fromParsed, direct) {
		t.Fatalf("from parsed = %#v, direct = %#v", fromParsed, direct)
	}
}
