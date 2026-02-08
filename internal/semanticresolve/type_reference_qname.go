package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
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

	if qname.Namespace == types.XSDNamespace {
		if types.GetBuiltin(types.TypeName(qname.Local)) == nil {
			return fmt.Errorf("type '%s' not found in XSD namespace", qname.Local)
		}
		return nil
	}

	if _, exists := schema.TypeDefs[qname]; !exists {
		return fmt.Errorf("type %s not found", qname)
	}

	return nil
}
