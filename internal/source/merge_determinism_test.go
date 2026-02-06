package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestMergeOriginsDeterministic(t *testing.T) {
	rootSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           xmlns:tns="urn:root"
           elementFormDefault="qualified">
  <xs:include schemaLocation="b.xsd"/>
  <xs:element name="e" type="xs:string"/>
</xs:schema>`

	includeSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:root"
           elementFormDefault="qualified">
  <xs:element name="e" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(rootSchema)},
		"b.xsd": &fstest.MapFile{Data: []byte(includeSchema)},
	}
	loader := NewLoader(Config{FS: fs})
	schema, err := loader.Load("a.xsd")
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}

	qname := types.QName{Namespace: "urn:root", Local: "e"}
	origin := schema.ElementOrigins[qname]
	if parser.ImportContextLocation(origin) != "a.xsd" {
		t.Fatalf("origin = %q, want root schema location", origin)
	}
}
