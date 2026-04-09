package model

import "testing"

func TestCopyValueNamespaceContextClonesForPreservedSourceNamespace(t *testing.T) {
	t.Parallel()

	src := map[string]string{"": "urn:source", "p": "urn:p"}
	cloned := copyValueNamespaceContext(src, CopyOptions{
		RemapQName:              NilRemap,
		SourceNamespace:         "urn:source",
		PreserveSourceNamespace: true,
	})

	src["p"] = "urn:changed"
	if got := cloned["p"]; got != "urn:p" {
		t.Fatalf("cloned context = %q, want %q", got, "urn:p")
	}
}

func TestCopyValueNamespaceContextSharesForMergeWithoutRemap(t *testing.T) {
	t.Parallel()

	src := map[string]string{"": "urn:source", "p": "urn:p"}
	shared := copyValueNamespaceContext(src, CopyOptions{
		RemapQName:      NilRemap,
		SourceNamespace: "urn:source",
	})

	src["p"] = "urn:changed"
	if got := shared["p"]; got != "urn:changed" {
		t.Fatalf("shared context = %q, want %q", got, "urn:changed")
	}
}

func TestCopyValueNamespaceContextChameleonRemapsDefaultNamespace(t *testing.T) {
	t.Parallel()

	src := map[string]string{"": "", "p": "urn:p"}
	chameleon := copyValueNamespaceContext(src, CopyOptions{
		SourceNamespace: "urn:target",
		RemapQName: func(q QName) QName {
			if q.Namespace == "" {
				q.Namespace = "urn:target"
			}
			return q
		},
	})

	src[""] = "urn:source"
	if got := chameleon[""]; got != "urn:target" {
		t.Fatalf("default namespace = %q, want %q", got, "urn:target")
	}
}

func TestCopyIdentityConstraintsSharesNamespaceContextForMergeWithoutRemap(t *testing.T) {
	t.Parallel()

	constraints := []*IdentityConstraint{{
		Name:             "id",
		NamespaceContext: map[string]string{"p": "urn:p"},
	}}
	copied := copyIdentityConstraints(constraints, CopyOptions{
		RemapQName:      NilRemap,
		SourceNamespace: "urn:source",
	})

	constraints[0].NamespaceContext["p"] = "urn:changed"
	if got := copied[0].NamespaceContext["p"]; got != "urn:changed" {
		t.Fatalf("shared constraint namespace = %q, want %q", got, "urn:changed")
	}
}
