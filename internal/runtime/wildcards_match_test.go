package runtime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

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

func TestWildcardAcceptsModelParity(t *testing.T) {
	tests := []struct {
		name       string
		constraint model.NamespaceConstraint
		list       []model.NamespaceURI
		target     model.NamespaceURI
	}{
		{
			name:       "any",
			constraint: model.NSCAny,
		},
		{
			name:       "other",
			constraint: model.NSCOther,
			target:     "urn:target",
		},
		{
			name:       "not absent",
			constraint: model.NSCNotAbsent,
			target:     "urn:target",
		},
		{
			name:       "target namespace",
			constraint: model.NSCTargetNamespace,
			target:     "urn:target",
		},
		{
			name:       "local",
			constraint: model.NSCLocal,
			target:     "urn:target",
		},
		{
			name:       "list with placeholder and local",
			constraint: model.NSCList,
			target:     "urn:target",
			list: []model.NamespaceURI{
				model.NamespaceTargetPlaceholder,
				model.NamespaceURI("urn:list"),
				model.NamespaceEmpty,
			},
		},
	}

	candidates := []model.NamespaceURI{
		model.NamespaceEmpty,
		model.NamespaceURI("urn:target"),
		model.NamespaceURI("urn:list"),
		model.NamespaceURI("urn:other"),
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			builder := NewBuilder()
			nsIDs := make(map[model.NamespaceURI]NamespaceID)
			register := func(ns model.NamespaceURI) {
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
			register(tc.target)
			for _, ns := range tc.list {
				if ns == model.NamespaceTargetPlaceholder {
					continue
				}
				register(ns)
			}
			for _, ns := range candidates {
				register(ns)
			}

			schema, err := builder.Build()
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			rule, nsList := runtimeRuleFromModel(tc.constraint, tc.list, tc.target, nsIDs)
			for _, ns := range candidates {
				want := model.AllowsNamespace(tc.constraint, tc.list, tc.target, ns)

				var nsID NamespaceID
				if ns == model.NamespaceEmpty {
					nsID = schema.PredefNS.Empty
				} else {
					nsID = nsIDs[ns]
				}
				gotByID := rule.Accepts(nil, nsID, &schema.Namespaces, nsList)
				if gotByID != want {
					t.Fatalf("id path allows(%q) = %v, want %v", ns, gotByID, want)
				}

				nsBytes := []byte(ns)
				if ns == model.NamespaceEmpty {
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

func runtimeRuleFromModel(
	constraint model.NamespaceConstraint,
	list []model.NamespaceURI,
	target model.NamespaceURI,
	nsIDs map[model.NamespaceURI]NamespaceID,
) (WildcardRule, []NamespaceID) {
	rule := WildcardRule{}
	if target != "" {
		rule.TargetNS = nsIDs[target]
	}
	var nsList []NamespaceID

	switch constraint {
	case model.NSCAny:
		rule.NS.Kind = NSAny
	case model.NSCOther:
		rule.NS.Kind = NSOther
		rule.NS.HasTarget = true
	case model.NSCNotAbsent:
		rule.NS.Kind = NSNotAbsent
	case model.NSCTargetNamespace:
		rule.NS.Kind = NSEnumeration
		rule.NS.HasTarget = true
	case model.NSCLocal:
		rule.NS.Kind = NSEnumeration
		rule.NS.HasLocal = true
	case model.NSCList:
		rule.NS.Kind = NSEnumeration
		for _, ns := range list {
			switch ns {
			case model.NamespaceTargetPlaceholder:
				rule.NS.HasTarget = true
			case model.NamespaceEmpty:
				rule.NS.HasLocal = true
			default:
				nsList = append(nsList, nsIDs[ns])
			}
		}
	default:
		rule.NS.Kind = NSAny
	}

	return rule, nsList
}
