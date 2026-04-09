package compiler

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestOrderedDeclNamesUsesGlobalDeclOrder(t *testing.T) {
	t.Parallel()

	first := model.QName{Namespace: "urn:test", Local: "first"}
	second := model.QName{Namespace: "urn:test", Local: "second"}
	schema := parser.NewSchema()
	schema.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclType, Name: first},
		{Kind: parser.GlobalDeclElement, Name: second},
		{Kind: parser.GlobalDeclElement, Name: first},
	}
	schema.ElementDecls = map[model.QName]*model.ElementDecl{
		first:  {Name: first},
		second: {Name: second},
	}

	got := orderedDeclNames(&schema.SchemaGraph, parser.GlobalDeclElement, schema.ElementDecls)
	want := []model.QName{second, first}
	if len(got) != len(want) {
		t.Fatalf("orderedDeclNames() len = %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("orderedDeclNames()[%d] = %v, want %v", i, got[i], name)
		}
	}
}

func TestOrderedDeclNamesSortsFallbackDecls(t *testing.T) {
	t.Parallel()

	ordered := model.QName{Namespace: "urn:test", Local: "ordered"}
	extraA := model.QName{Namespace: "urn:test", Local: "alpha"}
	extraB := model.QName{Namespace: "urn:test", Local: "beta"}
	schema := parser.NewSchema()
	schema.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclAttribute, Name: ordered},
	}
	schema.AttributeDecls = map[model.QName]*model.AttributeDecl{
		ordered: {Name: ordered},
		extraB:  {Name: extraB},
		extraA:  {Name: extraA},
	}

	got := orderedDeclNames(&schema.SchemaGraph, parser.GlobalDeclAttribute, schema.AttributeDecls)
	want := []model.QName{ordered, extraA, extraB}
	if len(got) != len(want) {
		t.Fatalf("orderedDeclNames() len = %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("orderedDeclNames()[%d] = %v, want %v", i, got[i], name)
		}
	}
}

func TestOrderedDeclNamesDeduplicatesRepeatedGlobalDeclEntries(t *testing.T) {
	t.Parallel()

	first := model.QName{Namespace: "urn:test", Local: "first"}
	second := model.QName{Namespace: "urn:test", Local: "second"}
	schema := parser.NewSchema()
	schema.GlobalDecls = []parser.GlobalDecl{
		{Kind: parser.GlobalDeclElement, Name: second},
		{Kind: parser.GlobalDeclElement, Name: first},
		{Kind: parser.GlobalDeclElement, Name: first},
	}
	schema.ElementDecls = map[model.QName]*model.ElementDecl{
		first:  {Name: first},
		second: {Name: second},
	}

	got := orderedDeclNames(&schema.SchemaGraph, parser.GlobalDeclElement, schema.ElementDecls)
	want := []model.QName{second, first}
	if len(got) != len(want) {
		t.Fatalf("orderedDeclNames() len = %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("orderedDeclNames()[%d] = %v, want %v", i, got[i], name)
		}
	}
}
