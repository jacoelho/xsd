package traversal

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// CollectElementDeclsFromComplexType collects element declarations in a complex
// type content model and recursively through its complex base types.
func CollectElementDeclsFromComplexType(schema *parser.Schema, complexType *types.ComplexType) []*types.ElementDecl {
	if schema == nil || complexType == nil {
		return nil
	}
	visited := make(map[*types.ComplexType]bool)
	return collectElementDeclsRecursive(schema, complexType, visited)
}

func collectElementDeclsRecursive(schema *parser.Schema, complexType *types.ComplexType, visited map[*types.ComplexType]bool) []*types.ElementDecl {
	if complexType == nil || visited[complexType] {
		return nil
	}
	visited[complexType] = true

	var out []*types.ElementDecl
	switch content := complexType.Content().(type) {
	case *types.ElementContent:
		if content.Particle != nil {
			out = append(out, CollectElements(content.Particle)...)
		}
	case *types.ComplexContent:
		if content.Extension != nil && content.Extension.Particle != nil {
			out = append(out, CollectElements(content.Extension.Particle)...)
		}
		if content.Restriction != nil && content.Restriction.Particle != nil {
			out = append(out, CollectElements(content.Restriction.Particle)...)
		}

		var baseQName types.QName
		if content.Extension != nil {
			baseQName = content.Extension.Base
		} else if content.Restriction != nil {
			baseQName = content.Restriction.Base
		}
		if !baseQName.IsZero() {
			if baseType, ok := schema.TypeDefs[baseQName]; ok {
				if baseComplex, ok := baseType.(*types.ComplexType); ok {
					out = append(out, collectElementDeclsRecursive(schema, baseComplex, visited)...)
				}
			}
		}
	}
	return out
}
