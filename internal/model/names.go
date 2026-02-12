package model

import (
	"github.com/jacoelho/xsd/internal/qname"
)

// TypeName represents the local name of an XSD type
// Using a typed string prevents mixing type names with other strings
type TypeName string

// Built-in XSD type name constants
const (
	// Complex type
	TypeNameAnyType TypeName = "anyType"

	// Base simple type (base of all simple types)
	TypeNameAnySimpleType TypeName = "anySimpleType"

	// Primitive types (19 total)
	TypeNameString TypeName = "string"
	// TypeNameBoolean is an exported constant.
	TypeNameBoolean TypeName = "boolean"
	// TypeNameDecimal is an exported constant.
	TypeNameDecimal TypeName = "decimal"
	// TypeNameFloat is an exported constant.
	TypeNameFloat TypeName = "float"
	// TypeNameDouble is an exported constant.
	TypeNameDouble TypeName = "double"
	// TypeNameDuration is an exported constant.
	TypeNameDuration TypeName = "duration"
	// TypeNameDateTime is an exported constant.
	TypeNameDateTime TypeName = "dateTime"
	// TypeNameTime is an exported constant.
	TypeNameTime TypeName = "time"
	// TypeNameDate is an exported constant.
	TypeNameDate TypeName = "date"
	// TypeNameGYearMonth is an exported constant.
	TypeNameGYearMonth TypeName = "gYearMonth"
	// TypeNameGYear is an exported constant.
	TypeNameGYear TypeName = "gYear"
	// TypeNameGMonthDay is an exported constant.
	TypeNameGMonthDay TypeName = "gMonthDay"
	// TypeNameGDay is an exported constant.
	TypeNameGDay TypeName = "gDay"
	// TypeNameGMonth is an exported constant.
	TypeNameGMonth TypeName = "gMonth"
	// TypeNameHexBinary is an exported constant.
	TypeNameHexBinary TypeName = "hexBinary"
	// TypeNameBase64Binary is an exported constant.
	TypeNameBase64Binary TypeName = "base64Binary"
	// TypeNameAnyURI is an exported constant.
	TypeNameAnyURI TypeName = "anyURI"
	// TypeNameQName is an exported constant.
	TypeNameQName TypeName = "QName"
	// TypeNameNOTATION is an exported constant.
	TypeNameNOTATION TypeName = "NOTATION"

	// Derived string types
	TypeNameNormalizedString TypeName = "normalizedString"
	// TypeNameToken is an exported constant.
	TypeNameToken TypeName = "token"
	// TypeNameLanguage is an exported constant.
	TypeNameLanguage TypeName = "language"
	// TypeNameName is an exported constant.
	TypeNameName TypeName = "Name"
	// TypeNameNCName is an exported constant.
	TypeNameNCName TypeName = "NCName"
	// TypeNameID is an exported constant.
	TypeNameID TypeName = "ID"
	// TypeNameIDREF is an exported constant.
	TypeNameIDREF TypeName = "IDREF"
	// TypeNameIDREFS is an exported constant.
	TypeNameIDREFS TypeName = "IDREFS"
	// TypeNameENTITY is an exported constant.
	TypeNameENTITY TypeName = "ENTITY"
	// TypeNameENTITIES is an exported constant.
	TypeNameENTITIES TypeName = "ENTITIES"
	// TypeNameNMTOKEN is an exported constant.
	TypeNameNMTOKEN TypeName = "NMTOKEN"
	// TypeNameNMTOKENS is an exported constant.
	TypeNameNMTOKENS TypeName = "NMTOKENS"

	// Derived numeric types
	TypeNameInteger TypeName = "integer"
	// TypeNameLong is an exported constant.
	TypeNameLong TypeName = "long"
	// TypeNameInt is an exported constant.
	TypeNameInt TypeName = "int"
	// TypeNameShort is an exported constant.
	TypeNameShort TypeName = "short"
	// TypeNameByte is an exported constant.
	TypeNameByte TypeName = "byte"
	// TypeNameNonNegativeInteger is an exported constant.
	TypeNameNonNegativeInteger TypeName = "nonNegativeInteger"
	// TypeNamePositiveInteger is an exported constant.
	TypeNamePositiveInteger TypeName = "positiveInteger"
	// TypeNameUnsignedLong is an exported constant.
	TypeNameUnsignedLong TypeName = "unsignedLong"
	// TypeNameUnsignedInt is an exported constant.
	TypeNameUnsignedInt TypeName = "unsignedInt"
	// TypeNameUnsignedShort is an exported constant.
	TypeNameUnsignedShort TypeName = "unsignedShort"
	// TypeNameUnsignedByte is an exported constant.
	TypeNameUnsignedByte TypeName = "unsignedByte"
	// TypeNameNegativeInteger is an exported constant.
	TypeNameNegativeInteger TypeName = "negativeInteger"
	// TypeNameNonPositiveInteger is an exported constant.
	TypeNameNonPositiveInteger TypeName = "nonPositiveInteger"
)

// IsQNameOrNotation reports whether the QName is the XSD QName or NOTATION type.
func IsQNameOrNotation(name QName) bool {
	if name.Namespace != XSDNamespace {
		return false
	}
	return name.Local == string(TypeNameQName) || name.Local == string(TypeNameNOTATION)
}

// String returns the string representation of the type name
func (tn TypeName) String() string {
	return string(tn)
}

// NamespaceURI is an alias of qname.NamespaceURI.
type NamespaceURI = qname.NamespaceURI

// NamespaceEmpty is an exported constant.
const NamespaceEmpty = qname.NamespaceEmpty

// QName is an alias of qname.QName.
type QName = qname.QName
