package runtimeids

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semanticcheck"
	"github.com/jacoelho/xsd/internal/semanticresolve"
)

func TestBuildDeterministicIDs(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test" xmlns:tns="urn:test">
  <xs:simpleType name="A"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="root" type="tns:A"/>
  <xs:attribute name="att" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("ParseString() error = %v", err)
	}
	if err := resolveAndValidateOwned(sch); err != nil {
		t.Fatalf("resolveAndValidateOwned() error = %v", err)
	}
	registry, err := analysis.AssignIDs(sch)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	plan, err := Build(registry)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(plan.BuiltinTypeIDs) == 0 {
		t.Fatal("Build() builtin map is empty")
	}
	if got := plan.BuiltinTypeIDs[model.TypeNameAnyType]; got == 0 {
		t.Fatal("Build() anyType id missing")
	}
	if len(plan.TypeIDs) != len(registry.TypeOrder) {
		t.Fatalf("Build() type ids len = %d, want %d", len(plan.TypeIDs), len(registry.TypeOrder))
	}
	if len(plan.ElementIDs) != len(registry.ElementOrder) {
		t.Fatalf("Build() element ids len = %d, want %d", len(plan.ElementIDs), len(registry.ElementOrder))
	}
	if len(plan.AttributeIDs) != len(registry.AttributeOrder) {
		t.Fatalf("Build() attribute ids len = %d, want %d", len(plan.AttributeIDs), len(registry.AttributeOrder))
	}
}

func TestBuiltinTypeNamesCopies(t *testing.T) {
	first := BuiltinTypeNames()
	second := BuiltinTypeNames()
	if len(first) == 0 {
		t.Fatal("BuiltinTypeNames() empty")
	}
	if !slices.Equal(first, second) {
		t.Fatalf("BuiltinTypeNames() mismatch: %v vs %v", first, second)
	}
	first[0] = model.TypeNameString
	third := BuiltinTypeNames()
	if third[0] != model.TypeNameAnyType {
		t.Fatalf("BuiltinTypeNames() returned shared backing slice")
	}
}

func resolveAndValidateOwned(sch *parser.Schema) error {
	if sch == nil {
		return fmt.Errorf("schema is nil")
	}
	if err := semanticresolve.ResolveGroupReferences(sch); err != nil {
		return fmt.Errorf("resolve group references: %w", err)
	}
	structureErrs := semanticcheck.ValidateStructure(sch)
	if len(structureErrs) > 0 {
		return formatValidationErrors(structureErrs)
	}
	if err := semanticresolve.NewResolver(sch).Resolve(); err != nil {
		return fmt.Errorf("resolve type references: %w", err)
	}
	refErrs := semanticresolve.ValidateReferences(sch)
	if len(refErrs) > 0 {
		return formatValidationErrors(refErrs)
	}
	deferredRangeErrs := semanticcheck.ValidateDeferredRangeFacetValues(sch)
	if len(deferredRangeErrs) > 0 {
		return formatValidationErrors(deferredRangeErrs)
	}
	if parser.HasPlaceholders(sch) {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	return nil
}

func formatValidationErrors(validationErrs []error) error {
	if len(validationErrs) == 0 {
		return nil
	}

	errs := validationErrs
	if len(validationErrs) > 1 {
		errs = slices.Clone(validationErrs)
		slices.SortStableFunc(errs, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
	}

	var msg strings.Builder
	msg.WriteString("schema validation failed:")
	for _, err := range errs {
		msg.WriteString("\n  - ")
		msg.WriteString(err.Error())
	}
	return errors.New(msg.String())
}
