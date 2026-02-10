package schemaxml

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestParseSubtreeIntoPreservesInScopeNamespacesAndConsumesSubtree(t *testing.T) {
	reader, err := xmlstream.NewReader(strings.NewReader(
		`<root xmlns="urn:root" xmlns:p="urn:p"><item attr="p:name"><inner/></item><tail/></root>`,
	))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatalf("Next() root error = %v", err)
	}
	start, err := reader.Next()
	if err != nil {
		t.Fatalf("Next() item error = %v", err)
	}
	if start.Kind != xmlstream.EventStartElement || start.Name.Local != "item" {
		t.Fatalf("start event = %+v, want item start", start)
	}

	pool := NewDocumentPool()
	doc := pool.Acquire()
	defer pool.Release(doc)

	if err := ParseSubtreeInto(reader, start, doc); err != nil {
		t.Fatalf("ParseSubtreeInto() error = %v", err)
	}

	root := doc.DocumentElement()
	if root == InvalidNode {
		t.Fatal("DocumentElement() = InvalidNode")
	}
	if got := doc.LocalName(root); got != "item" {
		t.Fatalf("subtree root local = %q, want item", got)
	}
	if got := doc.GetAttribute(root, "attr"); got != "p:name" {
		t.Fatalf("subtree attr value = %q, want p:name", got)
	}

	foundDefault := false
	foundPrefix := false
	for _, attr := range doc.Attributes(root) {
		if attr.NamespaceURI() != XMLNSNamespace {
			continue
		}
		if attr.LocalName() == "xmlns" && attr.Value() == "urn:root" {
			foundDefault = true
		}
		if attr.LocalName() == "p" && attr.Value() == "urn:p" {
			foundPrefix = true
		}
	}
	if !foundDefault || !foundPrefix {
		t.Fatalf("missing in-scope xmlns attrs: default=%v prefix=%v", foundDefault, foundPrefix)
	}

	next, err := reader.Next()
	if err != nil {
		t.Fatalf("Next() after subtree error = %v", err)
	}
	if next.Kind != xmlstream.EventStartElement || next.Name.Local != "tail" {
		t.Fatalf("next event after subtree = %+v, want tail start", next)
	}
}

func TestParseSubtreeIntoRejectsNonStartEvent(t *testing.T) {
	reader, err := xmlstream.NewReader(strings.NewReader(`<root/>`))
	if err != nil {
		t.Fatalf("NewReader() error = %v", err)
	}
	doc := &Document{}
	if err := ParseIntoWithOptions(strings.NewReader(`<a/>`), doc); err != nil {
		t.Fatalf("ParseIntoWithOptions() seed error = %v", err)
	}
	if doc.DocumentElement() == InvalidNode {
		t.Fatal("expected seeded document root")
	}

	err = ParseSubtreeInto(reader, xmlstream.Event{Kind: xmlstream.EventCharData}, doc)
	if err == nil {
		t.Fatal("ParseSubtreeInto() error = nil, want error")
	}
	if doc.DocumentElement() != InvalidNode {
		t.Fatal("expected ParseSubtreeInto() to reset document on error")
	}
}
