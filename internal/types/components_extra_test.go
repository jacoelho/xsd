package types

import "testing"

func TestComplexTypeMethods(t *testing.T) {
	qname := QName{Namespace: "urn:ct", Local: "ct"}
	ct := NewComplexType(qname, "urn:ct")
	if ct.ComponentName() != qname {
		t.Fatalf("unexpected component name")
	}
	if ct.DeclaredNamespace() != "urn:ct" {
		t.Fatalf("unexpected declared namespace")
	}
	if ct.IsBuiltin() {
		t.Fatalf("complex types should not be builtin")
	}
	if ct.BaseType().Name().Local != "anyType" {
		t.Fatalf("unexpected base type")
	}
	ct.ResolvedBase = GetBuiltin(TypeNameString)
	if ct.BaseType().Name().Local != "string" {
		t.Fatalf("unexpected resolved base type")
	}
	if ct.ResolvedBaseType() == nil {
		t.Fatalf("expected resolved base type")
	}
	if ct.PrimitiveType() != nil {
		t.Fatalf("complex types should not have primitive type")
	}
	if ct.FundamentalFacets() != nil {
		t.Fatalf("complex types should not have fundamental facets")
	}
	if ct.WhiteSpace() != WhiteSpacePreserve {
		t.Fatalf("unexpected whitespace for complex type")
	}
	ct.SetMixed(true)
	if !ct.Mixed() {
		t.Fatalf("expected mixed content")
	}
	ct.SetAttributes([]*AttributeDecl{{Name: QName{Local: "a"}}})
	if len(ct.Attributes()) != 1 {
		t.Fatalf("expected attributes to be set")
	}
	ct.SetAnyAttribute(&AnyAttribute{NamespaceList: []NamespaceURI{"##any"}})
	if ct.AnyAttribute() == nil {
		t.Fatalf("expected anyAttribute to be set")
	}
	ct.DerivationMethod = DerivationExtension
	if !ct.IsExtension() || ct.IsRestriction() || !ct.IsDerived() {
		t.Fatalf("unexpected derivation flags")
	}
}

func TestComponentConstructorsAndCopy(t *testing.T) {
	elem := &ElementDecl{
		Name:           QName{Namespace: "urn:src", Local: "e"},
		Type:           GetBuiltin(TypeNameString),
		MinOccurs:      0,
		MaxOccurs:      1,
		SourceNamespace: "urn:src",
	}
	if _, err := NewElementDeclFromParsed(elem); err != nil {
		t.Fatalf("unexpected element error: %v", err)
	}
	if elem.MinOcc() != 0 || elem.MaxOcc() != 1 {
		t.Fatalf("unexpected element occurrence bounds")
	}
	if elem.ComponentName() != elem.Name {
		t.Fatalf("unexpected element component name")
	}
	if elem.DeclaredNamespace() != "urn:src" {
		t.Fatalf("unexpected element declared namespace")
	}
	if _, err := NewElementDeclFromParsed(nil); err == nil {
		t.Fatalf("expected nil element error")
	}

	attr := &AttributeDecl{
		Name:           QName{Namespace: "urn:src", Local: "a"},
		Type:           GetBuiltin(TypeNameString),
		SourceNamespace: "urn:src",
	}
	if _, err := NewAttributeDeclFromParsed(attr); err != nil {
		t.Fatalf("unexpected attribute error: %v", err)
	}
	if attr.ComponentName() != attr.Name {
		t.Fatalf("unexpected attribute component name")
	}
	if attr.DeclaredNamespace() != "urn:src" {
		t.Fatalf("unexpected attribute declared namespace")
	}
	if _, err := NewAttributeDeclFromParsed(nil); err == nil {
		t.Fatalf("expected nil attribute error")
	}

	opts := CopyOptions{
		SourceNamespace: "urn:dst",
		RemapQName: func(q QName) QName {
			return QName{Namespace: "urn:dst", Local: q.Local}
		},
	}
	elem.SubstitutionGroup = QName{Local: "head"}
	elem.Constraints = []*IdentityConstraint{
		{
			Name:           "c1",
			Type:           KeyRefConstraint,
			ReferQName:     QName{Local: "ref"},
			NamespaceContext: map[string]string{"p": "urn:src"},
		},
	}
	elemCopy := elem.Copy(opts)
	if elemCopy.Name.Namespace != "urn:dst" {
		t.Fatalf("expected element name remap")
	}
	if elemCopy.SubstitutionGroup.Namespace != "urn:dst" {
		t.Fatalf("expected substitutionGroup remap")
	}
	if elemCopy.Constraints[0].TargetNamespace != "urn:dst" {
		t.Fatalf("expected constraint target namespace remap")
	}
	if elemCopy.Constraints[0].ReferQName.Namespace != "urn:dst" {
		t.Fatalf("expected refer QName remap")
	}

	group := &AttributeGroup{
		Name:         QName{Namespace: "urn:src", Local: "ag"},
		Attributes:   []*AttributeDecl{attr},
		AttrGroups:   []QName{{Local: "base"}},
		AnyAttribute: &AnyAttribute{NamespaceList: []NamespaceURI{"##any"}},
		SourceNamespace: "urn:src",
	}
	groupCopy := group.Copy(opts)
	if groupCopy.Name.Namespace != "urn:dst" {
		t.Fatalf("expected attributeGroup remap")
	}
	if group.ComponentName() != group.Name {
		t.Fatalf("unexpected attributeGroup component name")
	}
	if group.DeclaredNamespace() != "urn:src" {
		t.Fatalf("unexpected attributeGroup declared namespace")
	}
	if groupCopy.AnyAttribute == nil || len(groupCopy.AnyAttribute.NamespaceList) != 1 {
		t.Fatalf("expected anyAttribute copy")
	}

	notation := &NotationDecl{Name: QName{Namespace: "urn:src", Local: "note"}, SourceNamespace: "urn:src"}
	if notation.ComponentName() != notation.Name {
		t.Fatalf("unexpected notation component name")
	}
	if notation.DeclaredNamespace() != "urn:src" {
		t.Fatalf("unexpected notation declared namespace")
	}
	notationCopy := notation.Copy(opts)
	if notationCopy.Name.Namespace != "urn:dst" || notationCopy.SourceNamespace != "urn:dst" {
		t.Fatalf("expected notation remap")
	}

	if UniqueConstraint.String() != "unique" || KeyConstraint.String() != "key" || KeyRefConstraint.String() != "keyref" {
		t.Fatalf("unexpected constraint type strings")
	}
	var unknown ConstraintType = 99
	if unknown.String() != "unknown" {
		t.Fatalf("expected unknown constraint type string")
	}
}

func TestCopyTypeAndParticles(t *testing.T) {
	opts := CopyOptions{
		SourceNamespace: "urn:dst",
		RemapQName: func(q QName) QName {
			return QName{Namespace: "urn:dst", Local: q.Local}
		},
	}

	if CopyType(nil, opts) != nil {
		t.Fatalf("expected nil copy for nil type")
	}

	st := &SimpleType{
		QName: QName{Namespace: "", Local: "st"},
	}
	st.SetVariety(AtomicVariety)
	st.Restriction = &Restriction{Base: QName{Namespace: XSDNamespace, Local: "string"}}
	copied := CopyType(st, opts).(*SimpleType)
	if copied.QName.Namespace != "urn:dst" {
		t.Fatalf("expected simpleType QName remap")
	}

	ct := NewComplexType(QName{Namespace: "", Local: "ct"}, "urn:src")
	ctCopy := CopyType(ct, opts).(*ComplexType)
	if ctCopy.QName.Namespace != "urn:dst" {
		t.Fatalf("expected complexType QName remap")
	}

	bt := GetBuiltin(TypeNameString)
	if CopyType(bt, opts) != bt {
		t.Fatalf("expected builtin type to be returned as-is")
	}

	elem := &ElementDecl{Name: QName{Namespace: "", Local: "e"}}
	any := &AnyElement{NamespaceList: []NamespaceURI{"##any"}, MinOccurs: 1, MaxOccurs: 1}
	group := &ModelGroup{
		Kind:      Sequence,
		Particles: []Particle{elem, any},
		MinOccurs: 1,
		MaxOccurs: 1,
	}
	groupRef := &GroupRef{RefQName: QName{Namespace: "", Local: "g"}, MinOccurs: 1, MaxOccurs: 1}

	if copiedElem := copyParticle(elem, opts); copiedElem.(*ElementDecl).Name.Namespace != "urn:dst" {
		t.Fatalf("expected element particle remap")
	}
	if copiedGroup := copyParticle(group, opts); copiedGroup.(*ModelGroup) == nil {
		t.Fatalf("expected modelGroup copy")
	}
	if copiedGroupRef := copyParticle(groupRef, opts); copiedGroupRef.(*GroupRef).RefQName.Namespace != "urn:dst" {
		t.Fatalf("expected groupRef remap")
	}
	if copiedAny := copyParticle(any, opts); copiedAny.(*AnyElement) == nil {
		t.Fatalf("expected any element copy")
	}
}

func TestDerivationSetHelpers(t *testing.T) {
	set := DerivationSet(DerivationExtension | DerivationRestriction)
	if !set.Has(DerivationExtension) || set.Has(DerivationList) {
		t.Fatalf("unexpected derivation set Has result")
	}
	expanded := set.Add(DerivationList)
	if !expanded.Has(DerivationList) {
		t.Fatalf("expected derivation set to include list")
	}
	all := AllDerivations()
	if !all.Has(DerivationExtension) || !all.Has(DerivationRestriction) || !all.Has(DerivationList) || !all.Has(DerivationUnion) {
		t.Fatalf("expected AllDerivations to include all methods")
	}
}
