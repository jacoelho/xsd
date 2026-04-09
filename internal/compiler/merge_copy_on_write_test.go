package compiler

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestMergeElementDeclsCopiesSharedMapsOnInsert(t *testing.T) {
	t.Parallel()

	target := parser.NewSchema()
	target.Location = "target.xsd"
	target.TargetNamespace = "urn:test"

	existing := model.QName{Namespace: target.TargetNamespace, Local: "existing"}
	target.ElementDecls[existing] = &model.ElementDecl{Name: existing}
	target.ElementOrigins[existing] = target.Location
	target.GlobalDecls = []parser.GlobalDecl{{
		Kind: parser.GlobalDeclElement,
		Name: existing,
	}}

	source := parser.NewSchema()
	source.Location = "source.xsd"
	source.TargetNamespace = target.TargetNamespace

	inserted := model.QName{Namespace: source.TargetNamespace, Local: "inserted"}
	source.ElementDecls[inserted] = &model.ElementDecl{Name: inserted}
	source.ElementOrigins[inserted] = source.Location
	source.GlobalDecls = []parser.GlobalDecl{{
		Kind: parser.GlobalDeclElement,
		Name: inserted,
	}}

	staging := parser.CloneSchemaForMerge(target)
	ctx := newMergeContext(staging, source, Include, KeepNamespace)
	if err := ctx.mergeElementDecls(); err != nil {
		t.Fatalf("mergeElementDecls() error = %v", err)
	}

	if _, ok := target.ElementDecls[inserted]; ok {
		t.Fatal("target ElementDecls unexpectedly mutated through shared merge staging map")
	}
	if _, ok := target.ElementOrigins[inserted]; ok {
		t.Fatal("target ElementOrigins unexpectedly mutated through shared merge staging map")
	}
	if _, ok := staging.ElementDecls[inserted]; !ok {
		t.Fatal("staging ElementDecls missing inserted element")
	}
	if got := staging.ElementOrigins[inserted]; got != source.Location {
		t.Fatalf("staging ElementOrigins[%v] = %q, want %q", inserted, got, source.Location)
	}
}
