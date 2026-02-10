package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
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

	if qname.Namespace == model.XSDNamespace {
		if builtins.Get(builtins.TypeName(qname.Local)) == nil {
			return fmt.Errorf("type '%s' not found in XSD namespace", qname.Local)
		}
		return nil
	}

	if _, exists := schema.TypeDefs[qname]; !exists {
		return fmt.Errorf("type %s not found", qname)
	}

	return nil
}
