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
	TypeNameString       TypeName = "string"
	TypeNameBoolean      TypeName = "boolean"
	TypeNameDecimal      TypeName = "decimal"
	TypeNameFloat        TypeName = "float"
	TypeNameDouble       TypeName = "double"
	TypeNameDuration     TypeName = "duration"
	TypeNameDateTime     TypeName = "dateTime"
	TypeNameTime         TypeName = "time"
	TypeNameDate         TypeName = "date"
	TypeNameGYearMonth   TypeName = "gYearMonth"
	TypeNameGYear        TypeName = "gYear"
	TypeNameGMonthDay    TypeName = "gMonthDay"
	TypeNameGDay         TypeName = "gDay"
	TypeNameGMonth       TypeName = "gMonth"
	TypeNameHexBinary    TypeName = "hexBinary"
	TypeNameBase64Binary TypeName = "base64Binary"
	TypeNameAnyURI       TypeName = "anyURI"
	TypeNameQName        TypeName = "QName"
	TypeNameNOTATION     TypeName = "NOTATION"

	// Derived string types
	TypeNameNormalizedString TypeName = "normalizedString"
	TypeNameToken            TypeName = "token"
	TypeNameLanguage         TypeName = "language"
	TypeNameName             TypeName = "Name"
	TypeNameNCName           TypeName = "NCName"
	TypeNameID               TypeName = "ID"
	TypeNameIDREF            TypeName = "IDREF"
	TypeNameIDREFS           TypeName = "IDREFS"
	TypeNameENTITY           TypeName = "ENTITY"
	TypeNameENTITIES         TypeName = "ENTITIES"
	TypeNameNMTOKEN          TypeName = "NMTOKEN"
	TypeNameNMTOKENS         TypeName = "NMTOKENS"

	// Derived numeric types
	TypeNameInteger            TypeName = "integer"
	TypeNameLong               TypeName = "long"
	TypeNameInt                TypeName = "int"
	TypeNameShort              TypeName = "short"
	TypeNameByte               TypeName = "byte"
	TypeNameNonNegativeInteger TypeName = "nonNegativeInteger"
	TypeNamePositiveInteger    TypeName = "positiveInteger"
	TypeNameUnsignedLong       TypeName = "unsignedLong"
	TypeNameUnsignedInt        TypeName = "unsignedInt"
	TypeNameUnsignedShort      TypeName = "unsignedShort"
	TypeNameUnsignedByte       TypeName = "unsignedByte"
	TypeNameNegativeInteger    TypeName = "negativeInteger"
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

const NamespaceEmpty = qname.NamespaceEmpty

// QName is an alias of qname.QName.
type QName = qname.QName
