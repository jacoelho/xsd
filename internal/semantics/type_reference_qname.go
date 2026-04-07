package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateTypeQNameReference validates that a type QName reference exists.
func validateTypeQNameReference(schema *parser.Schema, qname model.QName, contextNamespace model.NamespaceURI) error {
	if qname.IsZero() {
		return nil
	}

	if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
		return err
	}
	return parser.ValidateTypeQName(schema, qname)
}
