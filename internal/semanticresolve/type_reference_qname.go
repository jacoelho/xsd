package semanticresolve

import (
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// validateTypeQNameReference validates that a type QName reference exists.
func validateTypeQNameReference(schema *parser.Schema, qname model.QName, contextNamespace model.NamespaceURI) error {
	if qname.IsZero() {
		return nil
	}

	if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
		return err
	}
	return typeresolve.ValidateTypeQName(schema, qname)
}
