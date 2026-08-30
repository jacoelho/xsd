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

	identity, err := xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNil}, " 1 ", nil, ctx)
	if err != nil {
		t.Fatalf("xsiAttributeIdentityKey(nil) error = %v", err)
	}
	if !identity.present || identity.name != nilName || identity.key != runtime.SimpleIdentityKey(runtime.PrimitiveBoolean, "true") {
		t.Fatalf("xsiAttributeIdentityKey(nil) = %+v, want nil boolean true", identity)
	}

	const typeCanonical = "{urn:test}T"
	resolveType := func(value string) (string, string, bool) {
		if value != "p:T" {
			return "", "", false
		}
		return "urn:test", "T", true
	}
	identity, err = xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrType}, " p:T ", resolveType, ctx)
	if err != nil {
		t.Fatalf("xsiAttributeIdentityKey(type) error = %v", err)
	}
	if !identity.present || identity.name != typeName || identity.key != runtime.SimpleIdentityKey(runtime.PrimitiveQName, typeCanonical) {
		t.Fatalf("xsiAttributeIdentityKey(type) = %+v, want type key", identity)
	}

	anyURI := simpleTypeIDByNameForTest(t, rt, vocab.XSDNamespaceURI, vocab.XSDValueAnyURI)
	ordinaryAnyURI, err := rt.ValidateSimpleValue(anyURI, "one.xsd", nil, runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(anyURI) error = %v", err)
	}
	identity, err = xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNoNamespaceSchemaLocation}, "  one.xsd\t", nil, ctx)
	if err != nil {
		t.Fatalf("xsiAttributeIdentityKey(noNamespaceSchemaLocation) error = %v", err)
	}
	if !identity.present || identity.name != noNamespaceSchemaLocationName || identity.key != ordinaryAnyURI.Identity {
		t.Fatalf("xsiAttributeIdentityKey(noNamespaceSchemaLocation) = %+v, want ordinary anyURI key %q", identity, ordinaryAnyURI.Identity)
	}
	emptyAnyURI, err := rt.ValidateSimpleValue(anyURI, "", nil, runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(empty anyURI) error = %v", err)
	}
	identity, err = xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNoNamespaceSchemaLocation}, " \t", nil, ctx)
	if err != nil {
		t.Fatalf("xsiAttributeIdentityKey(empty noNamespaceSchemaLocation) error = %v", err)
	}
	if !identity.present || identity.name != noNamespaceSchemaLocationName || identity.key != emptyAnyURI.Identity {
		t.Fatalf("xsiAttributeIdentityKey(empty noNamespaceSchemaLocation) = %+v, want ordinary empty anyURI key %q", identity, emptyAnyURI.Identity)
	}

	uriList := simpleTypeIDByNameForTest(t, rt, "", "URIs")
	ordinaryList, err := rt.ValidateSimpleValue(uriList, "urn:a a.xsd urn:b b.xsd", nil, runtime.SimpleNeedIdentity)
	if err != nil {
		t.Fatalf("ValidateSimpleValue(list<anyURI>) error = %v", err)
	}
	identity, err = xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrSchemaLocation}, " urn:a\ta.xsd\nurn:b  b.xsd ", nil, ctx)
	if err != nil {
		t.Fatalf("xsiAttributeIdentityKey(schemaLocation) error = %v", err)
	}
	if !identity.present || identity.name != schemaLocationName || identity.key != ordinaryList.Identity {
		t.Fatalf("xsiAttributeIdentityKey(schemaLocation) = %+v, want ordinary list<anyURI> key %q", identity, ordinaryList.Identity)
	}

	identity, err = xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: "other"}, " a\tb ", nil, ctx)
	if err != nil || identity != (xsiIdentityKey{}) {
		t.Fatalf("xsiAttributeIdentityKey(other) = %+v err %v, want ignored", identity, err)
	}
}

func TestXSIAttributeIdentityKeyErrors(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	ctx := StartContext{Line: 2, Column: 3, Path: "/root"}

	identity, err := xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: "unknown"}, "x", nil, ctx)
	if err != nil || identity != (xsiIdentityKey{}) {
		t.Fatalf("xsiAttributeIdentityKey(unknown) = %+v err %v, want ignored", identity, err)
	}

	identity, err = xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrNil}, "maybe", nil, ctx)
	if identity != (xsiIdentityKey{}) {
		t.Fatalf("xsiAttributeIdentityKey(invalid nil) = %+v, want empty error result", identity)
	}
	expectXSDCode(t, err, xsderrors.CodeValidationAttribute)

	identity, err = xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: vocab.XSIAttrType}, "bad", func(string) (string, string, bool) {
		return "", "", false
	}, ctx)
	if identity != (xsiIdentityKey{}) {
		t.Fatalf("xsiAttributeIdentityKey(invalid type) = %+v, want empty error result", identity)
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

			identity, err := xsiAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: test.local}, test.lexical, nil, ctx)
			if identity != (xsiIdentityKey{}) {
				t.Fatalf("xsiAttributeIdentityKey(invalid %s) = %+v, want empty error result", test.local, identity)
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
