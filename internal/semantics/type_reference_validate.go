package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
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
	if err := validateTypeImportReference(schema, typ, contextNamespace); err != nil {
		return err
	}
	if typ.IsBuiltin() {
		return nil
	}
	if err := validateSimpleTypeQNameReference(schema, typ); err != nil {
		return err
	}
	return validateComplexTypeContentReferences(schema, typ, visited, originLocation)
}

func validateTypeImportReference(schema *parser.Schema, typ model.Type, contextNamespace model.NamespaceURI) error {
	qname := typ.Name()
	if qname.IsZero() {
		return nil
	}
	return validateImportForNamespace(schema, contextNamespace, qname.Namespace)
}

func validateSimpleTypeQNameReference(schema *parser.Schema, typ model.Type) error {
	if st, ok := model.AsSimpleType(typ); ok {
		if !st.IsBuiltin() && st.Restriction == nil && st.List == nil && st.Union == nil {
			if err := parser.ValidateTypeQName(schema, st.QName); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateComplexTypeContentReferences(schema *parser.Schema, typ model.Type, visited map[*model.ModelGroup]bool, originLocation string) error {
	if ct, ok := model.AsComplexType(typ); ok {
		if content := ct.Content(); content != nil {
			return validateComplexTypeElementContentReferences(schema, content, visited, originLocation)
		}
	}
	return nil
}

func validateComplexTypeElementContentReferences(schema *parser.Schema, content model.Content, visited map[*model.ModelGroup]bool, originLocation string) error {
	ec, ok := content.(*model.ElementContent)
	if !ok || ec.Particle == nil {
		return nil
	}
	return validateParticleReferencesWithVisited(schema, ec.Particle, visited, originLocation)
}
