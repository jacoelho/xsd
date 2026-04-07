package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

type mockLexicalFacet struct {
	name    string
	lexical string
}

func (f mockLexicalFacet) Name() string { return f.name }

func (f mockLexicalFacet) Validate(model.TypedValue, model.Type) error { return nil }

func (f mockLexicalFacet) GetLexical() string { return f.lexical }

func TestValidateStructureRejectsTypedNilTypeDef(t *testing.T) {
	t.Parallel()

	schema := parser.NewSchema()
	baseQName := model.QName{Namespace: "urn:test", Local: "Base"}
	derivedQName := model.QName{Namespace: "urn:test", Local: "Derived"}

	var missing *model.SimpleType
	schema.TypeDefs[baseQName] = missing
	schema.TypeDefs[derivedQName] = &model.SimpleType{
		QName: derivedQName,
		Restriction: &model.Restriction{
			Base: baseQName,
			Facets: []any{
				mockLexicalFacet{name: "minInclusive", lexical: "1"},
			},
		},
	}

	errs := ValidateStructure(schema)
	if len(errs) == 0 {
		t.Fatal("ValidateStructure() returned no errors")
	}
	if got := errs[0].Error(); !strings.Contains(got, "type definition is nil") {
		t.Fatalf("ValidateStructure() first error = %q, want substring %q", got, "type definition is nil")
	}
}
