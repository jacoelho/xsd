package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/types"
)

// validateTypeQNameReference validates that a type QName reference exists.
func validateTypeQNameReference(schema *parser.Schema, qname types.QName, contextNamespace types.NamespaceURI) error {
	if qname.IsZero() {
		return nil
	}

	if err := validateImportForNamespace(schema, contextNamespace, qname.Namespace); err != nil {
		return err
	}
	return typeresolve.ValidateTypeQName(schema, qname)
}
