package schemacheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// TestCollectElementDeclarationsFromType tests the element collection logic
// to ensure it correctly collects elements from base types recursively.
func TestCollectElementDeclarationsFromType(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type with one element
	baseType := &types.ComplexType{
		QName: types.QName{Namespace: "http://example.com", Local: "BaseType"},
	}
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Namespace: "http://example.com", Local: "baseElement"},
					Type: types.GetBuiltin(types.TypeNameString),
				},
			},
		},
	})

	// middle type extending base
	middleType := &types.ComplexType{
		QName: types.QName{Namespace: "http://example.com", Local: "MiddleType"},
	}
	middleType.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: baseType.QName,
			Particle: &types.ModelGroup{
				Kind: types.Sequence,
				Particles: []types.Particle{
					&types.ElementDecl{
						Name: types.QName{Namespace: "http://example.com", Local: "middleElement"},
						Type: types.GetBuiltin(types.TypeNameInteger),
					},
				},
			},
		},
	})
	middleType.DerivationMethod = types.DerivationExtension

	// extended type extending middle
	extendedType := &types.ComplexType{
		QName: types.QName{Namespace: "http://example.com", Local: "ExtendedType"},
	}
	extendedType.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: middleType.QName,
			Particle: &types.ModelGroup{
				Kind: types.Sequence,
				Particles: []types.Particle{
					&types.ElementDecl{
						Name: types.QName{Namespace: "http://example.com", Local: "extendedElement"},
						Type: types.GetBuiltin(types.TypeNameDate),
					},
				},
			},
		},
	})
	extendedType.DerivationMethod = types.DerivationExtension

	schema.TypeDefs[baseType.QName] = baseType
	schema.TypeDefs[middleType.QName] = middleType
	schema.TypeDefs[extendedType.QName] = extendedType

	// test collecting from extended type should get all elements
	elements := collectAllElementDeclarationsFromType(schema, extendedType)
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
