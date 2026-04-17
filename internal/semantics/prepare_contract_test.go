package semantics_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func TestResolveReferencesRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	name := model.QName{Namespace: "urn:test", Local: "MissingType"}
	sch.TypeDefs[name] = model.NewPlaceholderSimpleType(name)
	sch.GlobalDecls = append(sch.GlobalDecls, parser.GlobalDecl{
		Kind: parser.GlobalDeclType,
		Name: name,
	})
	if _, err := semantics.ResolveReferences(sch, emptyRegistry()); err == nil {
		t.Fatalf("expected ResolveReferences to reject placeholders")
	}
}

func emptyRegistry() *analysis.Registry {
	reg, err := analysis.AssignIDs(parser.NewSchema())
	if err == nil {
		return reg
	}
	return &analysis.Registry{}
}
