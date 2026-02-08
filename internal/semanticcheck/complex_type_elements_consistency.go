package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// validateElementDeclarationsConsistent validates extension element consistency with base.
func validateElementDeclarationsConsistent(schema *parser.Schema, complexType *types.ComplexType) error {
	if !complexType.IsExtension() {
		return nil
	}

	content := complexType.Content()
	ext := content.ExtensionDef()
	if ext == nil {
		return nil
	}

	baseQName := content.BaseTypeQName()
	baseComplexType, ok := lookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}

	baseElements := CollectAllElementDeclarationsFromType(schema, baseComplexType)
	if ext.Particle == nil {
		return nil
	}
	extElements := traversal.CollectElements(ext.Particle)

	for _, extElem := range extElements {
		for _, baseElem := range baseElements {
			if extElem.Name != baseElem.Name {
				continue
			}
			extTypeQName := getTypeQName(extElem.Type)
			baseTypeQName := getTypeQName(baseElem.Type)
			if extTypeQName != baseTypeQName {
				return fmt.Errorf("element '%s' in extension has type '%s' but base type has type '%s' (Element Declarations Consistent violation)", extElem.Name.Local, extTypeQName, baseTypeQName)
			}
		}
	}

	return nil
}

// CollectAllElementDeclarationsFromType collects all element declarations from a complex type.
func CollectAllElementDeclarationsFromType(schema *parser.Schema, complexType *types.ComplexType) []*types.ElementDecl {
	return traversal.CollectElementDeclsFromComplexType(schema, complexType)
}
