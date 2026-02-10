package schemafacet

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
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
			BaseType:  builtins.Get(model.TypeNameString),
			BaseQName: model.QName{Namespace: model.XSDNamespace, Local: string(model.TypeNameString)},
		},
		SchemaConstraintCallbacks{},
	)
	if err == nil {
		t.Fatal("expected invalid facet name error")
	}
}

func TestValidateSchemaConstraintsDelegatesRangeChecks(t *testing.T) {
	base := builtins.Get(model.TypeNameInt)
	if base == nil {
		t.Fatal("builtin int is nil")
	}
	minFacet, err := facetvalue.NewMinInclusive("1", base)
	if err != nil {
		t.Fatalf("minInclusive: %v", err)
	}
	maxFacet, err := facetvalue.NewMaxInclusive("0", base)
	if err != nil {
		t.Fatalf("maxInclusive: %v", err)
	}

	wantErr := errors.New("range consistency")
	rangeCalled := false
	err = ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: []model.Facet{minFacet, maxFacet},
			BaseType:  base,
			BaseQName: base.Name(),
		},
		SchemaConstraintCallbacks{
			ValidateRangeConsistency: func(minExclusive, maxExclusive, minInclusive, maxInclusive *string, _ model.Type, _ model.QName) error {
				rangeCalled = true
				if minInclusive == nil || *minInclusive != "1" {
					t.Fatalf("minInclusive = %v, want 1", minInclusive)
				}
				if maxInclusive == nil || *maxInclusive != "0" {
					t.Fatalf("maxInclusive = %v, want 0", maxInclusive)
				}
				return wantErr
			},
		},
	)
	if !rangeCalled {
		t.Fatal("expected range callback to run")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
}

func TestValidateSchemaConstraintsValidatesEnumerationValues(t *testing.T) {
	enum := model.NewEnumeration([]string{"a", "b"})
	enum.SetValueContexts([]map[string]string{
		{"p": "urn:a"},
		{"p": "urn:b"},
	})

	seen := make([]string, 0, 2)
	contexts := make([]map[string]string, 0, 2)
	err := ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: []model.Facet{enum},
			BaseType:  builtins.Get(model.TypeNameString),
			BaseQName: model.QName{Namespace: model.XSDNamespace, Local: string(model.TypeNameString)},
		},
		SchemaConstraintCallbacks{
			ValidateEnumerationValue: func(value string, _ model.Type, context map[string]string) error {
				seen = append(seen, value)
				contexts = append(contexts, context)
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateSchemaConstraints() error = %v", err)
	}
	if len(seen) != 2 || seen[0] != "a" || seen[1] != "b" {
		t.Fatalf("enumeration values = %v, want [a b]", seen)
	}
	if len(contexts) != 2 || contexts[0]["p"] != "urn:a" || contexts[1]["p"] != "urn:b" {
		t.Fatalf("enumeration contexts = %v", contexts)
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
	err := ValidateSchemaConstraints(
		SchemaConstraintInput{
			FacetList: []model.Facet{enum},
			BaseType:  base,
			BaseQName: base.Name(),
		},
		SchemaConstraintCallbacks{
			ValidateEnumerationValue: func(string, model.Type, map[string]string) error {
				calls++
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateSchemaConstraints() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("enumeration callback calls = %d, want 0", calls)
	}
}
