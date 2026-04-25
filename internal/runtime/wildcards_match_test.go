package runtime

import "testing"

func TestWildcardAcceptsEnumerationWithBytesFallback(t *testing.T) {
	builder := NewBuilder()
	target, err := builder.InternNamespace([]byte("urn:target"))
	if err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	other, err := builder.InternNamespace([]byte("urn:other"))
	if err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	if _, err := builder.InternNamespace([]byte("urn:list")); err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

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
	target, err := builder.InternNamespace([]byte("urn:target"))
	if err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	other, err := builder.InternNamespace([]byte("urn:other"))
	if err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

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
	other, err := builder.InternNamespace([]byte("urn:other"))
	if err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

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
	target, err := builder.InternNamespace([]byte("urn:target"))
	if err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	other, err := builder.InternNamespace([]byte("urn:other"))
	if err != nil {
		t.Fatalf("InternNamespace: %v", err)
	}
	schema, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

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

func TestWildcardAcceptsExpectedNamespaces(t *testing.T) {
	tests := []struct {
		name string
		rule WildcardRule
		list []string
		want map[string]bool
	}{
		{
			name: "any",
			rule: WildcardRule{NS: NSConstraint{Kind: NSAny}},
			want: map[string]bool{
				"":           true,
				"urn:target": true,
				"urn:list":   true,
				"urn:other":  true,
			},
		},
		{
			name: "other",
			rule: WildcardRule{NS: NSConstraint{Kind: NSOther, HasTarget: true}},
			want: map[string]bool{
				"":           false,
				"urn:target": false,
				"urn:list":   true,
				"urn:other":  true,
			},
		},
		{
			name: "not absent",
			rule: WildcardRule{NS: NSConstraint{Kind: NSNotAbsent}},
			want: map[string]bool{
				"":           false,
				"urn:target": true,
				"urn:list":   true,
				"urn:other":  true,
			},
		},
		{
			name: "target namespace",
			rule: WildcardRule{NS: NSConstraint{Kind: NSEnumeration, HasTarget: true}},
			want: map[string]bool{
				"":           false,
				"urn:target": true,
				"urn:list":   false,
				"urn:other":  false,
			},
		},
		{
			name: "local",
			rule: WildcardRule{NS: NSConstraint{Kind: NSEnumeration, HasLocal: true}},
			want: map[string]bool{
				"":           true,
				"urn:target": false,
				"urn:list":   false,
				"urn:other":  false,
			},
		},
		{
			name: "list with target and local",
			rule: WildcardRule{NS: NSConstraint{Kind: NSEnumeration, HasTarget: true, HasLocal: true}},
			list: []string{"urn:list"},
			want: map[string]bool{
				"":           true,
				"urn:target": true,
				"urn:list":   true,
				"urn:other":  false,
			},
		},
	}

	candidates := []string{"", "urn:target", "urn:list", "urn:other"}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewBuilder()
			nsIDs := make(map[string]NamespaceID)
			register := func(ns string) {
				if ns == "" {
					return
				}
				if _, ok := nsIDs[ns]; ok {
					return
				}
				id, err := builder.InternNamespace([]byte(ns))
				if err != nil {
					t.Fatalf("InternNamespace(%q): %v", ns, err)
				}
				nsIDs[ns] = id
			}
			register("urn:target")
			for _, ns := range tc.list {
				register(ns)
			}
			for _, ns := range candidates {
				register(ns)
			}

			schema, err := builder.Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			rule := tc.rule
			if rule.NS.HasTarget {
				rule.TargetNS = nsIDs["urn:target"]
			}
			nsList := namespaceIDList(tc.list, nsIDs)
			for _, ns := range candidates {
				want := tc.want[ns]

				var nsID NamespaceID
				if ns == "" {
					nsID = schema.PredefNS.Empty
				} else {
					nsID = nsIDs[ns]
				}
				gotByID := rule.Accepts(nil, nsID, &schema.Namespaces, nsList)
				if gotByID != want {
					t.Fatalf("id path allows(%q) = %v, want %v", ns, gotByID, want)
				}

				nsBytes := []byte(ns)
				if ns == "" {
					nsBytes = nil
				}
				gotByBytes := rule.Accepts(nsBytes, 0, &schema.Namespaces, nsList)
				if gotByBytes != want {
					t.Fatalf("bytes path allows(%q) = %v, want %v", ns, gotByBytes, want)
				}
			}
		})
	}
}

func namespaceIDList(list []string, nsIDs map[string]NamespaceID) []NamespaceID {
	out := make([]NamespaceID, 0, len(list))
	for _, ns := range list {
		out = append(out, nsIDs[ns])
	}
	return out
}
