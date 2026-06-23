package xmlns

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
)

func TestStackPushRollsBackBindingsOnError(t *testing.T) {
	var ns Stack
	err := ns.Push([]xml.Attr{
		{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: "a"}, Value: "urn:a"},
		{Name: xml.Name{Space: vocab.XMLNSPrefix, Local: vocab.XMLPrefix}, Value: "urn:not-xml"},
	})
	if err == nil {
		t.Fatal("Push() error = nil")
	}
	if ns.FrameCapacity() != 0 {
		t.Fatalf("FrameCapacity() = %d, want 0", ns.FrameCapacity())
	}
	if got, ok := ns.Lookup("a"); ok || got != "" {
		t.Fatalf("Lookup(a) = %q, %v; want missing", got, ok)
	}
}

func TestStackResolvesDefaultNamespaceForElementsOnly(t *testing.T) {
	var ns Stack
	if err := ns.Push([]xml.Attr{{Name: xml.Name{Local: vocab.XMLNSPrefix}, Value: "urn:default"}}); err != nil {
		t.Fatalf("Push() error = %v", err)
	}
	elem, ok := ns.ResolveName(xml.Name{Local: "root"}, ElementName)
	if !ok || elem.Space != "urn:default" || elem.Local != "root" {
		t.Fatalf("ResolveName(element) = %+v, %v; want default namespace", elem, ok)
	}
	attr, ok := ns.ResolveName(xml.Name{Local: "attr"}, AttributeName)
	if !ok || attr.Space != "" || attr.Local != "attr" {
		t.Fatalf("ResolveName(attribute) = %+v, %v; want no namespace", attr, ok)
	}
}

func TestStackResetDropsOversizedCapacity(t *testing.T) {
	ns := NewStackWithCapacity(4, 4)
	ns.Reset(3)
	if ns.FrameCapacity() != 0 || ns.BindingCapacity() != 0 {
		t.Fatalf("Reset retained oversized capacities %d/%d", ns.FrameCapacity(), ns.BindingCapacity())
	}
}

func TestValidateUniqueAttributes(t *testing.T) {
	t.Parallel()

	err := ValidateUniqueAttributes(streamAttrs(
		xml.Name{Space: "urn:x", Local: "id"},
		xml.Name{Space: "urn:y", Local: "id"},
	))
	if err != nil {
		t.Fatalf("ValidateUniqueAttributes() error = %v", err)
	}

	err = ValidateUniqueAttributes(streamAttrs(
		xml.Name{Space: "urn:x", Local: "id"},
		xml.Name{Space: "urn:x", Local: "id"},
	))
	if err == nil || !strings.Contains(err.Error(), "duplicate attribute {urn:x}id") {
		t.Fatalf("ValidateUniqueAttributes() error = %v, want duplicate expanded name", err)
	}
}

func streamAttrs(names ...xml.Name) []stream.Attr {
	attrs := make([]stream.Attr, len(names))
	for i, name := range names {
		attrs[i].Name = name
	}
	return attrs
}
