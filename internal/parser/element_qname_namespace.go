package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func validateQNameNamespace(schema *Schema, namespace model.NamespaceURI) error {
	if schema == nil {
		return nil
	}
	if namespace == model.XSDNamespace || namespace == xmltree.XMLNamespace {
		return nil
	}
	if namespace == schema.TargetNamespace {
		return nil
	}
	imports := schema.ImportedNamespaces[schema.TargetNamespace]
	if namespace == "" {
		if schema.TargetNamespace == "" {
			return nil
		}
		if imports != nil && imports[model.NamespaceEmpty] {
			return nil
		}
		return fmt.Errorf("namespace %s not imported for %s", namespace, schema.TargetNamespace)
	}
	if imports != nil && imports[namespace] {
		return nil
	}
	return fmt.Errorf("namespace %s not imported for %s", namespace, schema.TargetNamespace)
}
