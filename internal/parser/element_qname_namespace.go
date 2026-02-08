package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func validateQNameNamespace(schema *Schema, namespace types.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if namespace == types.XSDNamespace || namespace == xsdxml.XMLNamespace {
		return nil
	}
	if namespace == schema.TargetNamespace {
		return nil
	}
	imports := schema.ImportedNamespaces[schema.TargetNamespace]
	if namespace.IsEmpty() {
		if schema.TargetNamespace.IsEmpty() {
			return nil
		}
		if imports != nil && imports[types.NamespaceEmpty] {
			return nil
		}
		return fmt.Errorf("namespace %s not imported for %s", namespace, schema.TargetNamespace)
	}
	if imports != nil && imports[namespace] {
		return nil
	}
	return fmt.Errorf("namespace %s not imported for %s", namespace, schema.TargetNamespace)
}
