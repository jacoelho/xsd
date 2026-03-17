package semanticresolve

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestCollectConstraintElementsFromContentSharedTraversal(t *testing.T) {
	schema := parseIdentityTraversalSchema(t)
	root := schema.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil {
		t.Fatalf("root element not found")
	}
	ct, ok := root.Type.(*model.ComplexType)
	if !ok || ct == nil {
		t.Fatalf("root type = %T, want *model.ComplexType", root.Type)
	}

	elements := collectConstraintElementsFromContent(ct.Content())
	if len(elements) != 1 {
		t.Fatalf("constraint elements = %d, want 1", len(elements))
	}
	if elements[0].Name.Local != "inline" {
		t.Fatalf("unexpected traversal result: %s", elements[0].Name.Local)
	}
}

func TestCollectAllIdentityConstraintsDeterministic(t *testing.T) {
	schema := parseIdentityTraversalSchema(t)
	index := buildIterationIndex(schema)

	var first []string
	for i := range 5 {
		constraints := collectAllIdentityConstraintsWithIndex(schema, index)
		got := make([]string, len(constraints))
		for i, c := range constraints {
			got[i] = c.Name
		}
		if i == 0 {
			first = got
			continue
		}
		if strings.Join(got, ",") != strings.Join(first, ",") {
			t.Fatalf("constraint order changed: first=%v current=%v", first, got)
		}
	}
	if strings.Join(first, ",") != "rootKey,inlineKey,itemKey" {
		t.Fatalf("unexpected collected constraints: %v", first)
	}
}

func parseIdentityTraversalSchema(t *testing.T) *parser.Schema {
	t.Helper()

	const schemaXML = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:group name="shared">
    <xs:sequence>
      <xs:element name="item">
        <xs:key name="itemKey">
          <xs:selector xpath="."/>
          <xs:field xpath="@id"/>
        </xs:key>
      </xs:element>
    </xs:sequence>
  </xs:group>

  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:group ref="tns:shared"/>
        <xs:group ref="tns:shared"/>
        <xs:element name="inline">
          <xs:complexType>
            <xs:sequence>
              <xs:group ref="tns:shared"/>
            </xs:sequence>
          </xs:complexType>
          <xs:key name="inlineKey">
            <xs:selector xpath="."/>
            <xs:field xpath="@id"/>
          </xs:key>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="rootKey">
      <xs:selector xpath="tns:inline"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return schema
}
