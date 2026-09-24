package schema

import (
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestValueConstraintQNameContextIsRepeatableAndPrefixBounded(t *testing.T) {
	lookup := func(prefix string) (string, bool) {
		switch prefix {
		case "":
			return "urn:default", true
		case "p":
			return "urn:p", true
		default:
			return "", false
		}
	}
	context := newValueConstraintQNameContext(" \tp:x q:y x p:z\n", lookup)

	for _, test := range []struct {
		lexical   string
		namespace string
		local     string
		ok        bool
	}{
		{lexical: "p:x", namespace: "urn:p", local: "x", ok: true},
		{lexical: "p:z", namespace: "urn:p", local: "z", ok: true},
		{lexical: "x", namespace: "urn:default", local: "x", ok: true},
		{lexical: "q:y", ok: false},
		{lexical: "bad::name", ok: false},
	} {
		got, ok := context.ResolveQName(test.lexical)
		if ok != test.ok || ok && (got.Namespace != test.namespace || got.Local != test.local) {
			t.Fatalf("ResolveQName(%q) = %+v, %v; want {%q %q}, %v", test.lexical, got, ok, test.namespace, test.local, test.ok)
		}
	}
	if got, ok := context.ResolveQName("p:x"); !ok || got.Namespace != "urn:p" {
		t.Fatalf("repeated ResolveQName(p:x) = %+v, %v", got, ok)
	}

	if err := validateValueConstraintQNameContext(
		" \tp:x q:y x p:z\n",
		context,
		[]ResolvedValueName{{Lexical: "p:x", NS: "urn:p", Local: "x"}},
	); err != nil {
		t.Fatalf("validateValueConstraintQNameContext() = %v", err)
	}
}

func TestValueConstraintQNameContextPreservesUnresolvedPrefixes(t *testing.T) {
	context := newValueConstraintQNameContext("p:x", func(string) (string, bool) { return "", false })
	if _, ok := context.ResolveQName("p:x"); ok {
		t.Fatal("ResolveQName(p:x) resolved an unbound schema prefix")
	}
	if err := validateValueConstraintQNameContext("p:x", context, nil); err != nil {
		t.Fatalf("validateValueConstraintQNameContext() = %v", err)
	}
	context.bindings[0].namespace = "urn:poison"
	if err := validateValueConstraintQNameContext("p:x", context, nil); err == nil {
		t.Fatal("validateValueConstraintQNameContext() accepted inconsistent unresolved binding")
	}
	if err := validateValueConstraintQNameContext("p:x", nil, nil); err == nil {
		t.Fatal("validateValueConstraintQNameContext() accepted missing context")
	}
}

func TestValueConstraintReadCopiesQNameContext(t *testing.T) {
	value := testQNameValue(t, "urn:p")
	vc := &ValueConstraint{
		Lexical:      "p:item",
		Canonical:    value.CanonicalText(),
		Value:        value,
		qnameContext: newValueConstraintQNameContext("p:item", func(string) (string, bool) { return "urn:p", true }),
	}
	read, ok := newValueConstraintReadFromConstraint(vc)
	if !ok {
		t.Fatal("newValueConstraintReadFromConstraint() reported absent constraint")
	}
	if got, ok := read.ResolveQName("p:item"); !ok || got.Namespace != "urn:p" || got.Local != "item" {
		t.Fatalf("read.ResolveQName(p:item) = %+v, %v", got, ok)
	}

	vc.qnameContext.bindings[0].namespace = "urn:poison"
	if got, ok := read.ResolveQName("p:item"); !ok || got.Namespace != "urn:p" {
		t.Fatalf("published read aliased compiler QName context: %+v, %v", got, ok)
	}
}

func TestValueConstraintQNameContextAuditMatchesResolvedProof(t *testing.T) {
	context := newValueConstraintQNameContext("p:item", func(string) (string, bool) { return "urn:p", true })
	if err := validateValueConstraintQNameContext(
		"p:item",
		context,
		[]ResolvedValueName{{Lexical: "p:item", NS: "urn:p", Local: "item"}},
	); err != nil {
		t.Fatalf("valid proof rejected: %v", err)
	}
	if err := validateValueConstraintQNameContext(
		"p:item",
		context,
		[]ResolvedValueName{{Lexical: "p:item", NS: "urn:other", Local: "item"}},
	); err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("inconsistent proof error = %v", err)
	}
}

func TestValueConstraintIdentityIncludesQNameContextOnly(t *testing.T) {
	value := testQNameValue(t, "urn:p")
	base := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:p", Local: "item"}},
		Lexical:       "p:item",
		Canonical:     value.CanonicalText(),
		Value:         value,
		qnameContext:  newValueConstraintQNameContext("p:item", func(string) (string, bool) { return "urn:p", true }),
	}
	other := cloneValueConstraint(base)
	other.qnameContext.bindings[0].namespace = "urn:other"
	baseIdentity := NewValueConstraintIdentity(base)
	otherIdentity := NewValueConstraintIdentity(other)
	base.qnameContext.bindings[0].namespace = "urn:mutated"
	if got, ok := baseIdentity.qnameContext.ResolveQName("p:item"); !ok || got.Namespace != "urn:p" {
		t.Fatalf("identity aliased compiler QName context: %+v, %v", got, ok)
	}
	if ValueConstraintIdentityEqual(baseIdentity, otherIdentity) {
		t.Fatal("ValueConstraintIdentityEqual() ignored QName context")
	}
	if !FixedValueConstraintEqual(baseIdentity, otherIdentity) {
		t.Fatal("FixedValueConstraintEqual() made QName context part of semantic value equality")
	}
}

func TestMixedContentConstraintCapturesProspectiveQNameContext(t *testing.T) {
	constraint := mixedContentConstraint("p:item", func(string) (string, bool) { return "urn:p", true })
	if constraint.Value.Type() != valuepkg.NoType {
		t.Fatalf("mixed constraint value type = %v, want untyped", constraint.Value.Type())
	}
	read, ok := newValueConstraintReadFromConstraint(constraint)
	if !ok {
		t.Fatal("mixed constraint read is absent")
	}
	if got, ok := read.ResolveQName("p:item"); !ok || got.Namespace != "urn:p" {
		t.Fatalf("mixed constraint QName resolution = %+v, %v", got, ok)
	}
}
