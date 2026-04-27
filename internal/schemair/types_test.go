package schemair

import "testing"

func TestTypeRefConstructorsEncodeKind(t *testing.T) {
	name := Name{Namespace: "urn:test", Local: "T"}

	tests := []struct {
		name string
		ref  TypeRef
		kind TypeRefKind
	}{
		{name: "none", ref: NoTypeRef(), kind: TypeRefNone},
		{name: "builtin", ref: BuiltinTypeRef(1, name), kind: TypeRefBuiltin},
		{name: "user", ref: UserTypeRef(2, name), kind: TypeRefUser},
		{name: "unresolved", ref: UnresolvedTypeRef(name), kind: TypeRefUnresolved},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.Kind(); got != tt.kind {
				t.Fatalf("kind = %d, want %d", got, tt.kind)
			}
		})
	}
}

func TestTypeRefZeroConstructorsNormalizeEmptyInputs(t *testing.T) {
	if ref := UserTypeRef(0, Name{}); !ref.IsZero() {
		t.Fatalf("zero user ref = %v, want zero", ref)
	}
	if ref := UnresolvedTypeRef(Name{}); !ref.IsZero() {
		t.Fatalf("empty unresolved ref = %v, want zero", ref)
	}
}

func TestValueConstraintConstructorsCloneContext(t *testing.T) {
	ctx := map[string]string{"p": "urn:one"}
	v := DefaultValueConstraint("x", ctx)
	ctx["p"] = "urn:two"

	if !v.IsPresent() || v.Kind() != ValueConstraintDefault || v.LexicalValue() != "x" {
		t.Fatalf("constraint = %+v, want default x", v)
	}
	got := v.NamespaceContext()
	if got["p"] != "urn:one" {
		t.Fatalf("stored context = %q, want original", got["p"])
	}
	got["p"] = "urn:three"
	if again := v.NamespaceContext(); again["p"] != "urn:one" {
		t.Fatalf("context accessor returned mutable storage: %q", again["p"])
	}
}

func TestNoValueConstraintIsAbsent(t *testing.T) {
	v := NoValueConstraint()
	if v.IsPresent() || v.Kind() != ValueConstraintNone || v.LexicalValue() != "" || v.NamespaceContext() != nil {
		t.Fatalf("none = %+v, want absent", v)
	}
}

func TestParticleConstructorsEncodeVariants(t *testing.T) {
	minOccurs := Occurs{Value: 1}
	maxOccurs := Occurs{Value: 2}

	elem := ElementParticle(1, 10, minOccurs, maxOccurs, true)
	if elem.ParticleKind() != ParticleElement {
		t.Fatalf("element kind = %v", elem.ParticleKind())
	}
	if id, ok := elem.ElementID(); !ok || id != 10 {
		t.Fatalf("element id = %d, %v", id, ok)
	}
	if _, ok := elem.WildcardID(); ok || elem.ChildParticles() != nil || !elem.AllowsSubstitutionGroup() {
		t.Fatalf("element variant leaked unrelated state: %+v", elem)
	}

	wildcard := WildcardParticle(2, 20, minOccurs, maxOccurs)
	if wildcard.ParticleKind() != ParticleWildcard {
		t.Fatalf("wildcard kind = %v", wildcard.ParticleKind())
	}
	if id, ok := wildcard.WildcardID(); !ok || id != 20 {
		t.Fatalf("wildcard id = %d, %v", id, ok)
	}
	if _, ok := wildcard.ElementID(); ok || wildcard.ChildParticles() != nil || wildcard.AllowsSubstitutionGroup() {
		t.Fatalf("wildcard variant leaked unrelated state: %+v", wildcard)
	}

	children := []ParticleID{3, 4}
	group := GroupParticle(3, GroupChoice, children, minOccurs, maxOccurs)
	children[0] = 99
	if kind, ok := group.GroupKindValue(); !ok || kind != GroupChoice {
		t.Fatalf("group kind = %d, %v", kind, ok)
	}
	gotChildren := group.ChildParticles()
	if len(gotChildren) != 2 || gotChildren[0] != 3 || gotChildren[1] != 4 {
		t.Fatalf("children = %v, want cloned [3 4]", gotChildren)
	}
	gotChildren[0] = 99
	if again := group.ChildParticles(); again[0] != 3 {
		t.Fatalf("child accessor returned mutable storage: %v", again)
	}
	if _, ok := group.ElementID(); ok {
		t.Fatalf("group exposes element id")
	}
	if _, ok := group.WildcardID(); ok {
		t.Fatalf("group exposes wildcard id")
	}
}

func TestParticleConstructorsNormalizeMissingVariantIDs(t *testing.T) {
	if p := ElementParticle(1, 0, Occurs{}, Occurs{}, false); p.ParticleKind() != ParticleNone {
		t.Fatalf("zero element particle = %+v, want none", p)
	}
	if p := WildcardParticle(1, 0, Occurs{}, Occurs{}); p.ParticleKind() != ParticleNone {
		t.Fatalf("zero wildcard particle = %+v, want none", p)
	}
}
