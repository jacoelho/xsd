package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

func validateIDAttributeCount(schema *schema.Schema, ct *types.ComplexType) error {
	attrs := collectAllAttributesForValidation(schema, ct)
	idCount := 0
	for _, attr := range attrs {
		if attr.Use == types.Prohibited {
			continue
		}
		if attr.Type == nil {
			continue
		}
		typeName := attr.Type.Name()
		if typeName.Namespace == types.XSDNamespace && typeName.Local == "ID" {
			idCount++
			continue
		}
		if st, ok := attr.Type.(*types.SimpleType); ok {
			if isIDOnlyDerivedType(st) {
				idCount++
			}
		}
	}
	if idCount > 1 {
		return fmt.Errorf("type %s has multiple ID attributes", ct.QName.Local)
	}
	return nil
}
