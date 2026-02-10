package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

const noOriginLocation = ""

func validateTypeReferenceFromTypeAtLocation(schema *parser.Schema, typ model.Type, contextNamespace model.NamespaceURI, originLocation string) error {
	visited := make(map[*model.ModelGroup]bool)
	return validateTypeReferenceFromTypeWithVisited(schema, typ, visited, contextNamespace, originLocation)
}

// validateTypeReferenceFromTypeWithVisited validates type reference with cycle detection.
func validateTypeReferenceFromTypeWithVisited(schema *parser.Schema, typ model.Type, visited map[*model.ModelGroup]bool, contextNamespace model.NamespaceURI, originLocation string) error {
	if typ == nil {
		return nil
	}
	qname := typ.Name()
	if !qname.IsZero() {
		if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
			return err
		}
	}
	if typ.IsBuiltin() {
		return nil
	}

	if st, ok := model.AsSimpleType(typ); ok {
		if !st.IsBuiltin() && st.Restriction == nil && st.List == nil && st.Union == nil {
			if _, exists := schema.TypeDefs[st.QName]; !exists {
				if st.QName.Namespace == model.XSDNamespace {
					if builtins.Get(builtins.TypeName(st.QName.Local)) == nil {
						return fmt.Errorf("type '%s' not found in XSD namespace", st.QName.Local)
					}
					return nil
				}
				return fmt.Errorf("type %s not found", st.QName)
			}
		}
	}

	if ct, ok := model.AsComplexType(typ); ok {
		if content := ct.Content(); content != nil {
			if ec, ok := content.(*model.ElementContent); ok && ec.Particle != nil {
				if err := validateParticleReferencesWithVisited(schema, ec.Particle, visited, originLocation); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
