package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xmlstream"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
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

func configureRootUniqueAttrConstraint(tb testing.TB, schema *runtime.Schema, root runtime.ElemID, selector, field runtime.PathID) {
	tb.Helper()
	setRuntimeIdentityConstraints(tb, schema, make([]runtime.IdentityConstraint, 2))
	schema.IdentityConstraints()[1] = runtime.IdentityConstraint{
		Category:    runtime.ICUnique,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	setRuntimeIdentitySelectors(tb, schema, []runtime.PathID{selector})
	setRuntimeIdentityFields(tb, schema, []runtime.PathID{field})
	setRuntimeElementIdentityConstraints(tb, schema, []runtime.ICID{1})
	schema.ElementTable()[root].ICOff = 0
	schema.ElementTable()[root].ICLen = 1
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

	setRuntimeTypes(tb, schema, make([]runtime.Type, 3))
	schema.TypeTable()[1] = runtime.Type{Kind: runtime.TypeSimple}
	schema.TypeTable()[2] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	setRuntimeComplexTypes(tb, schema, make([]runtime.ComplexType, 2))
	schema.ComplexTypeTable()[1] = runtime.ComplexType{Content: runtime.ContentElementOnly}

	setRuntimeElements(tb, schema, make([]runtime.Element, 4))
	schema.ElementTable()[1] = runtime.Element{Name: symRoot, Type: 2}
	schema.ElementTable()[2] = runtime.Element{Name: symGroup, Type: 2}
	schema.ElementTable()[3] = runtime.Element{Name: symItem, Type: 1}

	setRuntimePaths(tb, schema, make([]runtime.PathProgram, 5))
	schema.PathPrograms()[1] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpChildName, Sym: symItem, NS: ns}}}
	schema.PathPrograms()[2] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpDescend}, {Op: runtime.OpChildName, Sym: symItem, NS: ns}}}
	schema.PathPrograms()[3] = runtime.PathProgram{Ops: []runtime.PathOp{{Op: runtime.OpAttrName, Sym: symID, NS: empty}}}
	schema.PathPrograms()[4] = runtime.PathProgram{Ops: []runtime.PathOp{
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

	setRuntimeIdentityConstraints(t, schema, make([]runtime.IdentityConstraint, 2))
	schema.IdentityConstraints()[1] = runtime.IdentityConstraint{
		Category:    runtime.ICUnique,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	setRuntimeIdentitySelectors(t, schema, []runtime.PathID{fx.pathChild})
	setRuntimeIdentityFields(t, schema, []runtime.PathID{fx.pathAttrID})
	setRuntimeElementIdentityConstraints(t, schema, []runtime.ICID{1})
	schema.ElementTable()[fx.elemRoot].ICOff = 0
	schema.ElementTable()[fx.elemRoot].ICLen = 1

	sess := NewSession(schema)

	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemRoot, Type: fx.typeComplex, Sym: fx.symRoot, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart root: %v", err)
	}

	startAttrs := []Start{{
		Sym:      fx.symID,
		NS:       fx.empty,
		Local:    []byte("id"),
		Value:    []byte("one"),
		KeyKind:  runtime.VKString,
		KeyBytes: []byte("one"),
	}}
	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID, Attrs: startAttrs,
	}); err != nil {
		t.Fatalf("identityStart item: %v", err)
	}
	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item: %v", err)
	}

	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID,
	}); err != nil {
		t.Fatalf("identityStart item missing: %v", err)
	}
	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item missing: %v", err)
	}

	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd root: %v", err)
	}

	if len(sess.identity.icState.Uncommitted) != 0 {
		t.Fatalf("violations = %d, want 0", len(sess.identity.icState.Uncommitted))
	}
	if pending := xsderrors.AppendIssues(nil, sess.identity.icState.DrainCommitted()); len(pending) != 0 {
		t.Fatalf("pending errors = %d, want 0", len(pending))
	}
}

func TestIdentityKeyMissingFieldErrors(t *testing.T) {
	fx := buildIdentityFixture(t)
	schema := fx.schema

	setRuntimeIdentityConstraints(t, schema, make([]runtime.IdentityConstraint, 2))
	schema.IdentityConstraints()[1] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	setRuntimeIdentitySelectors(t, schema, []runtime.PathID{fx.pathChild})
	setRuntimeIdentityFields(t, schema, []runtime.PathID{fx.pathAttrID})
	setRuntimeElementIdentityConstraints(t, schema, []runtime.ICID{1})
	schema.ElementTable()[fx.elemRoot].ICOff = 0
	schema.ElementTable()[fx.elemRoot].ICLen = 1

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
	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item: %v", err)
	}
	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd root: %v", err)
	}

	pending := xsderrors.AppendIssues(nil, sess.identity.icState.DrainCommitted())
	if len(pending) == 0 {
		t.Fatalf("expected missing field violation")
	}
	code, ok := validationErrorInfo(pending[0])
	if !ok || code != xsderrors.ErrIdentityAbsent {
		t.Fatalf("expected %s, got %v", xsderrors.ErrIdentityAbsent, pending[0])
	}
}

func TestIdentityKeyrefScopeIsolation(t *testing.T) {
	fx := buildIdentityFixture(t)
	schema := fx.schema

	setRuntimeIdentityConstraints(t, schema, make([]runtime.IdentityConstraint, 5))
	schema.IdentityConstraints()[1] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.IdentityConstraints()[2] = runtime.IdentityConstraint{
		Category:    runtime.ICKeyRef,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
		Referenced:  1,
	}
	schema.IdentityConstraints()[3] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	schema.IdentityConstraints()[4] = runtime.IdentityConstraint{
		Category:    runtime.ICKeyRef,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
		Referenced:  3,
	}
	setRuntimeIdentitySelectors(t, schema, []runtime.PathID{fx.pathDescend})
	setRuntimeIdentityFields(t, schema, []runtime.PathID{fx.pathAttrID})
	setRuntimeElementIdentityConstraints(t, schema, []runtime.ICID{1, 2, 3, 4})
	schema.ElementTable()[fx.elemRoot].ICOff = 0
	schema.ElementTable()[fx.elemRoot].ICLen = 2
	schema.ElementTable()[fx.elemGroup].ICOff = 2
	schema.ElementTable()[fx.elemGroup].ICLen = 2

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
	startAttrs := []Start{{
		Sym:      fx.symID,
		NS:       fx.empty,
		Local:    []byte("id"),
		Value:    []byte("one"),
		KeyKind:  runtime.VKString,
		KeyBytes: []byte("one"),
	}}
	if err := sess.identityStart(identityStartInput{
		Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID, Attrs: startAttrs,
	}); err != nil {
		t.Fatalf("identityStart item: %v", err)
	}
	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd item: %v", err)
	}
	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd group: %v", err)
	}
	if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
		t.Fatalf("identityEnd root: %v", err)
	}

	if pending := xsderrors.AppendIssues(nil, sess.identity.icState.DrainCommitted()); len(pending) != 0 {
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

	setRuntimeTypes(t, schema, make([]runtime.Type, 2))
	setRuntimeComplexTypes(t, schema, make([]runtime.ComplexType, 2))
	schema.TypeTable()[1] = runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}}
	schema.ComplexTypeTable()[1] = runtime.ComplexType{Content: runtime.ContentEmpty}

	setRuntimeElements(t, schema, make([]runtime.Element, 2))
	schema.ElementTable()[1] = runtime.Element{Name: symRoot, Type: 1, ICOff: 0, ICLen: 1}
	setRuntimeGlobalElements(t, schema, make([]runtime.ElemID, schema.SymbolCount()+1))
	schema.GlobalElementIDs()[symRoot] = 1

	setRuntimeIdentityConstraints(t, schema, make([]runtime.IdentityConstraint, 2))
	schema.IdentityConstraints()[1] = runtime.IdentityConstraint{
		Category:    runtime.ICKey,
		SelectorOff: 0,
		SelectorLen: 1,
		FieldOff:    0,
		FieldLen:    1,
	}
	setRuntimeElementIdentityConstraints(t, schema, []runtime.ICID{1})

	sess := NewSession(schema)
	sess.Reset()

	reader, err := xmlstream.NewReader(strings.NewReader(`<root xmlns="urn:test"/>`))
	if err != nil {
		t.Fatalf("xml reader: %v", err)
	}
	sess.io.reader = reader

	ev, err := sess.io.reader.NextResolved()
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
	if sess.Names.Scopes.Len() != 0 {
		t.Fatalf("namespace scope depth = %d, want 0", sess.Names.Scopes.Len())
	}
}

func TestIdentityStartNoConstraintsSkipsAttrMaterialization(t *testing.T) {
	fx := buildIdentityFixture(t)
	sess := NewSession(fx.schema)

	inputAttrs := make([]Start, 0, 64)
	for i := range 64 {
		local := fmt.Appendf(nil, "attr%d", i)
		inputAttrs = append(inputAttrs, Start{
			NSBytes: []byte("urn:other"),
			Local:   local,
			Value:   []byte("x"),
		})
	}

	if err := sess.identityStart(identityStartInput{
		Elem:  fx.elemItem,
		Type:  fx.typeSimple,
		Sym:   fx.symItem,
		NS:    fx.nsID,
		Attrs: inputAttrs,
	}); err != nil {
		t.Fatalf("identityStart: %v", err)
	}

	if sess.identity.icState.Active {
		t.Fatalf("identity state unexpectedly active")
	}
	if sess.identity.icState.Frames.Len() != 0 {
		t.Fatalf("identity frames len = %d, want 0", sess.identity.icState.Frames.Len())
	}
	if sess.identity.icState.Scopes.Len() != 0 {
		t.Fatalf("identity scopes len = %d, want 0", sess.identity.icState.Scopes.Len())
	}
	if sess.identity.icState.NextNodeID != 0 {
		t.Fatalf("identity nextNodeID = %d, want 0", sess.identity.icState.NextNodeID)
	}
	if len(sess.identity.identityAttrs.Names) != 0 {
		t.Fatalf("identity attr names materialized: %d", len(sess.identity.identityAttrs.Names))
	}
	if len(sess.identity.identityAttrs.Buckets) != 0 {
		t.Fatalf("identity attr buckets materialized: %d", len(sess.identity.identityAttrs.Buckets))
	}
}

func TestIdentityAttrSelectionAllocationsScaleLinearly(t *testing.T) {
	fx := buildIdentityFixture(t)
	schema := fx.schema
	pathAttrNSAny := runtime.PathID(len(schema.PathPrograms()))
	setRuntimePaths(t, schema, append(schema.PathPrograms(), runtime.PathProgram{
		Ops: []runtime.PathOp{{Op: runtime.OpAttrNSAny, NS: fx.empty}},
	}))
	configureRootUniqueAttrConstraint(t, schema, fx.elemRoot, fx.pathChild, pathAttrNSAny)

	buildAttrs := func(extra int) []Start {
		out := make([]Start, 0, extra+1)
		for i := range extra {
			local := fmt.Appendf(nil, "attr%d", i)
			out = append(out, Start{
				NSBytes:  []byte("urn:other"),
				Local:    local,
				Value:    []byte("x"),
				KeyKind:  runtime.VKString,
				KeyBytes: []byte("x"),
			})
		}
		out = append(out, Start{
			NS:       fx.empty,
			NSBytes:  nil,
			Local:    []byte("id"),
			Value:    []byte("match"),
			KeyKind:  runtime.VKString,
			KeyBytes: []byte("match"),
		})
		return out
	}

	smallAttrs := buildAttrs(8)
	largeAttrs := buildAttrs(80)
	sess := NewSession(schema)
	run := func(inputAttrs []Start) {
		sess.Reset()
		if err := sess.identityStart(identityStartInput{
			Elem: fx.elemRoot, Type: fx.typeComplex, Sym: fx.symRoot, NS: fx.nsID,
		}); err != nil {
			panic(err)
		}
		if err := sess.identityStart(identityStartInput{
			Elem: fx.elemItem, Type: fx.typeSimple, Sym: fx.symItem, NS: fx.nsID, Attrs: inputAttrs,
		}); err != nil {
			panic(err)
		}
		if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
			panic(err)
		}
		if err := sess.identity.icState.end(sess.rt, identityEndInput{}); err != nil {
			panic(err)
		}
		if pending := xsderrors.AppendIssues(nil, sess.identity.icState.DrainCommitted()); len(pending) != 0 {
			panic(pending[0])
		}
	}

	run(largeAttrs)
	run(smallAttrs)

	smallAllocs := testing.AllocsPerRun(100, func() { run(smallAttrs) })
	largeAllocs := testing.AllocsPerRun(50, func() { run(largeAttrs) })

	// keep scaling near-linear for attribute-heavy identity paths.
	if largeAllocs > smallAllocs*12 {
		t.Fatalf("identity attr selection allocations grew too fast: small=%.2f large=%.2f ratio=%.2f", smallAllocs, largeAllocs, largeAllocs/smallAllocs)
	}
}
