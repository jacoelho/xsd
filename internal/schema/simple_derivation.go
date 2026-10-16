package schema

import "github.com/jacoelho/xsd/xsderrors"

// SimpleTypeFinalRole identifies the schema component role that is applying a
// simple-type final derivation rule.
type SimpleTypeFinalRole uint8

const (
	// SimpleTypeFinalBaseRestriction checks a restriction base simple type.
	SimpleTypeFinalBaseRestriction SimpleTypeFinalRole = iota
	// SimpleTypeFinalListItem checks an xs:list item type.
	SimpleTypeFinalListItem
	// SimpleTypeFinalUnionMember checks an xs:union member type.
	SimpleTypeFinalUnionMember
)

// CheckSimpleRestrictionBase rejects direct restriction of xs:anySimpleType.
func CheckSimpleRestrictionBase(baseID, anySimpleType SimpleTypeID) error {
	if baseID == anySimpleType {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "simple type cannot restrict xs:anySimpleType")
	}
	return nil
}

// CheckSimpleTypeFinalAllows maps runtime simple-type final-mask rejection into
// the compile diagnostic for the schema role being derived.
func CheckSimpleTypeFinalAllows(final, derivation DerivationMask, role SimpleTypeFinalRole) error {
	if err := ValidateSimpleTypeFinalAllows(final, derivation); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, simpleTypeFinalRoleMessage(role))
	}
	return nil
}

func simpleListItemListReachError() error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "list item type cannot be a list type")
}

func simpleTypeFinalRoleMessage(role SimpleTypeFinalRole) string {
	switch role {
	case SimpleTypeFinalBaseRestriction:
		return "base simple type final blocks restriction"
	case SimpleTypeFinalListItem:
		return "item simple type final blocks list"
	case SimpleTypeFinalUnionMember:
		return "member simple type final blocks union"
	default:
		return "simple type final blocks derivation"
	}
}
