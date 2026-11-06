package schema

import (
	"github.com/jacoelho/xsd/xsderrors"
)

// attributeTypeSource describes the two possible type sources for an
// attribute declaration.
type attributeTypeSource struct {
	Type               lexicalAttribute
	HasSimpleTypeChild bool
}

// validateAttributeTypeSource validates that an attribute declaration has only
// one type source.
func validateAttributeTypeSource(source attributeTypeSource) error {
	if source.Type.Present && source.HasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "attribute cannot have both type and simpleType")
	}
	return nil
}

// simpleRestrictionTypeSource describes the two possible base sources for a
// simple restriction.
type simpleRestrictionTypeSource struct {
	Base               lexicalAttribute
	HasSimpleTypeChild bool
}

// validateSimpleRestrictionTypeSource validates that a simple restriction has
// only one base type source.
func validateSimpleRestrictionTypeSource(source simpleRestrictionTypeSource) error {
	if source.Base.Present && source.HasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "restriction cannot have both base and simpleType")
	}
	if !source.Base.Present && !source.HasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "simple restriction missing base")
	}
	return nil
}

// simpleListItemTypeSource describes the two possible item-type sources for a
// simple list.
type simpleListItemTypeSource struct {
	ItemType           lexicalAttribute
	HasSimpleTypeChild bool
}

// validateSimpleListItemTypeSource validates that a simple list has only one
// item type source.
func validateSimpleListItemTypeSource(source simpleListItemTypeSource) error {
	if source.ItemType.Present && source.HasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "list cannot have both itemType and simpleType")
	}
	if !source.ItemType.Present && !source.HasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "list missing item type")
	}
	return nil
}
