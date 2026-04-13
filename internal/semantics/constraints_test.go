package semantics

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

type invalidFacet struct{}

func (invalidFacet) Name() string { return "invalidFacet" }
func (invalidFacet) Validate(model.TypedValue, model.Type) error {
	return nil
}

func TestValidateSchemaConstraintsRejectsInvalidFacetName(t *testing.T) {
	err := ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: []model.Facet{invalidFacet{}},
			BaseType:  model.GetBuiltin(model.TypeNameString),
			BaseQName: model.QName{Namespace: model.XSDNamespace, Local: string(model.TypeNameString)},
		},
	)
	if err == nil {
		t.Fatal("expected invalid facet name error")
	}
}

func TestValidateSchemaConstraintsDelegatesRangeChecks(t *testing.T) {
	base := model.GetBuiltin(model.TypeNameInt)
	if base == nil {
		t.Fatal("builtin int is nil")
	}
	minFacet, err := model.NewMinInclusive("1", base)
	if err != nil {
		t.Fatalf("minInclusive: %v", err)
	}
	maxFacet, err := model.NewMaxInclusive("0", base)
	if err != nil {
		t.Fatalf("maxInclusive: %v", err)
	}

	err = ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: []model.Facet{minFacet, maxFacet},
			BaseType:  base,
			BaseQName: base.Name(),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "minInclusive (1) must be <= maxInclusive (0)") {
		t.Fatalf("error = %v, want range consistency failure", err)
	}
}

func TestValidateSchemaConstraintsValidatesEnumerationValues(t *testing.T) {
	enum := model.NewEnumeration([]string{"a", "b"})
	enum.SetValueContexts([]map[string]string{
		{"p": "urn:a"},
		{"p": "urn:b"},
	})

	err := validateEnumerationValues([]model.Facet{enum}, model.GetBuiltin(model.TypeNameString), func(value string, _ model.Type, context map[string]string) error {
		switch value {
		case "a":
			if context["p"] != "urn:a" {
				t.Fatalf("context for a = %v", context)
			}
		case "b":
			if context["p"] != "urn:b" {
				t.Fatalf("context for b = %v", context)
			}
		default:
			t.Fatalf("unexpected enumeration value %q", value)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("validateEnumerationValues() error = %v", err)
	}
}

func TestValidateSchemaConstraintsDefersEnumerationForUnresolvedBase(t *testing.T) {
	enum := model.NewEnumeration([]string{"a"})
	base := &model.SimpleType{
		QName: model.QName{Namespace: "urn:test", Local: "MyType"},
		Restriction: &model.Restriction{
			Base: model.QName{Namespace: "urn:external", Local: "BaseType"},
		},
	}

	calls := 0
	err := validateEnumerationValues([]model.Facet{enum}, base, func(string, model.Type, map[string]string) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("validateEnumerationValues() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("enumeration callback calls = %d, want 0", calls)
	}
}
