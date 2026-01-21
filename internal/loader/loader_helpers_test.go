package loader

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveLocationAndGetLoaded(t *testing.T) {
	loader := NewLoader(Config{BasePath: "schemas"})
	schema := &parser.Schema{}

	abs, err := loader.resolveLocation("/abs/schema.xsd")
	if err != nil {
		t.Fatalf("resolveLocation absolute error = %v", err)
	}
	if abs != "/abs/schema.xsd" {
		t.Fatalf("expected absolute path to remain unchanged, got %q", abs)
	}

	rel, err := loader.resolveLocation("a/b.xsd")
	if err != nil {
		t.Fatalf("resolveLocation relative error = %v", err)
	}
	if rel != "schemas/a/b.xsd" {
		t.Fatalf("expected base path join, got %q", rel)
	}

	key := loader.loadKey(loader.defaultFSContext(), rel)
	loader.state.loaded[key] = schema
	loaded, ok, err := loader.GetLoaded("a/b.xsd")
	if err != nil {
		t.Fatalf("GetLoaded error = %v", err)
	}
	if !ok || loaded != schema {
		t.Fatalf("expected GetLoaded to return cached schema")
	}
}

func TestResolveLocationRejectsTraversal(t *testing.T) {
	loader := NewLoader(Config{BasePath: "schemas"})
	if _, err := loader.resolveLocation("../outside.xsd"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestIsNotFound(t *testing.T) {
	if !isNotFound(fs.ErrNotExist) {
		t.Fatalf("expected isNotFound to detect ErrNotExist")
	}
	if isNotFound(errors.New("other")) {
		t.Fatalf("expected non-ErrNotExist to be false")
	}
}

func TestDeepCopyModelGroup(t *testing.T) {
	original := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{
			&types.ElementDecl{Name: types.QName{Local: "a"}},
		},
	}

	clone := deepCopyModelGroup(original)
	if clone == original {
		t.Fatalf("expected a new model group instance")
	}
	if len(clone.Particles) != len(original.Particles) {
		t.Fatalf("expected copied particles length to match")
	}
	clone.Particles[0] = &types.ElementDecl{Name: types.QName{Local: "b"}}
	if original.Particles[0].(*types.ElementDecl).Name.Local != "a" {
		t.Fatalf("expected original particles to remain unchanged")
	}
}

func TestNormalizeAttributeForms(t *testing.T) {
	qualified := &types.AttributeDecl{Name: types.QName{Local: "q"}, Form: types.FormDefault}
	extAttr := &types.AttributeDecl{Name: types.QName{Local: "e"}, Form: types.FormDefault}
	ct := &types.ComplexType{}
	ct.SetAttributes([]*types.AttributeDecl{qualified})
	ct.SetContent(&types.ComplexContent{
		Extension: &types.Extension{Attributes: []*types.AttributeDecl{extAttr}},
	})

	normalizeAttributeForms(ct, parser.Qualified)
	if qualified.Form != types.FormQualified {
		t.Fatalf("expected qualified attribute to be FormQualified")
	}
	if extAttr.Form != types.FormQualified {
		t.Fatalf("expected extension attribute to be FormQualified")
	}

	restrAttr := &types.AttributeDecl{Name: types.QName{Local: "r"}, Form: types.FormDefault}
	ct = &types.ComplexType{}
	ct.SetAttributes([]*types.AttributeDecl{{Name: types.QName{Local: "u"}, Form: types.FormDefault}})
	ct.SetContent(&types.ComplexContent{
		Restriction: &types.Restriction{Attributes: []*types.AttributeDecl{restrAttr}},
	})
	normalizeAttributeForms(ct, parser.Unqualified)
	for _, attr := range []*types.AttributeDecl{ct.Attributes()[0], restrAttr} {
		if attr.Form != types.FormUnqualified {
			t.Fatalf("expected FormUnqualified, got %v", attr.Form)
		}
	}
}

func TestElementDeclEquivalent(t *testing.T) {
	elemA := &types.ElementDecl{
		Name:     types.QName{Local: "a"},
		Type:     types.GetBuiltin(types.TypeNameString),
		Form:     types.FormQualified,
		Fixed:    "x",
		HasFixed: true,
	}
	elemB := &types.ElementDecl{
		Name:     types.QName{Local: "a"},
		Type:     types.GetBuiltin(types.TypeNameString),
		Form:     types.FormQualified,
		Fixed:    "x",
		HasFixed: true,
	}

	if !elementDeclEquivalent(elemA, elemB) {
		t.Fatalf("expected equivalent element declarations")
	}

	elemA.HasDefault = true
	elemB.HasDefault = true
	elemA.Default = "x"
	elemB.Default = "x"
	if !elementDeclEquivalent(elemA, elemB) {
		t.Fatalf("expected equivalent element declarations with defaults")
	}

	elemB.Default = "y"
	if elementDeclEquivalent(elemA, elemB) {
		t.Fatalf("expected default mismatch to be non-equivalent")
	}

	elemA.Default = ""
	elemA.HasDefault = false
	elemB.Default = ""
	elemB.HasDefault = true
	if elementDeclEquivalent(elemA, elemB) {
		t.Fatalf("expected default presence mismatch to be non-equivalent")
	}
}

func TestElementDeclEquivalentNamespaceContext(t *testing.T) {
	elemA := &types.ElementDecl{
		Name: types.QName{Local: "a"},
		Type: types.GetBuiltin(types.TypeNameString),
		Form: types.FormQualified,
		Constraints: []*types.IdentityConstraint{{
			Name:             "c",
			Type:             types.UniqueConstraint,
			TargetNamespace:  types.NamespaceURI("urn:one"),
			NamespaceContext: map[string]string{"p": "urn:one"},
			Selector:         types.Selector{XPath: "p:el"},
			Fields:           []types.Field{{XPath: "@p:attr"}},
		}},
	}
	elemB := &types.ElementDecl{
		Name: types.QName{Local: "a"},
		Type: types.GetBuiltin(types.TypeNameString),
		Form: types.FormQualified,
		Constraints: []*types.IdentityConstraint{{
			Name:             "c",
			Type:             types.UniqueConstraint,
			TargetNamespace:  types.NamespaceURI("urn:one"),
			NamespaceContext: map[string]string{"p": "urn:two"},
			Selector:         types.Selector{XPath: "p:el"},
			Fields:           []types.Field{{XPath: "@p:attr"}},
		}},
	}

	if elementDeclEquivalent(elemA, elemB) {
		t.Fatalf("expected namespace context mismatch to be non-equivalent")
	}
}

func TestLoadValidatesCachedSchema(t *testing.T) {
	fsys := fstest.MapFS{
		"a.xsd": {Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:a"
           xmlns:tns="urn:a"
           elementFormDefault="qualified">
  <xs:import namespace="urn:b" schemaLocation="b.xsd"/>
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
		"b.xsd": {Data: []byte(`<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:b"
           xmlns:tns="urn:b"
           elementFormDefault="qualified">
  <xs:simpleType name="T">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`)},
	}

	loader := NewLoader(Config{FS: fsys})
	if _, err := loader.Load("a.xsd"); err != nil {
		t.Fatalf("load a.xsd: %v", err)
	}

	schema, err := loader.Load("b.xsd")
	if err != nil {
		t.Fatalf("load b.xsd: %v", err)
	}

	elem := schema.ElementDecls[types.QName{Namespace: types.NamespaceURI("urn:b"), Local: "root"}]
	if elem == nil {
		t.Fatalf("expected root element declaration to be present")
	}
	st, ok := elem.Type.(*types.SimpleType)
	if !ok {
		t.Fatalf("expected simple type, got %T", elem.Type)
	}
	if types.IsPlaceholderSimpleType(st) {
		t.Fatalf("expected resolved type, got placeholder")
	}
}
