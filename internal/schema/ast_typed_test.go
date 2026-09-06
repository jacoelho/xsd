package schema

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xmlstream"
)

func TestTypedParserDoesNotRetainGenericSyntaxTree(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaDocument("typed.xsd", "typed.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation><xs:appinfo><vendor:payload xmlns:vendor="urn:vendor"><vendor:item/></vendor:payload></xs:appinfo></xs:annotation>
  <xs:simpleType name="value"><xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
</xs:schema>`), limits, new(xmlstream.Reader))
	if err != nil {
		t.Fatal(err)
	}
	if doc.root == nil || len(doc.rootChildren) != 2 {
		t.Fatalf("root children = %d, want annotation and simpleType", len(doc.rootChildren))
	}
	var restriction *schemaNode
	for _, id := range doc.rootChildren {
		n := doc.node(id)
		if n != nil && n.local == simpleTypeChild {
			restriction = doc.node(n.children[0])
		}
	}
	if restriction == nil || restriction.kind == schemaKindForeign || restriction.local != restrictionChild {
		t.Fatalf("restriction node = %#v; want retained admitted XSD node", restriction)
	}
	if got := len(doc.arena); got != 5 {
		t.Fatalf("retained semantic nodes = %d, want root, annotation, simpleType, restriction, facet", got)
	}
	annotation := doc.node(doc.rootChildren[0])
	if annotation == nil || len(annotation.children) != 0 {
		t.Fatal("annotation payload was retained as a semantic child")
	}
}
