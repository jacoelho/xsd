package parser

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func nextStartElement(t *testing.T, r *xmlstream.Reader) xmlstream.Event {
	t.Helper()
	for {
		ev, err := r.Next()
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		if ev.Kind == xmlstream.EventStartElement {
			return ev
		}
	}
}

func TestSchemaAttributesParityFromStartAndDocument(t *testing.T) {
	const schemaXML = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns="urn:default"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified"
           attributeFormDefault="unqualified"
           blockDefault="extension restriction"
           finalDefault="list union"/>`

	doc, root := parseSchemaDoc(t, schemaXML)
	docSchema := NewSchema()
	if err := parseSchemaAttributes(doc, root, docSchema); err != nil {
		t.Fatalf("parseSchemaAttributes() error = %v", err)
	}

	reader, err := xmlstream.NewReader(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	start := nextStartElement(t, reader)

	startSchema := NewSchema()
	if err := parseSchemaAttributesFromStart(start, reader.NamespaceDeclsSeq(start.ScopeDepth), startSchema); err != nil {
		t.Fatalf("parseSchemaAttributesFromStart() error = %v", err)
	}

	if docSchema.TargetNamespace != startSchema.TargetNamespace {
		t.Fatalf("TargetNamespace mismatch: doc=%q start=%q", docSchema.TargetNamespace, startSchema.TargetNamespace)
	}
	if docSchema.ElementFormDefault != startSchema.ElementFormDefault {
		t.Fatalf("ElementFormDefault mismatch: doc=%v start=%v", docSchema.ElementFormDefault, startSchema.ElementFormDefault)
	}
	if docSchema.AttributeFormDefault != startSchema.AttributeFormDefault {
		t.Fatalf("AttributeFormDefault mismatch: doc=%v start=%v", docSchema.AttributeFormDefault, startSchema.AttributeFormDefault)
	}
	if docSchema.BlockDefault != startSchema.BlockDefault {
		t.Fatalf("BlockDefault mismatch: doc=%v start=%v", docSchema.BlockDefault, startSchema.BlockDefault)
	}
	if docSchema.FinalDefault != startSchema.FinalDefault {
		t.Fatalf("FinalDefault mismatch: doc=%v start=%v", docSchema.FinalDefault, startSchema.FinalDefault)
	}
	if len(docSchema.NamespaceDecls) != len(startSchema.NamespaceDecls) {
		t.Fatalf("NamespaceDecls length mismatch: doc=%d start=%d", len(docSchema.NamespaceDecls), len(startSchema.NamespaceDecls))
	}
	for prefix, uri := range docSchema.NamespaceDecls {
		if got := startSchema.NamespaceDecls[prefix]; got != uri {
			t.Fatalf("NamespaceDecls[%q] mismatch: doc=%q start=%q", prefix, uri, got)
		}
	}
}

func TestSchemaAttributesParityPrefixedTargetNamespaceError(t *testing.T) {
	const schemaXML = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xs:targetNamespace="urn:test"/>`

	doc, root := parseSchemaDoc(t, schemaXML)
	docSchema := NewSchema()
	docErr := parseSchemaAttributes(doc, root, docSchema)
	if docErr == nil {
		t.Fatalf("parseSchemaAttributes() expected error")
	}

	reader, err := xmlstream.NewReader(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	start := nextStartElement(t, reader)

	startSchema := NewSchema()
	startErr := parseSchemaAttributesFromStart(start, reader.NamespaceDeclsSeq(start.ScopeDepth), startSchema)
	if startErr == nil {
		t.Fatalf("parseSchemaAttributesFromStart() expected error")
	}
	if docErr.Error() != startErr.Error() {
		t.Fatalf("error mismatch: doc=%q start=%q", docErr.Error(), startErr.Error())
	}
}
