package schema

import "github.com/jacoelho/xsd/xsderrors"

// simpleTypeFinalRole identifies the schema component role that is applying a
// simple-type final derivation rule.
type simpleTypeFinalRole uint8

const (
	// simpleTypeFinalBaseRestriction checks a restriction base simple type.
	simpleTypeFinalBaseRestriction simpleTypeFinalRole = iota
	// simpleTypeFinalListItem checks an xs:list item type.
	simpleTypeFinalListItem
	// simpleTypeFinalUnionMember checks an xs:union member type.
	simpleTypeFinalUnionMember
)

// checkSimpleRestrictionBase rejects direct restriction of xs:anySimpleType.
func checkSimpleRestrictionBase(baseID, anySimpleType SimpleTypeID) error {
	if baseID == anySimpleType {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, "simple type cannot restrict xs:anySimpleType")
	}
	return nil
}

// checkSimpleTypeFinalAllows maps runtime simple-type final-mask rejection into
// the compile diagnostic for the schema role being derived.
func checkSimpleTypeFinalAllows(final, derivation DerivationMask, role simpleTypeFinalRole) error {
	if err := validateSimpleTypeFinalAllows(final, derivation); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaReference, simpleTypeFinalRoleMessage(role))
	}
	return nil
}

func simpleListItemListReachError() error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "list item type cannot be a list type")
}

func simpleTypeFinalRoleMessage(role simpleTypeFinalRole) string {
	switch role {
	case simpleTypeFinalBaseRestriction:
		return "base simple type final blocks restriction"
	case simpleTypeFinalListItem:
		return "item simple type final blocks list"
	case simpleTypeFinalUnionMember:
		return "member simple type final blocks union"
	default:
		return "simple type final blocks derivation"
	}
}
