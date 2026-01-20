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
	_, err := resolveFieldType(schema, field, root, "tns:item", schema.NamespaceDecls)
	if !errors.Is(err, ErrFieldSelectsComplexContent) {
		t.Fatalf("expected ErrFieldSelectsComplexContent, got %v", err)
	}
}
