package semantics

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xpath"
)

func TestResolveFieldTypeMixedContent(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"
	schema.NamespaceDecls["tns"] = "urn:field"

	mixedType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "mixedType"}, "urn:field")
	mixedType.SetContent(&model.EmptyContent{})
	mixedType.SetMixed(true)

	item := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "item"},
		Type: mixedType,
	}

	rootType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&model.ElementContent{Particle: item})
	root := &model.ElementDecl{Name: model.QName{Namespace: "urn:field", Local: "root"}, Type: rootType}

	field := &model.Field{XPath: "."}
	_, err := ResolveFieldType(schema, field, root, "tns:item", schema.NamespaceDecls)
	if !errors.Is(err, ErrFieldSelectsComplexContent) {
		t.Fatalf("expected ErrFieldSelectsComplexContent, got %v", err)
	}
}

func TestResolveSelectorElementTypeUnionMissingBranch(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"
	schema.NamespaceDecls["tns"] = "urn:field"

	child := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "a"},
		Type: model.GetBuiltin(model.TypeName("string")),
	}
	rootType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&model.ElementContent{Particle: child})
	root := &model.ElementDecl{Name: model.QName{Namespace: "urn:field", Local: "root"}, Type: rootType}

	if _, err := ResolveSelectorElementType(schema, root, "tns:a | tns:missing", schema.NamespaceDecls); err == nil {
		t.Fatalf("expected selector union missing branch error")
	} else if !strings.Contains(err.Error(), "branch 2") {
		t.Fatalf("expected selector error to include branch index, got %v", err)
	}
}

func TestResolveFieldTypeUnionComplexContent(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"
	schema.NamespaceDecls["tns"] = "urn:field"

	simple := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "simple"},
		Type: model.GetBuiltin(model.TypeName("string")),
	}
	complexType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "complexType"}, "urn:field")
	complexType.SetContent(&model.EmptyContent{})
	complexElem := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "complex"},
		Type: complexType,
	}

	containerType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "containerType"}, "urn:field")
	containerType.SetContent(&model.ElementContent{Particle: &model.ModelGroup{
		Kind:      model.Sequence,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
		Particles: []model.Particle{simple, complexElem},
	}})
	container := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "container"},
		Type: containerType,
	}

	rootType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&model.ElementContent{Particle: container})
	root := &model.ElementDecl{Name: model.QName{Namespace: "urn:field", Local: "root"}, Type: rootType}

	field := &model.Field{XPath: "tns:simple | tns:complex"}
	_, err := ResolveFieldType(schema, field, root, "tns:container", schema.NamespaceDecls)
	if !errors.Is(err, ErrXPathUnresolvable) {
		t.Fatalf("expected ErrXPathUnresolvable, got %v", err)
	}
}

func TestResolveFieldElementDeclBranchIndexDiagnostics(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"
	schema.NamespaceDecls["tns"] = "urn:field"

	item := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "item"},
		Type: model.GetBuiltin(model.TypeName("string")),
	}
	containerType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "containerType"}, "urn:field")
	containerType.SetContent(&model.ElementContent{Particle: item})
	container := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "container"},
		Type: containerType,
	}
	rootType := model.NewComplexType(model.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&model.ElementContent{Particle: container})
	root := &model.ElementDecl{
		Name: model.QName{Namespace: "urn:field", Local: "root"},
		Type: rootType,
	}

	field := &model.Field{XPath: "tns:item | tns:missing"}
	_, err := ResolveFieldElementDecl(schema, field, root, "tns:container", schema.NamespaceDecls)
	if err == nil {
		t.Fatalf("expected branch-specific resolution error")
	}
	if !strings.Contains(err.Error(), "branch 2") {
		t.Fatalf("expected error to include branch index, got %v", err)
	}
}

func TestUnprefixedNodeTestMatchesNoNamespace(t *testing.T) {
	expr, err := xpath.Parse("item", nil, xpath.AttributesDisallowed)
	if err != nil {
		t.Fatalf("parse xpath: %v", err)
	}
	if len(expr.Paths) == 0 || len(expr.Paths[0].Steps) == 0 {
		t.Fatalf("expected parsed xpath steps")
	}
	test := expr.Paths[0].Steps[0].Test
	if nodeTestMatchesQName(test, model.QName{Namespace: "urn:test", Local: "item"}) {
		t.Fatalf("unprefixed node test should not match namespaced element")
	}
	if !nodeTestMatchesQName(test, model.QName{Namespace: model.NamespaceEmpty, Local: "item"}) {
		t.Fatalf("unprefixed node test should match no-namespace element")
	}
}

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
