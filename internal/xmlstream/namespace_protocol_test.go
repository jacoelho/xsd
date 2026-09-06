package xmlstream

import (
	"encoding/xml"
	"testing"
)

func TestNamespaceMatchEndRestoresShadowedBinding(t *testing.T) {
	var namespaceStack stack
	var values cache

	rootStart := OwnedStartElement(
		xml.Name{Space: "p", Local: "root"},
		OwnedAttr(xml.Name{Space: "xmlns", Local: "p"}, "urn:root"),
	)
	rootFrame, root, err := namespaceStack.StartStream(&rootStart, &values)
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != (xml.Name{Space: "urn:root", Local: "root"}) {
		t.Fatalf("root expanded name = %+v", root.Name)
	}

	childStart := OwnedStartElement(
		xml.Name{Space: "p", Local: "child"},
		OwnedAttr(xml.Name{Space: "xmlns", Local: "p"}, "urn:child"),
	)
	childFrame, child, err := namespaceStack.StartStream(&childStart, &values)
	if err != nil {
		t.Fatal(err)
	}
	if child.Name != (xml.Name{Space: "urn:child", Local: "child"}) {
		t.Fatalf("child expanded name = %+v", child.Name)
	}
	if err := namespaceStack.End(childFrame, Lexical(childStart.Name)); err != nil {
		t.Fatal(err)
	}
	if uri, ok := namespaceStack.Lookup("p"); !ok || uri != "urn:root" {
		t.Fatalf("restored p binding = %q, %v", uri, ok)
	}
	if err := namespaceStack.End(rootFrame, Lexical(rootStart.Name)); err != nil {
		t.Fatal(err)
	}
}

func TestNamespaceMatchEndRetryAfterMismatch(t *testing.T) {
	var namespaceStack stack
	var values cache
	start := OwnedStartElement(xml.Name{Local: "root"})
	frame, _, err := namespaceStack.StartStream(&start, &values)
	if err != nil {
		t.Fatal(err)
	}

	if err := namespaceStack.MatchEnd(frame, LexicalName{Local: "wrong"}); err == nil {
		t.Fatal("MatchEnd accepted a mismatched lexical name")
	}
	if namespaceStack.depth() != 1 {
		t.Fatalf("depth after failed MatchEnd = %d, want 1", namespaceStack.depth())
	}
	if err := namespaceStack.End(frame, Lexical(start.Name)); err != nil {
		t.Fatal(err)
	}
	if namespaceStack.depth() != 0 {
		t.Fatalf("depth after retry = %d, want 0", namespaceStack.depth())
	}
}
