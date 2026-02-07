package typeops

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestEffectiveAttributeQName(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace:      "urn:schema",
		AttributeFormDefault: parser.Unqualified,
	}

	t.Run("nil attribute", func(t *testing.T) {
		if got := EffectiveAttributeQName(schema, nil); got != (types.QName{}) {
			t.Fatalf("got %v, want zero QName", got)
		}
	})

	t.Run("reference uses declared name", func(t *testing.T) {
		attr := &types.AttributeDecl{
			IsReference: true,
			Name:        types.QName{Namespace: "urn:ref", Local: "a"},
			Form:        types.FormQualified,
		}
		if got := EffectiveAttributeQName(schema, attr); got != attr.Name {
			t.Fatalf("got %v, want %v", got, attr.Name)
		}
	})

	t.Run("qualified default uses schema namespace", func(t *testing.T) {
		schema.AttributeFormDefault = parser.Qualified
		attr := &types.AttributeDecl{
			Name: types.QName{Local: "a"},
			Form: types.FormDefault,
		}
		want := types.QName{Namespace: "urn:schema", Local: "a"}
		if got := EffectiveAttributeQName(schema, attr); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("source namespace overrides schema namespace", func(t *testing.T) {
		attr := &types.AttributeDecl{
			Name:            types.QName{Local: "a"},
			Form:            types.FormQualified,
			SourceNamespace: "urn:source",
		}
		want := types.QName{Namespace: "urn:source", Local: "a"}
		if got := EffectiveAttributeQName(schema, attr); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("nil schema keeps qualified attributes in source namespace only", func(t *testing.T) {
		attr := &types.AttributeDecl{
			Name: types.QName{Local: "a"},
			Form: types.FormQualified,
		}
		want := types.QName{Namespace: types.NamespaceEmpty, Local: "a"}
		if got := EffectiveAttributeQName(nil, attr); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}
