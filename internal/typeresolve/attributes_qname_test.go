package typeresolve

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestEffectiveAttributeQName(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace:      "urn:schema",
		AttributeFormDefault: parser.Unqualified,
	}

	t.Run("nil attribute", func(t *testing.T) {
		if got := EffectiveAttributeQName(schema, nil); got != (model.QName{}) {
			t.Fatalf("got %v, want zero QName", got)
		}
	})

	t.Run("reference uses declared name", func(t *testing.T) {
		attr := &model.AttributeDecl{
			IsReference: true,
			Name:        model.QName{Namespace: "urn:ref", Local: "a"},
			Form:        model.FormQualified,
		}
		if got := EffectiveAttributeQName(schema, attr); got != attr.Name {
			t.Fatalf("got %v, want %v", got, attr.Name)
		}
	})

	t.Run("qualified default uses schema namespace", func(t *testing.T) {
		schema.AttributeFormDefault = parser.Qualified
		attr := &model.AttributeDecl{
			Name: model.QName{Local: "a"},
			Form: model.FormDefault,
		}
		want := model.QName{Namespace: "urn:schema", Local: "a"}
		if got := EffectiveAttributeQName(schema, attr); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("source namespace overrides schema namespace", func(t *testing.T) {
		attr := &model.AttributeDecl{
			Name:            model.QName{Local: "a"},
			Form:            model.FormQualified,
			SourceNamespace: "urn:source",
		}
		want := model.QName{Namespace: "urn:source", Local: "a"}
		if got := EffectiveAttributeQName(schema, attr); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("nil schema keeps qualified attributes in source namespace only", func(t *testing.T) {
		attr := &model.AttributeDecl{
			Name: model.QName{Local: "a"},
			Form: model.FormQualified,
		}
		want := model.QName{Namespace: model.NamespaceEmpty, Local: "a"}
		if got := EffectiveAttributeQName(nil, attr); got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	})
}
