package types

import "testing"

func TestElementDeclCopy_ConstraintsAreIndependent(t *testing.T) {
	orig := &ElementDecl{
		Name: QName{Local: "elem"},
		Constraints: []*IdentityConstraint{&IdentityConstraint{
			Name:             "idc",
			Fields:           []Field{{XPath: "a"}},
			NamespaceContext: map[string]string{"p": "urn:test"},
		}},
	}

	cloned := orig.Copy(CopyOptions{SourceNamespace: NamespaceURI("urn:copy"), RemapQName: NilRemap})
	if cloned.Constraints[0].TargetNamespace != NamespaceURI("urn:copy") {
		t.Fatalf("TargetNamespace = %q, want %q", cloned.Constraints[0].TargetNamespace, "urn:copy")
	}

	cloned.Constraints[0].Fields[0].ResolvedType = GetBuiltin(TypeNameString)
	cloned.Constraints[0].NamespaceContext["p"] = "urn:changed"

	if orig.Constraints[0].Fields[0].ResolvedType != nil {
		t.Fatal("ResolvedType leaked into original constraint")
	}
	if got := orig.Constraints[0].NamespaceContext["p"]; got != "urn:test" {
		t.Fatalf("NamespaceContext[\"p\"] = %q, want %q", got, "urn:test")
	}
}

func TestAttributeGroupCopy_AnyAttributeIsolated(t *testing.T) {
	orig := &AttributeGroup{
		Name: QName{Local: "attrs"},
		AnyAttribute: &AnyAttribute{
			NamespaceList: []NamespaceURI{"urn:a", "urn:b"},
		},
	}

	cloned := orig.Copy(CopyOptions{SourceNamespace: NamespaceURI("urn:copy"), RemapQName: NilRemap})
	cloned.AnyAttribute.NamespaceList[0] = "urn:changed"

	if orig.AnyAttribute.NamespaceList[0] == "urn:changed" {
		t.Fatal("AnyAttribute NamespaceList leaked into original")
	}
}

func TestModelGroupCopy_AnyElementIsolated(t *testing.T) {
	orig := &ModelGroup{
		Kind: Sequence,
		Particles: []Particle{
			&AnyElement{NamespaceList: []NamespaceURI{"urn:a", "urn:b"}},
		},
	}

	cloned := orig.Copy(CopyOptions{SourceNamespace: NamespaceURI("urn:copy"), RemapQName: NilRemap})
	anyElem, ok := cloned.Particles[0].(*AnyElement)
	if !ok {
		t.Fatalf("Particle = %T, want *AnyElement", cloned.Particles[0])
	}
	anyElem.NamespaceList[0] = "urn:changed"

	origAny, ok := orig.Particles[0].(*AnyElement)
	if !ok {
		t.Fatalf("Original particle = %T, want *AnyElement", orig.Particles[0])
	}
	if origAny.NamespaceList[0] == "urn:changed" {
		t.Fatal("AnyElement NamespaceList leaked into original")
	}
}

func TestSimpleTypeCopy_FacetsAreIndependent(t *testing.T) {
	orig := &SimpleType{
		QName: QName{Local: "t"},
		Restriction: &Restriction{
			Facets: []any{"a"},
		},
	}

	cloned := orig.Copy(CopyOptions{SourceNamespace: NamespaceURI("urn:copy"), RemapQName: NilRemap})
	cloned.Restriction.Facets[0] = "b"

	if orig.Restriction.Facets[0] == "b" {
		t.Fatal("Restriction facets leaked into original")
	}
}
