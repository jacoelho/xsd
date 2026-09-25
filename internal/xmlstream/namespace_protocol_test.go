package xmlstream

import (
	"encoding/xml"
	"testing"
)

func TestNamespaceMatchEndRestoresShadowedBinding(t *testing.T) {
	var namespaceStack stack
	var values cache

	rootStart := StartElement{
		Name: xml.Name{Space: "p", Local: "root"},
		Attr: []Attr{{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:root"}},
	}
	rootFrame, root, err := namespaceStack.StartStream(&rootStart, &values)
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != (xml.Name{Space: "urn:root", Local: "root"}) {
		t.Fatalf("root expanded name = %+v", root.Name)
	}

	childStart := StartElement{
		Name: xml.Name{Space: "p", Local: "child"},
		Attr: []Attr{{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:child"}},
	}
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
	start := StartElement{Name: xml.Name{Local: "root"}}
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

func TestNamespaceDefaultLookupUsesShadowAndRollback(t *testing.T) {
	var namespaceStack stack
	var values cache

	rootStart := StartElement{
		Name: xml.Name{Local: "root"},
		Attr: []Attr{{Name: xml.Name{Local: "xmlns"}, Value: "urn:root"}},
	}
	rootFrame, root, err := namespaceStack.StartStream(&rootStart, &values)
	if err != nil {
		t.Fatal(err)
	}
	if root.Name != (xml.Name{Space: "urn:root", Local: "root"}) {
		t.Fatalf("root expanded name = %+v", root.Name)
	}
	if uri, ok := namespaceStack.Lookup(""); !ok || uri != "urn:root" {
		t.Fatalf("root default namespace = %q, %v", uri, ok)
	}

	childStart := StartElement{
		Name: xml.Name{Local: "child"},
		Attr: []Attr{{Name: xml.Name{Local: "xmlns"}, Value: "urn:child"}},
	}
	childFrame, child, err := namespaceStack.StartStream(&childStart, &values)
	if err != nil {
		t.Fatal(err)
	}
	if child.Name != (xml.Name{Space: "urn:child", Local: "child"}) {
		t.Fatalf("child expanded name = %+v", child.Name)
	}
	if uri, ok := namespaceStack.Lookup(""); !ok || uri != "urn:child" {
		t.Fatalf("child default namespace = %q, %v", uri, ok)
	}
	if err := namespaceStack.MatchEnd(childFrame, LexicalName{Local: "wrong"}); err == nil {
		t.Fatal("MatchEnd accepted a mismatched default-namespaced element")
	}
	if uri, ok := namespaceStack.Lookup(""); !ok || uri != "urn:child" {
		t.Fatalf("default namespace after failed end match = %q, %v", uri, ok)
	}
	if err := namespaceStack.End(childFrame, Lexical(childStart.Name)); err != nil {
		t.Fatal(err)
	}
	if uri, ok := namespaceStack.Lookup(""); !ok || uri != "urn:root" {
		t.Fatalf("restored default namespace = %q, %v", uri, ok)
	}

	failedStart := StartElement{
		Name: xml.Name{Local: "failed"},
		Attr: []Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: "urn:failed"},
			{Name: xml.Name{Local: "x"}, Value: "1"},
			{Name: xml.Name{Local: "x"}, Value: "2"},
		},
	}
	if _, _, err := namespaceStack.StartStream(&failedStart, &values); err == nil {
		t.Fatal("StartStream accepted duplicate attributes")
	}
	if uri, ok := namespaceStack.Lookup(""); !ok || uri != "urn:root" {
		t.Fatalf("default namespace after failed start = %q, %v", uri, ok)
	}
	if err := namespaceStack.End(rootFrame, Lexical(rootStart.Name)); err != nil {
		t.Fatal(err)
	}
}

func TestNamespaceContextRetainsDefaultBindingAfterStackMutation(t *testing.T) {
	var namespaceStack stack
	var values cache
	rootStart := StartElement{
		Name: xml.Name{Local: "root"},
		Attr: []Attr{{Name: xml.Name{Local: "xmlns"}, Value: "urn:root"}},
	}
	rootFrame, _, err := namespaceStack.StartStream(&rootStart, &values)
	if err != nil {
		t.Fatal(err)
	}
	context := namespaceStack.Context()
	childStart := StartElement{
		Name: xml.Name{Local: "child"},
		Attr: []Attr{{Name: xml.Name{Local: "xmlns"}, Value: "urn:child"}},
	}
	childFrame, _, err := namespaceStack.StartStream(&childStart, &values)
	if err != nil {
		t.Fatal(err)
	}
	if err := namespaceStack.End(childFrame, Lexical(childStart.Name)); err != nil {
		t.Fatal(err)
	}
	if uri, ok := context.Lookup(""); !ok || uri != "urn:root" {
		t.Fatalf("captured default namespace = %q, %v", uri, ok)
	}
	if err := namespaceStack.End(rootFrame, Lexical(rootStart.Name)); err != nil {
		t.Fatal(err)
	}
}
