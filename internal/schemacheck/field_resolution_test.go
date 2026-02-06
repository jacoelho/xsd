package schemacheck

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveFieldTypeMixedContent(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"
	schema.NamespaceDecls["tns"] = "urn:field"

	mixedType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "mixedType"}, "urn:field")
	mixedType.SetContent(&types.EmptyContent{})
	mixedType.SetMixed(true)

	item := &types.ElementDecl{
		Name: types.QName{Namespace: "urn:field", Local: "item"},
		Type: mixedType,
	}

	rootType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&types.ElementContent{Particle: item})
	root := &types.ElementDecl{Name: types.QName{Namespace: "urn:field", Local: "root"}, Type: rootType}

	field := &types.Field{XPath: "."}
	_, err := ResolveFieldType(schema, field, root, "tns:item", schema.NamespaceDecls)
	if !errors.Is(err, ErrFieldSelectsComplexContent) {
		t.Fatalf("expected ErrFieldSelectsComplexContent, got %v", err)
	}
}

func TestResolveSelectorElementTypeUnionMissingBranch(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"
	schema.NamespaceDecls["tns"] = "urn:field"

	child := &types.ElementDecl{
		Name: types.QName{Namespace: "urn:field", Local: "a"},
		Type: types.GetBuiltin(types.TypeName("string")),
	}
	rootType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&types.ElementContent{Particle: child})
	root := &types.ElementDecl{Name: types.QName{Namespace: "urn:field", Local: "root"}, Type: rootType}

	if _, err := ResolveSelectorElementType(schema, root, "tns:a | tns:missing", schema.NamespaceDecls); err == nil {
		t.Fatalf("expected selector union missing branch error")
	}
}

func TestResolveFieldTypeUnionComplexContent(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:field"
	schema.NamespaceDecls["tns"] = "urn:field"

	simple := &types.ElementDecl{
		Name: types.QName{Namespace: "urn:field", Local: "simple"},
		Type: types.GetBuiltin(types.TypeName("string")),
	}
	complexType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "complexType"}, "urn:field")
	complexType.SetContent(&types.EmptyContent{})
	complexElem := &types.ElementDecl{
		Name: types.QName{Namespace: "urn:field", Local: "complex"},
		Type: complexType,
	}

	containerType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "containerType"}, "urn:field")
	containerType.SetContent(&types.ElementContent{Particle: &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{simple, complexElem},
	}})
	container := &types.ElementDecl{
		Name: types.QName{Namespace: "urn:field", Local: "container"},
		Type: containerType,
	}

	rootType := types.NewComplexType(types.QName{Namespace: "urn:field", Local: "rootType"}, "urn:field")
	rootType.SetContent(&types.ElementContent{Particle: container})
	root := &types.ElementDecl{Name: types.QName{Namespace: "urn:field", Local: "root"}, Type: rootType}

	field := &types.Field{XPath: "tns:simple | tns:complex"}
	_, err := ResolveFieldType(schema, field, root, "tns:container", schema.NamespaceDecls)
	if !errors.Is(err, ErrXPathUnresolvable) {
		t.Fatalf("expected ErrXPathUnresolvable, got %v", err)
	}
}
