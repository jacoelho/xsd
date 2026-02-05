package validator

import (
	"strings"
	"testing"

	xsdErrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
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

func buildIdentityFixture(tb testing.TB) identityFixture {
	tb.Helper()
	builder := runtime.NewBuilder()
	empty := mustInternNamespace(tb, builder, nil)
	ns := mustInternNamespace(tb, builder, []byte("urn:test"))
	symRoot := mustInternSymbol(tb, builder, ns, []byte("root"))
	symGroup := mustInternSymbol(tb, builder, ns, []byte("group"))
	symItem := mustInternSymbol(tb, builder, ns, []byte("item"))
	symID := mustInternSymbol(tb, builder, empty, []byte("id"))
	schema, err := builder.Build()
	if err != nil {
		tb.Fatalf("Build() error = %v", err)
	}

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
	fx := buildIdentityFixture(t)
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
		Sym:      fx.symID,
		NS:       fx.empty,
		Local:    []byte("id"),
		Value:    []byte("one"),
		KeyKind:  runtime.VKString,
		KeyBytes: []byte("one"),
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
	if pending := sess.icState.drainPending(); len(pending) != 0 {
		t.Fatalf("pending errors = %d, want 0", len(pending))
	}
}

func TestIdentityKeyMissingFieldErrors(t *testing.T) {
	fx := buildIdentityFixture(t)
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

	pending := sess.icState.drainPending()
	if len(pending) == 0 {
		t.Fatalf("expected missing field violation")
	}
	code, _, ok := validationErrorInfo(pending[0])
	if !ok || code != xsdErrors.ErrIdentityAbsent {
		t.Fatalf("expected %s, got %v", xsdErrors.ErrIdentityAbsent, pending[0])
	}
}

func TestIdentityKeyrefScopeIsolation(t *testing.T) {
	fx := buildIdentityFixture(t)
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
		Sym:      fx.symID,
		NS:       fx.empty,
		Local:    []byte("id"),
		Value:    []byte("one"),
		KeyKind:  runtime.VKString,
		KeyBytes: []byte("one"),
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

	if pending := sess.icState.drainPending(); len(pending) != 0 {
		t.Fatalf("pending errors = %d, want 0", len(pending))
	}
}

func TestIdentityStartRollbackOnError(t *testing.T) {
	builder := runtime.NewBuilder()
	ns := mustInternNamespace(t, builder, []byte("urn:test"))
	symRoot := mustInternSymbol(t, builder, ns, []byte("root"))
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	schema.Types = make([]runtime.Type, 2)
	schema.ComplexTypes = make([]runtime.ComplexType, 2)
	schema.Types[1] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	schema.ComplexTypes[1] = runtime.ComplexType{Content: runtime.ContentEmpty}

	schema.Elements = make([]runtime.Element, 2)
	schema.Elements[1] = runtime.Element{Name: symRoot, Type: 1, ICOff: 0, ICLen: 1}
	schema.GlobalElements = make([]runtime.ElemID, schema.Symbols.Count()+1)
	schema.GlobalElements[symRoot] = 1

	schema.ICs = make([]runtime.IdentityConstraint, 2)
	schema.ICs[1] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.ElemICs = []runtime.ICID{1}

	sess := NewSession(schema)
	sess.Reset()

	reader, err := xmlstream.NewReader(strings.NewReader(`<root xmlns="urn:test"/>`))
	if err != nil {
		t.Fatalf("xml reader: %v", err)
	}
	sess.reader = reader

	ev, err := sess.reader.NextResolved()
	if err != nil {
		t.Fatalf("NextResolved: %v", err)
	}
	if ev.Kind != xmlstream.EventStartElement {
		t.Fatalf("expected start element, got %v", ev.Kind)
	}
	if err := sess.handleStartElement(&ev, sessionResolver{s: sess}); err == nil {
		t.Fatalf("expected identityStart error")
	}
	if len(sess.elemStack) != 0 {
		t.Fatalf("elemStack len = %d, want 0", len(sess.elemStack))
	}
	if len(sess.nsStack) != 0 {
		t.Fatalf("nsStack len = %d, want 0", len(sess.nsStack))
	}
}
