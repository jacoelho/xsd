package analysis_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestAssignIDsRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	name := model.QName{Namespace: "urn:test", Local: "MissingType"}
	sch.TypeDefs[name] = model.NewPlaceholderSimpleType(name)
	sch.GlobalDecls = append(sch.GlobalDecls, parser.GlobalDecl{
		Kind: parser.GlobalDeclType,
		Name: name,
	})
	if _, err := analysis.AssignIDs(sch); err == nil {
		t.Fatalf("expected AssignIDs to reject placeholders")
	}
}

func TestRequireResolvedRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	name := model.QName{Namespace: "urn:test", Local: "MissingType"}
	sch.TypeDefs[name] = model.NewPlaceholderSimpleType(name)
	sch.GlobalDecls = append(sch.GlobalDecls, parser.GlobalDecl{
		Kind: parser.GlobalDeclType,
		Name: name,
	})
	if err := analysis.RequireResolved(sch); err == nil {
		t.Fatalf("expected RequireResolved to reject placeholders")
	}
}
