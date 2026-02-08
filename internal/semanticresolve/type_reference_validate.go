package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

const noOriginLocation = ""

// validateTypeReferenceFromType validates that a type reference exists (from a Type interface).
func validateTypeReferenceFromType(schema *parser.Schema, typ types.Type, contextNamespace types.NamespaceURI) error {
	return validateTypeReferenceFromTypeAtLocation(schema, typ, contextNamespace, noOriginLocation)
}

func validateTypeReferenceFromTypeAtLocation(schema *parser.Schema, typ types.Type, contextNamespace types.NamespaceURI, originLocation string) error {
	visited := make(map[*types.ModelGroup]bool)
	return validateTypeReferenceFromTypeWithVisited(schema, typ, visited, contextNamespace, originLocation)
}

// validateTypeReferenceFromTypeWithVisited validates type reference with cycle detection.
func validateTypeReferenceFromTypeWithVisited(schema *parser.Schema, typ types.Type, visited map[*types.ModelGroup]bool, contextNamespace types.NamespaceURI, originLocation string) error {
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

	if st, ok := types.AsSimpleType(typ); ok {
		if !st.IsBuiltin() && st.Restriction == nil && st.List == nil && st.Union == nil {
			if _, exists := schema.TypeDefs[st.QName]; !exists {
				if st.QName.Namespace == types.XSDNamespace {
					if types.GetBuiltin(types.TypeName(st.QName.Local)) == nil {
						return fmt.Errorf("type '%s' not found in XSD namespace", st.QName.Local)
					}
					return nil
				}
				return fmt.Errorf("type %s not found", st.QName)
			}
		}
	}

	if ct, ok := types.AsComplexType(typ); ok {
		if content := ct.Content(); content != nil {
			if ec, ok := content.(*types.ElementContent); ok && ec.Particle != nil {
				if err := validateParticleReferencesWithVisited(schema, ec.Particle, visited, originLocation); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
