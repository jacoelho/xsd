package xmltree

import "testing"

func TestDocumentAttributeAccessNilDocument(t *testing.T) {
	var d *Document
	if got := d.GetAttribute(InvalidNode, "attr"); got != "" {
		t.Fatalf("GetAttribute() = %q, want empty", got)
	}
	if got := d.GetAttributeNS(InvalidNode, "urn:test", "attr"); got != "" {
		t.Fatalf("GetAttributeNS() = %q, want empty", got)
	}
	if d.HasAttribute(InvalidNode, "attr") {
		t.Fatalf("HasAttribute() = true, want false")
	}
	if d.HasAttributeNS(InvalidNode, "urn:test", "attr") {
		t.Fatalf("HasAttributeNS() = true, want false")
	}
}

func TestDocumentAttributeAccessInvalidNode(t *testing.T) {
	d := &Document{}
	if got := d.GetAttribute(0, "attr"); got != "" {
		t.Fatalf("GetAttribute() = %q, want empty", got)
	}
	if got := d.GetAttributeNS(0, "urn:test", "attr"); got != "" {
		t.Fatalf("GetAttributeNS() = %q, want empty", got)
	}
	if d.HasAttribute(0, "attr") {
		t.Fatalf("HasAttribute() = true, want false")
	}
	if d.HasAttributeNS(0, "urn:test", "attr") {
		t.Fatalf("HasAttributeNS() = true, want false")
	}
}
