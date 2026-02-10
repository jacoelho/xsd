package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
)

// TestCollectElementDeclarationsFromType tests the element collection logic
// to ensure it correctly collects elements from base types recursively.
func TestCollectElementDeclarationsFromType(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[model.QName]model.Type),
	}

	// base type with one element
	baseType := &model.ComplexType{
		QName: model.QName{Namespace: "http://example.com", Local: "BaseType"},
	}
	baseType.SetContent(&model.ElementContent{
		Particle: &model.ModelGroup{
			Kind: model.Sequence,
			Particles: []model.Particle{
				&model.ElementDecl{
					Name: model.QName{Namespace: "http://example.com", Local: "baseElement"},
					Type: builtins.Get(model.TypeNameString),
				},
			},
		},
	})

	// middle type extending base
	middleType := &model.ComplexType{
		QName: model.QName{Namespace: "http://example.com", Local: "MiddleType"},
	}
	middleType.SetContent(&model.ComplexContent{
		Extension: &model.Extension{
			Base: baseType.QName,
			Particle: &model.ModelGroup{
				Kind: model.Sequence,
				Particles: []model.Particle{
					&model.ElementDecl{
						Name: model.QName{Namespace: "http://example.com", Local: "middleElement"},
						Type: builtins.Get(model.TypeNameInteger),
					},
				},
			},
		},
	})
	middleType.DerivationMethod = model.DerivationExtension

	// extended type extending middle
	extendedType := &model.ComplexType{
		QName: model.QName{Namespace: "http://example.com", Local: "ExtendedType"},
	}
	extendedType.SetContent(&model.ComplexContent{
		Extension: &model.Extension{
			Base: middleType.QName,
			Particle: &model.ModelGroup{
				Kind: model.Sequence,
				Particles: []model.Particle{
					&model.ElementDecl{
						Name: model.QName{Namespace: "http://example.com", Local: "extendedElement"},
						Type: builtins.Get(model.TypeNameDate),
					},
				},
			},
		},
	})
	extendedType.DerivationMethod = model.DerivationExtension

	schema.TypeDefs[baseType.QName] = baseType
	schema.TypeDefs[middleType.QName] = middleType
	schema.TypeDefs[extendedType.QName] = extendedType

	// test collecting from extended type should get all elements
	elements := traversal.CollectElementDeclsFromComplexType(schema, extendedType)
	if len(elements) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(elements))
	}

	// verify element names
	names := make(map[string]bool)
	for _, elem := range elements {
		names[elem.Name.Local] = true
	}
	expectedNames := []string{"baseElement", "middleElement", "extendedElement"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("Expected element %s not found in collection", name)
		}
	}
}
