package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/typechain"
)

// validateElementDeclarationsConsistent validates extension element consistency with base.
func validateElementDeclarationsConsistent(schema *parser.Schema, complexType *model.ComplexType) error {
	if !complexType.IsExtension() {
		return nil
	}

	content := complexType.Content()
	ext := content.ExtensionDef()
	if ext == nil {
		return nil
	}

	baseQName := content.BaseTypeQName()
	baseComplexType, ok := typechain.LookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}

	baseElements := traversal.CollectElementDeclsFromComplexType(schema, baseComplexType)
	if ext.Particle == nil {
		return nil
	}
	extElements := traversal.CollectFromParticlesWithVisited([]model.Particle{ext.Particle}, nil, func(p model.Particle) (*model.ElementDecl, bool) {
		elem, ok := p.(*model.ElementDecl)
		return elem, ok
	})

	for _, extElem := range extElements {
		for _, baseElem := range baseElements {
			if extElem.Name != baseElem.Name {
				continue
			}
			extTypeQName := model.QName{}
			if extElem.Type != nil {
				extTypeQName = extElem.Type.Name()
			}
			baseTypeQName := model.QName{}
			if baseElem.Type != nil {
				baseTypeQName = baseElem.Type.Name()
			}
			if extTypeQName != baseTypeQName {
				return fmt.Errorf("element '%s' in extension has type '%s' but base type has type '%s' (Element Declarations Consistent violation)", extElem.Name.Local, extTypeQName, baseTypeQName)
			}
		}
	}

	return nil
}
