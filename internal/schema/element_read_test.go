package schema

import (
	"testing"
)

func allReachable(n int) []bool {
	reachable := make([]bool, n)
	for i := range reachable {
		reachable[i] = true
	}
	return reachable
}

func TestElementReadTableOwnsCanonicalFacts(t *testing.T) {
	fixed := &valueConstraint{Lexical: "01", Canonical: "1", Value: builtinValueForTest(t, "integer", "01"), Fixed: true}
	def := &valueConstraint{Lexical: "default", Canonical: "default", Value: builtinValueForTest(t, "string", "default")}
	decls := []elementDecl{
		{
			Name:            QName{Namespace: 1, Local: 2},
			Type:            SimpleRef(3),
			Block:           DerivationRestriction,
			Identity:        []IdentityConstraintID{5, 6},
			ValueConstraint: fixed,
			Abstract:        true,
			Nillable:        true,
		},
		{
			Name:            QName{Namespace: 7, Local: 8},
			Type:            SimpleRef(4),
			ValueConstraint: def,
		},
	}
	table := newElementReadTable(decls, allReachable(len(decls)))
	if name, ok := table.name(0); !ok || name != decls[0].Name {
		t.Fatalf("name(0) = %v, %v", name, ok)
	}
	start, ok := table.start(0)
	if !ok || start.Type != decls[0].Type || start.Block != decls[0].Block ||
		!start.Abstract || !start.Nillable || !start.Fixed || start.Default {
		t.Fatalf("start(0) = %+v, %v", start, ok)
	}
	start, ok = table.start(1)
	if !ok || start.Fixed || !start.Default {
		t.Fatalf("start(1) = %+v, %v", start, ok)
	}
	ids, ok := table.identityConstraints(0)
	if !ok || ids.Len() != 2 {
		t.Fatalf("identityConstraints(0).Len() = %d, %v", ids.Len(), ok)
	}
	constraints, declared, valid := table.valueConstraints(0)
	if !valid || !declared || constraints.OwnerType() != decls[0].Type {
		t.Fatalf("valueConstraints(0) = %+v, %v, %v", constraints, declared, valid)
	}
	if value, present := constraints.FixedValue(); !present || value.CanonicalText() != "1" {
		t.Fatalf("FixedValue() = %+v, %v", value, present)
	}

	decls[0].Name = QName{}
	decls[0].Identity[0] = 99
	fixed.Canonical = "poison"
	if name, _ := table.name(0); name != (QName{Namespace: 1, Local: 2}) {
		t.Fatalf("retained declaration mutation changed name to %v", name)
	}
	ids, _ = table.identityConstraints(0)
	if id, _ := ids.At(0); id != 5 {
		t.Fatalf("retained declaration mutation changed identity to %d", id)
	}
	updatedConstraints, updatedDeclared, updatedValid := table.valueConstraints(0)
	if !updatedDeclared || !updatedValid {
		t.Fatal("valueConstraints(0) became invalid after source mutation")
	}
	constraints = updatedConstraints
	if value, _ := constraints.FixedValue(); value.CanonicalText() != "1" {
		t.Fatalf("retained constraint mutation changed canonical value to %q", value.CanonicalText())
	}
}
