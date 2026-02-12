package traversal

import (
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

// CollectElementDeclsFromComplexType collects element declarations in a complex
// type content model and recursively through its complex base model.
func CollectElementDeclsFromComplexType(schema *parser.Schema, complexType *model.ComplexType) []*model.ElementDecl {
	if schema == nil || complexType == nil {
		return nil
	}
	visited := make(map[*model.ComplexType]bool)
	return collectElementDeclsRecursive(schema, complexType, visited)
}

func collectElementDeclsRecursive(schema *parser.Schema, complexType *model.ComplexType, visited map[*model.ComplexType]bool) []*model.ElementDecl {
	if complexType == nil || visited[complexType] {
		return nil
	}
	visited[complexType] = true
	collectElements := func(particle model.Particle) []*model.ElementDecl {
		return CollectFromParticlesWithVisited([]model.Particle{particle}, nil, func(p model.Particle) (*model.ElementDecl, bool) {
			elem, ok := p.(*model.ElementDecl)
			return elem, ok
		})
	}

	var out []*model.ElementDecl
	switch content := complexType.Content().(type) {
	case *model.ElementContent:
		if content.Particle != nil {
			out = append(out, collectElements(content.Particle)...)
		}
	case *model.ComplexContent:
		if content.Extension != nil && content.Extension.Particle != nil {
			out = append(out, collectElements(content.Extension.Particle)...)
		}
		if content.Restriction != nil && content.Restriction.Particle != nil {
			out = append(out, collectElements(content.Restriction.Particle)...)
		}

		var baseQName model.QName
		if content.Extension != nil {
			baseQName = content.Extension.Base
		} else if content.Restriction != nil {
			baseQName = content.Restriction.Base
		}
		if !baseQName.IsZero() {
			if baseType, ok := schema.TypeDefs[baseQName]; ok {
				if baseComplex, ok := baseType.(*model.ComplexType); ok {
					out = append(out, collectElementDeclsRecursive(schema, baseComplex, visited)...)
				}
			}
		}
	}
	return out
}
