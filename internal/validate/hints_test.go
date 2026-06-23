package validate

import (
	"encoding/xml"
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaLocationHintsRecordNamespacePairs(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "urn:a a.xsd\turn:b b.xsd", StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordAttribute() error = %v", err)
	}
	if !hints.Has("urn:a") || !hints.Has("urn:b") {
		t.Fatalf("RecordAttribute() did not retain schema-location namespaces")
	}
	if hints.Has("a.xsd") || hints.Has("b.xsd") {
		t.Fatalf("RecordAttribute() retained schema-location documents as namespaces")
	}
}

func TestSchemaLocationHintsRecordNoNamespace(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	if err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrNoNamespaceSchemaLocation), "\tno-ns.xsd\n", StartContext{Path: "/root", Line: 2, Column: 3}); err != nil {
		t.Fatalf("RecordAttribute() error = %v", err)
	}
	if !hints.Has("") {
		t.Fatalf("RecordAttribute() did not retain no-namespace schema-location hint")
	}
}

func TestSchemaLocationHintsRejectMalformedHints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		local   string
		value   string
		message string
	}{
		{
			name:    "schemaLocation odd field count",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "urn:t",
			message: "xsi:schemaLocation must contain namespace/location pairs",
		},
		{
			name:    "schemaLocation invalid URI",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "urn:t %zz",
			message: "invalid xsi:schemaLocation URI %zz",
		},
		{
			name:    "schemaLocation non XML whitespace",
			local:   vocab.XSIAttrSchemaLocation,
			value:   "urn:t\u00a0hinted.xsd",
			message: "xsi:schemaLocation must contain namespace/location pairs",
		},
		{
			name:    "noNamespace empty",
			local:   vocab.XSIAttrNoNamespaceSchemaLocation,
			value:   "",
			message: "xsi:noNamespaceSchemaLocation is empty",
		},
		{
			name:    "noNamespace XML whitespace",
			local:   vocab.XSIAttrNoNamespaceSchemaLocation,
			value:   " \t\n\r",
			message: "xsi:noNamespaceSchemaLocation is empty",
		},
		{
			name:    "noNamespace invalid URI",
			local:   vocab.XSIAttrNoNamespaceSchemaLocation,
			value:   "%zz",
			message: "invalid xsi:noNamespaceSchemaLocation URI %zz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var hints SchemaLocationHints
			err := hints.RecordAttribute(xsiHintName(tc.local), tc.value, StartContext{Path: "/root", Line: 2, Column: 3})
			expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
			expectXSDMessage(t, err, tc.message)
		})
	}
}

func TestSchemaLocationHintsRejectMalformedAtomically(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	err := hints.RecordAttribute(xsiHintName(vocab.XSIAttrSchemaLocation), "urn:t %zz", StartContext{Path: "/root", Line: 2, Column: 3})
	expectXSDCode(t, err, xsderrors.CodeValidationAttribute)
	if hints.Has("urn:t") {
		t.Fatalf("RecordAttribute() retained namespace from malformed schema-location hint")
	}
}

func TestSchemaLocationHintsRecordAttributesFiltersHints(t *testing.T) {
	t.Parallel()

	var hints SchemaLocationHints
	err := hints.RecordAttributes(hintAttrs(
		hintStreamAttr("", "id", "ignored"),
		hintStreamAttr("urn:other", vocab.XSIAttrSchemaLocation, "ignored"),
		hintStreamAttr(vocab.XSINamespaceURI, vocab.XSIAttrSchemaLocation, "urn:a a.xsd"),
		hintStreamAttr(vocab.XSINamespaceURI, vocab.XSIAttrNoNamespaceSchemaLocation, "no-ns.xsd"),
	), nil, StartContext{Path: "/root", Line: 2, Column: 3})
	if err != nil {
		t.Fatalf("RecordAttributes() error = %v", err)
	}
	if !hints.Has("urn:a") || !hints.Has("") {
		t.Fatalf("RecordAttributes() did not retain expected schema-location hints")
	}
	if hints.Has("urn:other") {
		t.Fatalf("RecordAttributes() retained non-XSI schema-location-like attribute")
	}
}

func TestSchemaLocationHintsResetClearsAndDropsOversizedMaps(t *testing.T) {
	t.Parallel()

	hints := SchemaLocationHints{namespaces: map[string]bool{"urn:a": true}}
	hints.Reset(1)
	if hints.namespaces == nil {
		t.Fatalf("Reset() dropped bounded namespace map")
	}
	if hints.Has("urn:a") {
		t.Fatalf("Reset() retained stale namespace")
	}

	hints.namespaces = map[string]bool{"urn:a": true, "urn:b": true}
	hints.Reset(1)
	if hints.namespaces != nil {
		t.Fatalf("Reset() retained oversized namespace map")
	}
}

func xsiHintName(local string) xml.Name {
	return xml.Name{Space: vocab.XSINamespaceURI, Local: local}
}

func hintStreamAttr(ns, local, value string) stream.Attr {
	return stream.Attr{Name: xml.Name{Space: ns, Local: local}, Value: value}
}

func hintAttrs(attrs ...stream.Attr) []stream.Attr {
	return attrs
}

func expectXSDMessage(t *testing.T, err error, message string) {
	t.Helper()
	var x *xsderrors.Error
	if !errors.As(err, &x) {
		t.Fatalf("error = %v, want *xsderrors.Error", err)
	}
	if x.Message != message {
		t.Fatalf("error message = %q, want %q", x.Message, message)
	}
}
