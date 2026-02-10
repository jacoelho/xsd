package fieldresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
	"github.com/jacoelho/xsd/internal/xpath"
)

func resolveFieldPathType(schema *parser.Schema, selectedElementDecl *model.ElementDecl, fieldPath xpath.Path) (model.Type, error) {
	if selectedElementDecl == nil {
		return nil, fmt.Errorf("cannot resolve field without selector element")
	}
	if fieldPath.Attribute != nil && isDescendantOnlySteps(fieldPath.Steps) {
		attrType, attrErr := findAttributeTypeDescendant(schema, selectedElementDecl, *fieldPath.Attribute)
		if attrErr != nil {
			return nil, fmt.Errorf("resolve attribute field '%s': %w", formatNodeTest(*fieldPath.Attribute), attrErr)
		}
		return attrType, nil
	}

	elementDecl, err := resolvePathElementDecl(schema, selectedElementDecl, fieldPath.Steps)
	if err != nil {
		return nil, fmt.Errorf("resolve field path: %w", err)
	}
	elementDecl = resolveElementReference(schema, elementDecl)

	if fieldPath.Attribute != nil {
		attrType, err := findAttributeType(schema, elementDecl, *fieldPath.Attribute)
		if err != nil {
			return nil, fmt.Errorf("resolve attribute field '%s': %w", formatNodeTest(*fieldPath.Attribute), err)
		}
		return attrType, nil
	}

	elementType := typeresolve.ResolveTypeReference(schema, elementDecl.Type, typeresolve.TypeReferenceMustExist)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}
	if elementDecl.Nillable {
		return elementType, ErrFieldSelectsNillable
	}

	if ct, ok := elementType.(*model.ComplexType); ok {
		if _, ok := ct.Content().(*model.SimpleContent); ok {
			baseType := ct.BaseType()
			if baseType != nil {
				return baseType, nil
			}
		}
		return nil, ErrFieldSelectsComplexContent
	}

	return elementType, nil
}
