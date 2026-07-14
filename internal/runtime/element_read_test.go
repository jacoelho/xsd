package runtime

import "testing"

func TestElementReadTableOwnsCanonicalFacts(t *testing.T) {
	fixed := &ValueConstraint{Lexical: "01", Canonical: "1", Value: SimpleValue{Canonical: "1", Type: 3}}
	def := &ValueConstraint{Lexical: "default", Canonical: "default", Value: SimpleValue{Canonical: "default", Type: 4}}
	decls := []ElementDecl{
		{
			Name:     QName{Namespace: 1, Local: 2},
			Type:     SimpleRef(3),
			Block:    DerivationRestriction,
			Identity: []IdentityConstraintID{5, 6},
			Fixed:    fixed,
			Abstract: true,
			Nillable: true,
		},
		{
			Name:    QName{Namespace: 7, Local: 8},
			Type:    SimpleRef(4),
			Default: def,
		},
	}
	table := newElementReadTable(decls, nil)
	if err := validateElementReadTableProjection(table, decls, nil); err != nil {
		t.Fatalf("validateElementReadTableProjection() error = %v", err)
	}
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

func TestElementReadTableAuditRejectsCorruption(t *testing.T) {
	decls := []ElementDecl{{
		Name:     QName{Local: 1},
		Type:     SimpleRef(2),
		Identity: []IdentityConstraintID{3},
		Fixed:    &ValueConstraint{Canonical: "x"},
	}}
	tests := []struct {
		name   string
		mutate func(*elementReadTable)
	}{
		{"name count", func(table *elementReadTable) { table.names = nil }},
		{"metadata", func(table *elementReadTable) { table.meta[0].typ = SimpleRef(9) }},
		{"flags", func(table *elementReadTable) { table.meta[0].flags |= 1 << 7 }},
		{"identity offset", func(table *elementReadTable) { table.meta[0].identityStart = 1 }},
		{"identity value", func(table *elementReadTable) { table.identities[0] = 4 }},
		{"constraint index", func(table *elementReadTable) { table.meta[0].constraint = 1 }},
		{"constraint value", func(table *elementReadTable) { table.constraints[0].value.canonical = "y" }},
		{"extra identity", func(table *elementReadTable) { table.identities = append(table.identities, 4) }},
		{"extra constraint", func(table *elementReadTable) { table.constraints = append(table.constraints, elementConstraintRead{}) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table := newElementReadTable(decls, nil)
			test.mutate(&table)
			if err := validateElementReadTableProjection(table, decls, nil); err == nil {
				t.Fatal("validateElementReadTableProjection() accepted corruption")
			}
		})
	}
}

func TestElementReadTablePrecomputesEffectiveTypeBlock(t *testing.T) {
	decls := []ElementDecl{{Type: ComplexRef(0), Block: DerivationRestriction}}
	complexTypes := []ComplexType{{Block: DerivationExtension}}
	table := newElementReadTable(decls, complexTypes)
	start, ok := table.start(0)
	if !ok || start.Block != DerivationRestriction|DerivationExtension {
		t.Fatalf("start(0).Block = %08b, %v", start.Block, ok)
	}
	if err := validateElementReadTableProjection(table, decls, complexTypes); err != nil {
		t.Fatalf("validateElementReadTableProjection() error = %v", err)
	}
}
