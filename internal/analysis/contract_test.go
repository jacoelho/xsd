package analysis

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestAssignIDsRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	name := types.QName{Namespace: "urn:test", Local: "MissingType"}
	sch.TypeDefs[name] = types.NewPlaceholderSimpleType(name)
	sch.GlobalDecls = append(sch.GlobalDecls, parser.GlobalDecl{
		Kind: parser.GlobalDeclType,
		Name: name,
	})
	if _, err := AssignIDs(sch); err == nil {
		t.Fatalf("expected AssignIDs to reject placeholders")
	}
}

func TestResolveReferencesRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	name := types.QName{Namespace: "urn:test", Local: "MissingType"}
	sch.TypeDefs[name] = types.NewPlaceholderSimpleType(name)
	sch.GlobalDecls = append(sch.GlobalDecls, parser.GlobalDecl{
		Kind: parser.GlobalDeclType,
		Name: name,
	})
	if _, err := ResolveReferences(sch, newRegistry()); err == nil {
		t.Fatalf("expected ResolveReferences to reject placeholders")
	}
}

func TestRequireResolvedRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	name := types.QName{Namespace: "urn:test", Local: "MissingType"}
	sch.TypeDefs[name] = types.NewPlaceholderSimpleType(name)
	sch.GlobalDecls = append(sch.GlobalDecls, parser.GlobalDecl{
		Kind: parser.GlobalDeclType,
		Name: name,
	})
	if err := RequireResolved(sch); err == nil {
		t.Fatalf("expected RequireResolved to reject placeholders")
	}
}
