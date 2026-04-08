package compiler

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestMergeGlobalDeclsAppendsAtEnd(t *testing.T) {
	t.Parallel()

	existingName := model.QName{Namespace: "urn:test", Local: "existing"}
	newType := model.QName{Namespace: "urn:test", Local: "new-type"}
	newElement := model.QName{Namespace: "urn:test", Local: "new-element"}

	target := parser.NewSchema()
	target.GlobalDecls = []parser.GlobalDecl{{
		Kind: parser.GlobalDeclElement,
		Name: existingName,
	}}
	target.ElementDecls[existingName] = &model.ElementDecl{Name: existingName}
	target.TypeDefs[newType] = &model.SimpleType{}
	target.ElementDecls[newElement] = &model.ElementDecl{Name: newElement}

	source := parser.NewSchema()
	source.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclElement, Name: existingName},
		{Kind: parser.GlobalDeclType, Name: newType},
		{Kind: parser.GlobalDeclElement, Name: newElement},
	}

	ctx := mergeContext{
		targetGraph: &target.SchemaGraph,
		sourceGraph: &source.SchemaGraph,
		remapQName:  func(name model.QName) model.QName { return name },
	}
	ctx.recordInsertedGlobalDecl(parser.GlobalDeclType, newType)
	ctx.recordInsertedGlobalDecl(parser.GlobalDeclElement, newElement)
	ctx.mergeGlobalDecls(len(target.GlobalDecls))

	want := []parser.GlobalDecl{
		{Kind: parser.GlobalDeclElement, Name: existingName},
		{Kind: parser.GlobalDeclType, Name: newType},
		{Kind: parser.GlobalDeclElement, Name: newElement},
	}
	if len(target.GlobalDecls) != len(want) {
		t.Fatalf("GlobalDecls len = %d, want %d", len(target.GlobalDecls), len(want))
	}
	for i, decl := range want {
		if target.GlobalDecls[i] != decl {
			t.Fatalf("GlobalDecls[%d] = %+v, want %+v", i, target.GlobalDecls[i], decl)
		}
	}
}

func TestMergeGlobalDeclsInsertsInMiddle(t *testing.T) {
	t.Parallel()

	before := model.QName{Namespace: "urn:test", Local: "before"}
	insertType := model.QName{Namespace: "urn:test", Local: "insert-type"}
	insertElement := model.QName{Namespace: "urn:test", Local: "insert-element"}
	after := model.QName{Namespace: "urn:test", Local: "after"}

	target := parser.NewSchema()
	target.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclElement, Name: before},
		{Kind: parser.GlobalDeclElement, Name: after},
	}
	target.ElementDecls[before] = &model.ElementDecl{Name: before}
	target.ElementDecls[after] = &model.ElementDecl{Name: after}
	target.TypeDefs[insertType] = &model.SimpleType{}
	target.ElementDecls[insertElement] = &model.ElementDecl{Name: insertElement}

	source := parser.NewSchema()
	source.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclType, Name: insertType},
		{Kind: parser.GlobalDeclElement, Name: insertElement},
	}

	ctx := mergeContext{
		targetGraph: &target.SchemaGraph,
		sourceGraph: &source.SchemaGraph,
		remapQName:  func(name model.QName) model.QName { return name },
	}
	ctx.recordInsertedGlobalDecl(parser.GlobalDeclType, insertType)
	ctx.recordInsertedGlobalDecl(parser.GlobalDeclElement, insertElement)
	ctx.mergeGlobalDecls(1)

	want := []parser.GlobalDecl{
		{Kind: parser.GlobalDeclElement, Name: before},
		{Kind: parser.GlobalDeclType, Name: insertType},
		{Kind: parser.GlobalDeclElement, Name: insertElement},
		{Kind: parser.GlobalDeclElement, Name: after},
	}
	if len(target.GlobalDecls) != len(want) {
		t.Fatalf("GlobalDecls len = %d, want %d", len(target.GlobalDecls), len(want))
	}
	for i, decl := range want {
		if target.GlobalDecls[i] != decl {
			t.Fatalf("GlobalDecls[%d] = %+v, want %+v", i, target.GlobalDecls[i], decl)
		}
	}
}

func TestRecordInsertedGlobalDeclUsesInlineSetBeforePromotion(t *testing.T) {
	t.Parallel()

	var ctx mergeContext
	key := model.QName{Namespace: "urn:test", Local: "root"}
	ctx.recordInsertedGlobalDecl(parser.GlobalDeclElement, key)
	inserted := ctx.insertedGlobalDeclSet(parser.GlobalDeclElement)
	if inserted.large != nil {
		t.Fatal("insertedGlobalDecls[element] promoted eagerly, want inline set")
	}
	if !inserted.contains(key) {
		t.Fatalf("insertedGlobalDecls[element] missing %v", key)
	}
}

func TestNewMergeContextDefersInsertedGlobalDeclsAllocation(t *testing.T) {
	t.Parallel()

	target := parser.NewSchema()
	source := parser.NewSchema()
	source.GlobalDecls = []parser.GlobalDecl{{
		Kind: parser.GlobalDeclElement,
		Name: model.QName{Namespace: "urn:test", Local: "root"},
	}}

	ctx := newMergeContext(target, source, Include, KeepNamespace)
	inserted := ctx.insertedGlobalDeclSet(parser.GlobalDeclElement)
	if inserted.large != nil || inserted.len() != 0 {
		t.Fatal("insertedGlobalDecls[element] initialized eagerly, want zero-value set until first insert")
	}

	key := source.GlobalDecls[0].Name
	ctx.recordInsertedGlobalDecl(parser.GlobalDeclElement, key)
	if inserted.large != nil {
		t.Fatal("insertedGlobalDecls[element] promoted eagerly after first insert, want inline set")
	}
	if !inserted.contains(key) {
		t.Fatalf("insertedGlobalDecls[element] missing %v", key)
	}
}

func TestRecordInsertedGlobalDeclPromotesInlineSetWhenNeeded(t *testing.T) {
	t.Parallel()

	var ctx mergeContext
	for i := 0; i <= inlineInsertedQNameCap; i++ {
		ctx.recordInsertedGlobalDecl(parser.GlobalDeclElement, model.QName{
			Namespace: "urn:test",
			Local:     string(rune('a' + i)),
		})
	}

	inserted := ctx.insertedGlobalDeclSet(parser.GlobalDeclElement)
	if inserted.large == nil {
		t.Fatal("insertedGlobalDecls[element] did not promote after inline capacity")
	}
	for i := 0; i <= inlineInsertedQNameCap; i++ {
		key := model.QName{Namespace: "urn:test", Local: string(rune('a' + i))}
		if !inserted.contains(key) {
			t.Fatalf("insertedGlobalDecls[element] missing %v after promotion", key)
		}
	}
}

func TestExpectedInsertedGlobalDeclCountCachesInitialUpperBound(t *testing.T) {
	t.Parallel()

	first := model.QName{Namespace: "urn:test", Local: "first"}
	second := model.QName{Namespace: "urn:test", Local: "second"}

	target := parser.NewSchema()
	source := parser.NewSchema()
	source.ElementDecls[first] = &model.ElementDecl{Name: first}
	source.ElementDecls[second] = &model.ElementDecl{Name: second}

	ctx := mergeContext{
		targetGraph: &target.SchemaGraph,
		sourceGraph: &source.SchemaGraph,
		remapQName:  func(name model.QName) model.QName { return name },
	}

	if got := ctx.expectedInsertedGlobalDeclCount(parser.GlobalDeclElement); got != 2 {
		t.Fatalf("expectedInsertedGlobalDeclCount(element) = %d, want 2", got)
	}

	target.ElementDecls[first] = &model.ElementDecl{Name: first}
	if got := ctx.expectedInsertedGlobalDeclCount(parser.GlobalDeclElement); got != 2 {
		t.Fatalf("cached expectedInsertedGlobalDeclCount(element) = %d, want 2", got)
	}
}
