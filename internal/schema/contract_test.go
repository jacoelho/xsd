package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestAssignIDsRequiresResolved(t *testing.T) {
	sch := parser.NewSchema()
	sch.Phase = parser.PhaseParsed
	if _, err := AssignIDs(sch); err == nil {
		t.Fatalf("expected AssignIDs to reject non-resolved schema")
	}
}

func TestAssignIDsRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	sch.Phase = parser.PhaseResolved
	sch.HasPlaceholders = true
	if _, err := AssignIDs(sch); err == nil {
		t.Fatalf("expected AssignIDs to reject placeholders")
	}
}

func TestResolveReferencesRequiresResolved(t *testing.T) {
	sch := parser.NewSchema()
	sch.Phase = parser.PhaseParsed
	if _, err := ResolveReferences(sch, newRegistry()); err == nil {
		t.Fatalf("expected ResolveReferences to reject non-resolved schema")
	}
}

func TestResolveReferencesRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	sch.Phase = parser.PhaseResolved
	sch.HasPlaceholders = true
	if _, err := ResolveReferences(sch, newRegistry()); err == nil {
		t.Fatalf("expected ResolveReferences to reject placeholders")
	}
}

func TestMarkResolvedRejectsPlaceholders(t *testing.T) {
	sch := parser.NewSchema()
	sch.Phase = parser.PhaseSemantic
	sch.HasPlaceholders = true
	if err := MarkResolved(sch); err == nil {
		t.Fatalf("expected MarkResolved to reject placeholders")
	}
}
