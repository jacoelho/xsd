package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateIDAttributeCount(schema *parser.Schema, complexType *model.ComplexType) error {
	attrs := collectAllAttributesForValidation(schema, complexType)
	idCount := 0
	for _, attr := range attrs {
		if attr.Use == model.Prohibited || attr.Type == nil {
			continue
		}
		resolvedType := parser.ResolveTypeReferenceAllowMissing(schema, attr.Type)
		if resolvedType == nil {
			continue
		}
		typeName := resolvedType.Name()
		if typeName.Namespace == model.XSDNamespace && typeName.Local == string(model.TypeNameID) {
			idCount++
			continue
		}
		if simpleType, ok := resolvedType.(*model.SimpleType); ok {
			if parser.IsIDOnlyDerivedType(schema, simpleType) {
				idCount++
			}
		}
	}
	if idCount > 1 {
		return fmt.Errorf("type %s has multiple ID attributes", complexType.QName.Local)
	}
	return nil
}
