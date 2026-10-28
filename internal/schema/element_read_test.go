package schema

import (
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestElementReadTableOwnsCanonicalFacts(t *testing.T) {
	fixed := &ValueConstraint{Lexical: "01", Canonical: "1", Value: builtinValueForTest(t, "integer", "01")}
	def := &ValueConstraint{Lexical: "default", Canonical: "default", Value: builtinValueForTest(t, "string", "default")}
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
	table := newElementReadTable(decls)
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

func TestElementReadTableOwnsConstraintNamespaceContexts(t *testing.T) {
	t.Parallel()
	fixed := &ValueConstraint{
		Lexical: "p:fixed", Canonical: "p:fixed", Value: builtinValueForTest(t, "string", "p:fixed"),
		qnameContext: &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:fixed", bound: true}}},
	}
	def := &ValueConstraint{
		Lexical: "p:default", Canonical: "p:default", Value: builtinValueForTest(t, "string", "p:default"),
		qnameContext: &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:default", bound: true}}},
	}
	table := newElementReadTable([]ElementDecl{{Type: SimpleRef(3), Fixed: fixed}, {Type: SimpleRef(4), Default: def}})
	fixed.qnameContext.bindings[0].namespace = "urn:changed"
	def.qnameContext.bindings[0].bound = false
	for range 2 {
		fixedConstraints, declared, valid := table.valueConstraints(0)
		if !declared || !valid {
			t.Fatal("fixed declaration became unavailable after input mutation")
		}
		fixedRead, present := fixedConstraints.FixedValue()
		if !present {
			t.Fatal("fixed value became absent after input mutation")
		}
		if _, defaultPresent := fixedConstraints.DefaultValueConstraint(); defaultPresent {
			t.Fatal("fixed declaration exposes a default")
		}
		if name, ok := fixedRead.ResolveQName("p:fixed"); !ok || name != (valuepkg.ExpandedName{Namespace: "urn:fixed", Local: "fixed"}) {
			t.Fatalf("published fixed QName = %v, %v; want {urn:fixed}fixed, true", name, ok)
		}
		defaultConstraints, declared, valid := table.valueConstraints(1)
		if !declared || !valid {
			t.Fatal("default declaration became unavailable after input mutation")
		}
		defaultRead, present := defaultConstraints.DefaultValueConstraint()
		if !present {
			t.Fatal("default value became absent after input mutation")
		}
		if _, fixedPresent := defaultConstraints.FixedValue(); fixedPresent {
			t.Fatal("default declaration exposes a fixed value")
		}
		if name, ok := defaultRead.ResolveQName("p:default"); !ok || name != (valuepkg.ExpandedName{Namespace: "urn:default", Local: "default"}) {
			t.Fatalf("published default QName = %v, %v; want {urn:default}default, true", name, ok)
		}
	}
}
