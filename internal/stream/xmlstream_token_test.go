package stream

import (
	"encoding/xml"
	"testing"
)

func TestOwnedAttrsMaterializesAndDetachesBorrowedValues(t *testing.T) {
	raw := []byte("borrowed")
	source := []Attr{{Name: xml.Name{Local: "value"}, raw: raw}}

	owned := OwnedAttrs(source...)
	if len(owned) != 1 || owned[0].Value != "borrowed" || owned[0].HasBorrowedValue() {
		t.Fatalf("OwnedAttrs() = %+v, want one materialized attribute", owned)
	}

	raw[0] = 'X'
	source[0] = Attr{Name: xml.Name{Local: "changed"}, Value: "changed"}
	if owned[0].Name.Local != "value" || owned[0].Value != "borrowed" {
		t.Fatalf("OwnedAttrs() retained source storage: %+v", owned[0])
	}
}

func TestOwnedStartElementDetachesAttributes(t *testing.T) {
	raw := []byte("borrowed")
	attrs := []Attr{{Name: xml.Name{Local: "value"}, raw: raw}}

	start := OwnedStartElement(xml.Name{Local: "root"}, attrs...)
	raw[0] = 'X'
	attrs[0] = Attr{Name: xml.Name{Local: "changed"}, Value: "changed"}

	if start.Name.Local != "root" || len(start.Attr) != 1 {
		t.Fatalf("OwnedStartElement() = %+v", start)
	}
	if start.Attr[0].Name.Local != "value" || start.Attr[0].Value != "borrowed" || start.Attr[0].HasBorrowedValue() {
		t.Fatalf("OwnedStartElement() retained borrowed attribute storage: %+v", start.Attr[0])
	}
}
