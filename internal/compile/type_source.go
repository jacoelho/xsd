package compile

import (
	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/xsderrors"
)

// ValidateAttributeTypeSource validates that an attribute declaration has only
// one type source.
func ValidateAttributeTypeSource(hasTypeAttr, hasSimpleTypeChild bool) error {
	if hasTypeAttr && hasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "attribute cannot have both type and simpleType")
	}
	return nil
}

// ValidateSimpleRestrictionTypeSource validates that a simple restriction has
// only one base type source.
func ValidateSimpleRestrictionTypeSource(hasBaseAttr, hasSimpleTypeChild bool) error {
	if hasBaseAttr && hasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "restriction cannot have both base and simpleType")
	}
	if !hasBaseAttr && !hasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "simple restriction missing base")
	}
	return nil
}

// ValidateSimpleListItemTypeSource validates that a simple list has only one
// item type source.
func ValidateSimpleListItemTypeSource(hasItemTypeAttr, hasSimpleTypeChild bool) error {
	if hasItemTypeAttr && hasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "list cannot have both itemType and simpleType")
	}
	if !hasItemTypeAttr && !hasSimpleTypeChild {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "list missing item type")
	}
	return nil
}

// ParseUnionMemberTypes parses the xs:union memberTypes attribute and validates
// that the union has at least one member source.
func ParseUnionMemberTypes(memberTypes string, hasMemberTypes, hasSimpleTypeChild bool) ([]string, error) {
	var members []string
	if hasMemberTypes {
		for part := range lex.XMLFieldsSeq(memberTypes) {
			members = append(members, part)
		}
	}
	if len(members) == 0 && !hasSimpleTypeChild {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "union missing member types")
	}
	return members, nil
}
