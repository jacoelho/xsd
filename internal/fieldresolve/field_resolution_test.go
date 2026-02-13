package fieldresolve

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
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
		Type: builtins.Get(model.TypeName("string")),
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
		Type: builtins.Get(model.TypeName("string")),
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
		Type: builtins.Get(model.TypeName("string")),
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
