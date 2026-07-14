package validate

import (
	"encoding/xml"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestXSIAttributeIdentityKey(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="URIs"><xs:list itemType="xs:anyURI"/></xs:simpleType>
</xs:schema>`)
	nilName, _ := rt.LookupQName(vocab.XSINamespaceURI, vocab.XSIAttrNil)
	typeName, _ := rt.LookupQName(vocab.XSINamespaceURI, vocab.XSIAttrType)
	schemaLocationName, _ := rt.LookupQName(vocab.XSINamespaceURI, vocab.XSIAttrSchemaLocation)
	noNamespaceSchemaLocationName, _ := rt.LookupQName(vocab.XSINamespaceURI, vocab.XSIAttrNoNamespaceSchemaLocation)
	ctx := StartContext{Line: 2, Column: 3, Path: "/root"}

	name, key, ok, err := XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNil}, " 1 ", nil, ctx)
	if err != nil {
		t.Fatalf("XSIAttributeIdentityKey(nil) error = %v", err)
	}
	if !ok || name != nilName || key != runtime.SimpleIdentityKey(runtime.PrimitiveBoolean, "true") {
		t.Fatalf("XSIAttributeIdentityKey(nil) = %v %q %v, want nil boolean true", name, key, ok)
	}

	const typeCanonical = "{urn:test}T"
	resolveType := func(value string) (string, string, bool) {
		if value != "p:T" {
			return "", "", false
		}
		return "urn:test", "T", true
	}
	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrType}, " p:T ", resolveType, ctx)
	if err != nil {
		t.Fatalf("XSIAttributeIdentityKey(type) error = %v", err)
	}
	if !ok || name != typeName || key != runtime.SimpleIdentityKey(runtime.PrimitiveQName, typeCanonical) {
		t.Fatalf("XSIAttributeIdentityKey(type) = %v %q %v, want type key", name, key, ok)
	}

	anyURI := simpleTypeIDByNameForTest(t, rt, vocab.XSDNamespaceURI, vocab.XSDValueAnyURI)
	ordinaryAnyURI, err := rt.ValidateSimpleValue(anyURI, "one.xsd", nil, runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(anyURI) error = %v", err)
	}
	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNoNamespaceSchemaLocation}, "  one.xsd\t", nil, ctx)
	if err != nil {
		t.Fatalf("XSIAttributeIdentityKey(noNamespaceSchemaLocation) error = %v", err)
	}
	if !ok || name != noNamespaceSchemaLocationName || key != ordinaryAnyURI.Identity {
		t.Fatalf("XSIAttributeIdentityKey(noNamespaceSchemaLocation) = %v %q %v, want ordinary anyURI key %q", name, key, ok, ordinaryAnyURI.Identity)
	}
	emptyAnyURI, err := rt.ValidateSimpleValue(anyURI, "", nil, runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(empty anyURI) error = %v", err)
	}
	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNoNamespaceSchemaLocation}, " \t", nil, ctx)
	if err != nil {
		t.Fatalf("XSIAttributeIdentityKey(empty noNamespaceSchemaLocation) error = %v", err)
	}
	if !ok || name != noNamespaceSchemaLocationName || key != emptyAnyURI.Identity {
		t.Fatalf("XSIAttributeIdentityKey(empty noNamespaceSchemaLocation) = %v %q %v, want ordinary empty anyURI key %q", name, key, ok, emptyAnyURI.Identity)
	}

	uriList := simpleTypeIDByNameForTest(t, rt, "", "URIs")
	ordinaryList, err := rt.ValidateSimpleValue(uriList, "urn:a a.xsd urn:b b.xsd", nil, runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(list<anyURI>) error = %v", err)
	}
	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrSchemaLocation}, " urn:a\ta.xsd\nurn:b  b.xsd ", nil, ctx)
	if err != nil {
		t.Fatalf("XSIAttributeIdentityKey(schemaLocation) error = %v", err)
	}
	if !ok || name != schemaLocationName || key != ordinaryList.Identity {
		t.Fatalf("XSIAttributeIdentityKey(schemaLocation) = %v %q %v, want ordinary list<anyURI> key %q", name, key, ok, ordinaryList.Identity)
	}

	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: "other"}, " a\tb ", nil, ctx)
	if err != nil || ok || name != (runtime.QName{}) || key != "" {
		t.Fatalf("XSIAttributeIdentityKey(other) = %v %q %v err %v, want ignored", name, key, ok, err)
	}
}

func TestXSIAttributeIdentityKeyErrors(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	ctx := StartContext{Line: 2, Column: 3, Path: "/root"}

	name, key, ok, err := XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: "unknown"}, "x", nil, ctx)
	if err != nil || ok || name != (runtime.QName{}) || key != "" {
		t.Fatalf("XSIAttributeIdentityKey(unknown) = %v %q %v err %v, want ignored", name, key, ok, err)
	}

	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNil}, "maybe", nil, ctx)
	if name != (runtime.QName{}) || key != "" || ok {
		t.Fatalf("XSIAttributeIdentityKey(invalid nil) = %v %q %v, want empty error result", name, key, ok)
	}
	expectXSDCode(t, err, xsderrors.CodeValidationAttribute)

	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrType}, "bad", func(string) (string, string, bool) {
		return "", "", false
	}, ctx)
	if name != (runtime.QName{}) || key != "" || ok {
		t.Fatalf("XSIAttributeIdentityKey(invalid type) = %v %q %v, want empty error result", name, key, ok)
	}
	expectXSDCode(t, err, xsderrors.CodeValidationAttribute)

	for _, test := range []struct {
		name    string
		local   string
		lexical string
	}{
		{name: "schemaLocation", local: vocab.XSIAttrSchemaLocation, lexical: "urn:test %zz"},
		{name: "noNamespaceSchemaLocation", local: vocab.XSIAttrNoNamespaceSchemaLocation, lexical: "%zz"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			name, key, ok, err := XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: test.local}, test.lexical, nil, ctx)
			if name != (runtime.QName{}) || key != "" || ok {
				t.Fatalf("XSIAttributeIdentityKey(invalid %s) = %v %q %v, want empty error result", test.local, name, key, ok)
			}
			expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
		})
	}
}

func simpleTypeIDByNameForTest(t *testing.T, rt *runtime.Schema, ns, local string) runtime.SimpleTypeID {
	t.Helper()

	name, ok := rt.LookupQName(ns, local)
	if !ok {
		t.Fatalf("LookupQName(%q, %q) did not find compiled name", ns, local)
	}
	typ, ok := rt.Type(name)
	if !ok {
		t.Fatalf("Type(%q, %q) did not find compiled type", ns, local)
	}
	id, ok := typ.Simple()
	if !ok {
		t.Fatalf("Type(%q, %q) is not simple", ns, local)
	}
	return id
}
