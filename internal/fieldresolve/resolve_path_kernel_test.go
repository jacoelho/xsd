package fieldresolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xpath"
)

func TestResolvePathElementDeclDirectVsDescendant(t *testing.T) {
	schema := parseResolvePathSchema(t)
	root := schema.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil {
		t.Fatalf("root element not found")
	}
	ns := map[string]string{"tns": "urn:test"}

	t.Run("direct nested path succeeds", func(t *testing.T) {
		steps := mustXPathSteps(t, "tns:parent/tns:mid/tns:leaf", ns)
		decl, err := resolvePathElementDecl(schema, root, steps)
		if err != nil {
			t.Fatalf("resolvePathElementDecl() error = %v", err)
		}
		if decl == nil || decl.Name.Local != "leaf" {
			t.Fatalf("resolved decl = %v, want leaf", decl)
		}
	})

	t.Run("direct path without intermediate step fails", func(t *testing.T) {
		steps := mustXPathSteps(t, "tns:parent/tns:leaf", ns)
		if _, err := resolvePathElementDecl(schema, root, steps); err == nil {
			t.Fatalf("expected direct path resolution error")
		}
	})

	t.Run("descendant path resolves same target", func(t *testing.T) {
		steps := mustXPathSteps(t, "tns:parent//tns:leaf", ns)
		decl, err := resolvePathElementDecl(schema, root, steps)
		if err != nil {
			t.Fatalf("resolvePathElementDecl() descendant error = %v", err)
		}
		if decl == nil || decl.Name.Local != "leaf" {
			t.Fatalf("resolved decl = %v, want leaf", decl)
		}
	})
}

func TestFindElementDeclDescendantAbstractWrapsUnresolvable(t *testing.T) {
	schema := parseResolvePathSchema(t)
	start := schema.ElementDecls[model.QName{Namespace: "urn:test", Local: "abstractStart"}]
	if start == nil {
		t.Fatalf("abstractStart element not found")
	}

	decl, err := findElementDeclDescendant(schema, start, xpath.NodeTest{
		Local:              "missing",
		Namespace:          "urn:test",
		NamespaceSpecified: true,
	})
	if decl != nil {
		t.Fatalf("expected nil declaration for unresolved search")
	}
	if err == nil {
		t.Fatalf("expected unresolved error")
	}
	if !errors.Is(err, ErrXPathUnresolvable) {
		t.Fatalf("expected ErrXPathUnresolvable, got %v", err)
	}
}

func mustXPathSteps(t *testing.T, expr string, ns map[string]string) []xpath.Step {
	t.Helper()
	parsed, err := parseXPathExpression(expr, ns, xpath.AttributesDisallowed)
	if err != nil {
		t.Fatalf("parse xpath %q: %v", expr, err)
	}
	if len(parsed.Paths) != 1 {
		t.Fatalf("xpath %q paths = %d, want 1", expr, len(parsed.Paths))
	}
	return parsed.Paths[0].Steps
}

func parseResolvePathSchema(t *testing.T) *parser.Schema {
	t.Helper()

	const schemaXML = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="AbstractT" abstract="true">
    <xs:sequence>
      <xs:element name="known" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>

  <xs:element name="abstractStart" type="tns:AbstractT"/>

  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parent">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="mid">
                <xs:complexType>
                  <xs:sequence>
                    <xs:element name="leaf" type="xs:string"/>
                  </xs:sequence>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	return schema
}
