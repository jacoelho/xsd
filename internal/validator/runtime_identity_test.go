package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

type identityFixture struct {
	schema *runtime.Schema
	nsID   runtime.NamespaceID
	empty  runtime.NamespaceID

	symRoot  runtime.SymbolID
	symGroup runtime.SymbolID
	symItem  runtime.SymbolID
	symID    runtime.SymbolID

	elemRoot  runtime.ElemID
	elemGroup runtime.ElemID
	elemItem  runtime.ElemID

	typeSimple  runtime.TypeID
	typeComplex runtime.TypeID

	pathChild     runtime.PathID
	pathDescend   runtime.PathID
	pathAttrID    runtime.PathID
	pathGroupItem runtime.PathID
}

func buildIdentityFixture() identityFixture {
	builder := runtime.NewBuilder()
	empty := builder.InternNamespace(nil)
	ns := builder.InternNamespace([]byte("urn:test"))
	symRoot := builder.InternSymbol(ns, []byte("root"))
	symGroup := builder.InternSymbol(ns, []byte("group"))
	symItem := builder.InternSymbol(ns, []byte("item"))
	symID := builder.InternSymbol(empty, []byte("id"))
	schema := builder.Build()

	schema.Types = make([]runtime.Type, 3)
	schema.Types[1] = runtime.Type{Kind: runtime.TypeSimple}
	schema.Types[2] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	schema.ComplexTypes = make([]runtime.ComplexType, 2)
	schema.ComplexTypes[1] = runtime.ComplexType{Content: runtime.ContentElementOnly}

	schema.Elements = make([]runtime.Element, 4)
	schema.Elements[1] = runtime.Element{Name: symRoot, Type: 2}
	schema.Elements[2] = runtime.Element{Name: symGroup, Type: 2}
	schema.Elements[3] = runtime.Element{Name: symItem, Type: 1}

	schema.Paths = make([]runtime.PathProgram, 5)
	schema.Paths[1] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpChildName, Sym: symItem, NS: ns}}}
	schema.Paths[2] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpDescend}, {Op: runtime.OpChildName, Sym: symItem, NS: ns}}}
	schema.Paths[3] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpAttrName, Sym: symID, NS: empty}}}
	schema.Paths[4] = runtime.PathProgram{Ops: []runtime.PathOp{
		{Op: runtime.OpChildName, Sym: symGroup, NS: ns},
		{Op: runtime.OpChildName, Sym: symItem, NS: ns},
	}}

	return identityFixture{
		schema:        schema,
		nsID:          ns,
		empty:         empty,
		symRoot:       symRoot,
		symGroup:      symGroup,
		symItem:       symItem,
		symID:         symID,
		elemRoot:      1,
		elemGroup:     2,
		elemItem:      3,
		typeSimple:    1,
		typeComplex:   2,
		pathChild:     1,
		pathDescend:   2,
		pathAttrID:    3,
		pathGroupItem: 4,
	}
}

func TestIdentityUniqueMissingFieldIgnored(t *testing.T) {
	fx := buildIdentityFixture()
	schema := fx.schema

	schema.ICs = make([]runtime.IdentityConstraint, 2)
	schema.ICs[1] = runtime.IdentityConstraint{
		Category:    runtime.ICUnique,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.ICSelectors = []runtime.PathID{fx.pathChild}
	schema.ICFields = []runtime.PathID{fx.pathAttrID}
	schema.ElemICs = []runtime.ICID{1}
	schema.Elements[fx.elemRoot].ICOff = 0
	schema.Elements[fx.elemRoot].ICLen = 1

	sess := NewSession(schema)

	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemRoot, Type: fx.typeComplex, Sym: fx.symRoot, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart root: %v", err)
	}

	attrs := []StartAttr{{
		Sym:   fx.symID,
		NS:    fx.empty,
		Local: []byte("id"),
		Value: []byte("one"),
	}}
	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID, Attrs: attrs,
	}); err != nil {
		t.Fatalf("identityStart item: %v", err)
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item: %v", err)
	}

	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart item missing: %v", err)
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item missing: %v", err)
	}

	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd root: %v", err)
	}

	if len(sess.icState.violations) != 0 {
		t.Fatalf("violations = %d, want 0", len(sess.icState.violations))
	}
	if len(sess.icState.completed) != 1 {
		t.Fatalf("completed scopes = %d, want 1", len(sess.icState.completed))
	}
	scope := sess.icState.completed[0]
	if len(scope.constraints) != 1 {
		t.Fatalf("constraints = %d, want 1", len(scope.constraints))
	}
	if got := len(scope.constraints[0].rows); got != 1 {
		t.Fatalf("rows = %d, want 1", got)
	}
}

func TestIdentityKeyMissingFieldErrors(t *testing.T) {
	fx := buildIdentityFixture()
	schema := fx.schema

	schema.ICs = make([]runtime.IdentityConstraint, 2)
	schema.ICs[1] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.ICSelectors = []runtime.PathID{fx.pathChild}
	schema.ICFields = []runtime.PathID{fx.pathAttrID}
	schema.ElemICs = []runtime.ICID{1}
	schema.Elements[fx.elemRoot].ICOff = 0
	schema.Elements[fx.elemRoot].ICLen = 1

	sess := NewSession(schema)

	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemRoot, Type: fx.typeComplex, Sym: fx.symRoot, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart root: %v", err)
	}
	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart item: %v", err)
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item: %v", err)
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd root: %v", err)
	}

	if len(sess.icState.violations) == 0 {
		t.Fatalf("expected missing field violation")
	}
}

func TestIdentityKeyrefScopeIsolation(t *testing.T) {
	fx := buildIdentityFixture()
	schema := fx.schema

	schema.ICs = make([]runtime.IdentityConstraint, 5)
	schema.ICs[1] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.ICs[2] = runtime.IdentityConstraint{
		Category:    runtime.ICKeyRef,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
		Referenced:  1,
	}
	schema.ICs[3] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.ICs[4] = runtime.IdentityConstraint{
		Category:    runtime.ICKeyRef,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
		Referenced:  3,
	}
	schema.ICSelectors = []runtime.PathID{fx.pathDescend}
	schema.ICFields = []runtime.PathID{fx.pathAttrID}
	schema.ElemICs = []runtime.ICID{1, 2, 3, 4}
	schema.Elements[fx.elemRoot].ICOff = 0
	schema.Elements[fx.elemRoot].ICLen = 2
	schema.Elements[fx.elemGroup].ICOff = 2
	schema.Elements[fx.elemGroup].ICLen = 2

	sess := NewSession(schema)

	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemRoot, Type: fx.typeComplex, Sym: fx.symRoot, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart root: %v", err)
	}
	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemGroup, Type: fx.typeComplex, Sym: fx.symGroup, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart group: %v", err)
	}
	attrs := []StartAttr{{
		Sym:   fx.symID,
		NS:    fx.empty,
		Local: []byte("id"),
		Value: []byte("one"),
	}}
	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID, Attrs: attrs,
	}); err != nil {
		t.Fatalf("identityStart item: %v", err)
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item: %v", err)
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd group: %v", err)
	}
	if err := sess.identityEnd(identityEndInput{}); err != nil {
		t.Fatalf("identityEnd root: %v", err)
	}

	if len(sess.icState.completed) != 2 {
		t.Fatalf("completed scopes = %d, want 2", len(sess.icState.completed))
	}
	for _, scope := range sess.icState.completed {
		if len(scope.constraints) != 2 {
			t.Fatalf("scope constraints = %d, want 2", len(scope.constraints))
		}
		for _, constraint := range scope.constraints {
			if constraint.category == runtime.ICKey && len(constraint.rows) != 1 {
				t.Fatalf("key rows = %d, want 1", len(constraint.rows))
			}
			if constraint.category == runtime.ICKeyRef && len(constraint.keyrefs) != 1 {
				t.Fatalf("keyref rows = %d, want 1", len(constraint.keyrefs))
			}
		}
	}
}
