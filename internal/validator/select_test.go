package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

type testFrame struct {
	sym runtime.SymbolID
	ns  runtime.NamespaceID
}

func (f testFrame) MatchSymbol() runtime.SymbolID {
	return f.sym
}

func (f testFrame) MatchNamespace() runtime.NamespaceID {
	return f.ns
}

type selectionFixture struct {
	rt       *runtime.Schema
	nsID     runtime.NamespaceID
	emptyNS  runtime.NamespaceID
	rootElem runtime.ElemID
	itemType runtime.TypeID
	itemSym  runtime.SymbolID
	attrSym  runtime.SymbolID
}

func buildSelectionFixture(tb testing.TB) selectionFixture {
	tb.Helper()

	builder := runtime.NewBuilder()
	emptyNS, err := builder.InternNamespace(nil)
	if err != nil {
		tb.Fatalf("InternNamespace(empty): %v", err)
	}
	nsID, err := builder.InternNamespace([]byte("urn:test"))
	if err != nil {
		tb.Fatalf("InternNamespace(ns): %v", err)
	}
	rootSym, err := builder.InternSymbol(nsID, []byte("root"))
	if err != nil {
		tb.Fatalf("InternSymbol(root): %v", err)
	}
	itemSym, err := builder.InternSymbol(nsID, []byte("item"))
	if err != nil {
		tb.Fatalf("InternSymbol(item): %v", err)
	}
	attrSym, err := builder.InternSymbol(emptyNS, []byte("id"))
	if err != nil {
		tb.Fatalf("InternSymbol(id): %v", err)
	}
	nameSym, err := builder.InternSymbol(nsID, []byte("u1"))
	if err != nil {
		tb.Fatalf("InternSymbol(u1): %v", err)
	}
	rt, err := builder.Build()
	if err != nil {
		tb.Fatalf("Build(): %v", err)
	}

	setRuntimeTypes(tb, rt, make([]runtime.Type, 3))
	rt.TypeTable()[1] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	rt.TypeTable()[2] = runtime.Type{Kind: runtime.TypeSimple}
	setRuntimeComplexTypes(tb, rt, make([]runtime.ComplexType, 2))
	rt.ComplexTypeTable()[1] = runtime.ComplexType{Content: runtime.ContentElementOnly}

	setRuntimeElements(tb, rt, make([]runtime.Element, 3))
	rt.ElementTable()[1] = runtime.Element{Name: rootSym, Type: 1, ICOff: 0, ICLen: 1}
	rt.ElementTable()[2] = runtime.Element{Name: itemSym, Type: 2}

	setRuntimePaths(tb, rt, make([]runtime.PathProgram, 3))
	rt.PathPrograms()[1] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpChildName, Sym: itemSym, NS: nsID}}}
	rt.PathPrograms()[2] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpAttrName, Sym: attrSym, NS: emptyNS}}}

	setRuntimeIdentityConstraints(tb, rt, make([]runtime.IdentityConstraint, 2))
	rt.IdentityConstraints()[1] = runtime.IdentityConstraint{
		Name:        nameSym,
		Category:    runtime.ICUnique,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	setRuntimeElementIdentityConstraints(tb, rt, []runtime.ICID{1})
	setRuntimeIdentitySelectors(tb, rt, []runtime.PathID{1})
	setRuntimeIdentityFields(tb, rt, []runtime.PathID{2})

	return selectionFixture{
		rt:       rt,
		nsID:     nsID,
		emptyNS:  emptyNS,
		rootElem: 1,
		itemType: 2,
		itemSym:  itemSym,
		attrSym:  attrSym,
	}
}

func TestOpenScopeBuildsConstraintState(t *testing.T) {
	fx := buildSelectionFixture(t)

	scope, ok, err := OpenScope(fx.rt, 7, 0, fx.rootElem, &fx.rt.ElementTable()[fx.rootElem])
	if err != nil {
		t.Fatalf("OpenScope(): %v", err)
	}
	if !ok {
		t.Fatalf("OpenScope() ok = false, want true")
	}
	if scope.RootID != 7 || scope.RootDepth != 0 || scope.RootElem != fx.rootElem {
		t.Fatalf("scope root = %+v", scope)
	}
	if len(scope.Constraints) != 1 {
		t.Fatalf("constraints = %d, want 1", len(scope.Constraints))
	}
	constraint := scope.Constraints[0]
	if constraint.Name != "{urn:test}u1" {
		t.Fatalf("constraint name = %q, want %q", constraint.Name, "{urn:test}u1")
	}
	if len(constraint.Selectors) != 1 || len(constraint.Fields) != 1 || len(constraint.Fields[0]) != 1 {
		t.Fatalf("constraint paths = %+v", constraint)
	}
}

func TestMatchSelectorsAndApplySelections(t *testing.T) {
	fx := buildSelectionFixture(t)
	scope, ok, err := OpenScope(fx.rt, 1, 0, fx.rootElem, &fx.rt.ElementTable()[fx.rootElem])
	if err != nil {
		t.Fatalf("OpenScope(): %v", err)
	}
	if !ok {
		t.Fatalf("OpenScope() ok = false, want true")
	}
	scopes := []Scope{scope}
	frames := []testFrame{
		{sym: fx.rt.ElementTable()[fx.rootElem].Name, ns: fx.nsID},
		{sym: fx.itemSym, ns: fx.nsID},
	}

	matches := MatchSelectors(fx.rt, scopes, frames, 2, 1, nil)
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1", len(matches))
	}

	attrs := []Attr{{
		Sym:      fx.attrSym,
		NS:       fx.emptyNS,
		KeyKind:  runtime.VKString,
		KeyBytes: []byte("one"),
	}}
	captures, errs := ApplySelections(fx.rt, scopes, frames, 1, 2, fx.itemType, attrs, nil)
	if len(errs) != 0 {
		t.Fatalf("errs = %v, want none", errs)
	}
	if len(captures) != 0 {
		t.Fatalf("captures = %d, want 0", len(captures))
	}

	match := scopes[0].Constraints[0].Matches[2]
	if match == nil {
		t.Fatalf("expected match to be registered in scope")
	}
	field := match.Fields[0]
	if !field.HasValue {
		t.Fatalf("field.HasValue = false, want true")
	}
	if got := string(field.KeyBytes); got != "one" {
		t.Fatalf("field value = %q, want %q", got, "one")
	}
}
