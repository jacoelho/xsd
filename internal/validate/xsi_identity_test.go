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

	nilName := runtime.QName{Namespace: 1, Local: 1}
	typeName := runtime.QName{Namespace: 1, Local: 2}
	otherName := runtime.QName{Namespace: 1, Local: 3}
	rt := startRuntimeStub{
		names: map[expandedName]runtime.QName{
			{ns: vocab.XSINamespaceURI, local: vocab.XSIAttrNil}:  nilName,
			{ns: vocab.XSINamespaceURI, local: vocab.XSIAttrType}: typeName,
			{ns: vocab.XSINamespaceURI, local: "other"}:           otherName,
		},
	}
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

	name, key, ok, err = XSIAttributeIdentityKey(rt, xml.Name{Space: vocab.XSINamespaceURI, Local: "other"}, " a\tb ", nil, ctx)
	if err != nil {
		t.Fatalf("XSIAttributeIdentityKey(other) error = %v", err)
	}
	if !ok || name != otherName || key != runtime.SimpleIdentityKey(runtime.PrimitiveString, "a b") {
		t.Fatalf("XSIAttributeIdentityKey(other) = %v %q %v, want collapsed string", name, key, ok)
	}
}

func TestXSIAttributeIdentityKeyErrors(t *testing.T) {
	t.Parallel()

	nilName := runtime.QName{Namespace: 1, Local: 1}
	typeName := runtime.QName{Namespace: 1, Local: 2}
	rt := startRuntimeStub{
		names: map[expandedName]runtime.QName{
			{ns: vocab.XSINamespaceURI, local: vocab.XSIAttrNil}:  nilName,
			{ns: vocab.XSINamespaceURI, local: vocab.XSIAttrType}: typeName,
		},
	}
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
}
