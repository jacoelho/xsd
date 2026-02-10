package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
)

func validateIDAttributeCount(schema *parser.Schema, complexType *model.ComplexType) error {
	attrs := collectAllAttributesForValidation(schema, complexType)
	idCount := 0
	for _, attr := range attrs {
		if attr.Use == model.Prohibited || attr.Type == nil {
			continue
		}
		resolvedType := typeops.ResolveTypeReference(schema, attr.Type, typeops.TypeReferenceAllowMissing)
		if resolvedType == nil {
			continue
		}
		typeName := resolvedType.Name()
		if typeName.Namespace == model.XSDNamespace && typeName.Local == string(model.TypeNameID) {
			idCount++
			continue
		}
		if simpleType, ok := resolvedType.(*model.SimpleType); ok {
			if typeops.IsIDOnlyDerivedType(schema, simpleType) {
				idCount++
			}
		}
	}
	if idCount > 1 {
		return fmt.Errorf("type %s has multiple ID attributes", complexType.QName.Local)
	}
	return nil
}
