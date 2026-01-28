package runtime

import "testing"

func TestWildcardAcceptsEnumerationWithBytesFallback(t *testing.T) {
	builder := NewBuilder()
	target := builder.InternNamespace([]byte("urn:target"))
	other := builder.InternNamespace([]byte("urn:other"))
	_ = builder.InternNamespace([]byte("urn:list"))
	schema := builder.Build()

	rule := WildcardRule{
		NS: NSConstraint{
			Kind:      NSEnumeration,
			HasTarget: true,
			HasLocal:  true,
			Off:       0,
			Len:       1,
		},
		TargetNS: target,
	}
	list := []NamespaceID{other}

	if !rule.Accepts(nil, target, &schema.Namespaces, list) {
		t.Fatalf("expected target namespace to be accepted")
	}
	if !rule.Accepts([]byte("urn:target"), 0, &schema.Namespaces, list) {
		t.Fatalf("expected byte-compare target namespace to be accepted")
	}
	if !rule.Accepts(nil, 0, &schema.Namespaces, list) {
		t.Fatalf("expected local namespace to be accepted")
	}
	if !rule.Accepts([]byte("urn:other"), 0, &schema.Namespaces, list) {
		t.Fatalf("expected byte-compare list namespace to be accepted")
	}
	if rule.Accepts([]byte("urn:unknown"), 0, &schema.Namespaces, list) {
		t.Fatalf("expected unknown namespace to be rejected")
	}
}

func TestWildcardAcceptsOther(t *testing.T) {
	builder := NewBuilder()
	target := builder.InternNamespace([]byte("urn:target"))
	other := builder.InternNamespace([]byte("urn:other"))
	schema := builder.Build()

	rule := WildcardRule{
		NS:       NSConstraint{Kind: NSOther},
		TargetNS: target,
	}
	if rule.Accepts(nil, target, &schema.Namespaces, nil) {
		t.Fatalf("expected target namespace to be rejected")
	}
	if rule.Accepts(nil, 0, &schema.Namespaces, nil) {
		t.Fatalf("expected local namespace to be rejected")
	}
	if !rule.Accepts(nil, other, &schema.Namespaces, nil) {
		t.Fatalf("expected other namespace to be accepted")
	}
	if !rule.Accepts([]byte("urn:other"), 0, &schema.Namespaces, nil) {
		t.Fatalf("expected byte-compare other namespace to be accepted")
	}
	if rule.Accepts([]byte("urn:target"), 0, &schema.Namespaces, nil) {
		t.Fatalf("expected byte-compare target namespace to be rejected")
	}
}

func TestWildcardAcceptsNotAbsent(t *testing.T) {
	builder := NewBuilder()
	other := builder.InternNamespace([]byte("urn:other"))
	schema := builder.Build()

	rule := WildcardRule{
		NS: NSConstraint{Kind: NSNotAbsent},
	}
	if rule.Accepts(nil, 0, &schema.Namespaces, nil) {
		t.Fatalf("expected local namespace to be rejected")
	}
	if !rule.Accepts(nil, other, &schema.Namespaces, nil) {
		t.Fatalf("expected other namespace to be accepted")
	}
	if !rule.Accepts([]byte("urn:other"), 0, &schema.Namespaces, nil) {
		t.Fatalf("expected byte-compare other namespace to be accepted")
	}
}

func TestSchemaWildcardAccepts(t *testing.T) {
	builder := NewBuilder()
	target := builder.InternNamespace([]byte("urn:target"))
	other := builder.InternNamespace([]byte("urn:other"))
	schema := builder.Build()

	schema.Wildcards = []WildcardRule{
		{},
		{
			NS:       NSConstraint{Kind: NSEnumeration, HasTarget: true, HasLocal: true, Off: 0, Len: 1},
			TargetNS: target,
		},
	}
	schema.WildcardNS = []NamespaceID{other}

	if !schema.WildcardAccepts(1, nil, other) {
		t.Fatalf("expected schema wildcard to accept list namespace")
	}
	if !schema.WildcardAccepts(1, nil, target) {
		t.Fatalf("expected schema wildcard to accept target namespace")
	}
	if !schema.WildcardAccepts(1, nil, 0) {
		t.Fatalf("expected schema wildcard to accept local namespace")
	}
	if !schema.WildcardAccepts(1, nil, schema.PredefNS.Empty) {
		t.Fatalf("expected schema wildcard to accept empty namespace ID")
	}
}
