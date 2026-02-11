package semanticresolve

import (
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
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
			if err := typeresolve.ValidateTypeQName(schema, st.QName); err != nil {
				return err
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
