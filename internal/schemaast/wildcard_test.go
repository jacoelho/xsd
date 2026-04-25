package schemaast

import "testing"

func TestIntersectWildcardsDetailed(t *testing.T) {
	t.Run("target namespace intersects matching list", func(t *testing.T) {
		target := wildcardConstraint{constraint: NSCTargetNamespace, target: "urn:test"}
		list := wildcardConstraint{constraint: NSCList, target: "urn:test", list: []NamespaceURI{"urn:test", "urn:other"}}

		got, expressible, empty := intersectWildcardsDetailed(target, list)
		if !expressible || empty {
			t.Fatalf("expected expressible non-empty intersection, got expressible=%v empty=%v", expressible, empty)
		}
		if got.constraint != target.constraint || got.target != target.target {
			t.Fatalf("expected target constraint %v, got %v", target, got)
		}
	})

	t.Run("list intersects other by filtering entries", func(t *testing.T) {
		list := wildcardConstraint{constraint: NSCList, target: "urn:test", list: []NamespaceURI{"urn:other", "urn:test"}}
		other := wildcardConstraint{constraint: NSCOther, target: "urn:test"}

		got, expressible, empty := intersectWildcardsDetailed(list, other)
		if !expressible || empty {
			t.Fatalf("expected expressible non-empty intersection, got expressible=%v empty=%v", expressible, empty)
		}
		if got.constraint != NSCList || len(got.list) != 1 || got.list[0] != "urn:other" {
			t.Fatalf("expected filtered list intersection, got %+v", got)
		}
	})

	t.Run("other with different concrete targets is not expressible", func(t *testing.T) {
		left := wildcardConstraint{constraint: NSCOther, target: "urn:left"}
		right := wildcardConstraint{constraint: NSCOther, target: "urn:right"}

		_, expressible, empty := intersectWildcardsDetailed(left, right)
		if expressible || empty {
			t.Fatalf("expected non-expressible intersection, got expressible=%v empty=%v", expressible, empty)
		}
	})

	t.Run("target namespace with not absent and empty target is empty", func(t *testing.T) {
		target := wildcardConstraint{constraint: NSCTargetNamespace, target: NamespaceEmpty}
		notAbsent := wildcardConstraint{constraint: NSCNotAbsent}

		_, expressible, empty := intersectWildcardsDetailed(target, notAbsent)
		if !expressible || !empty {
			t.Fatalf("expected expressible empty intersection, got expressible=%v empty=%v", expressible, empty)
		}
	})

	t.Run("local with not absent is empty", func(t *testing.T) {
		local := wildcardConstraint{constraint: NSCLocal}
		notAbsent := wildcardConstraint{constraint: NSCNotAbsent}

		_, expressible, empty := intersectWildcardsDetailed(local, notAbsent)
		if !expressible || !empty {
			t.Fatalf("expected expressible empty intersection, got expressible=%v empty=%v", expressible, empty)
		}
	})
}
