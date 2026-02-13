package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/types"
)

const noOriginLocation = ""

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
			if err := typeresolve.ValidateTypeQName(schema, st.QName); err != nil {
				return err
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
