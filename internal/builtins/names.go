package builtins

import (
	schematypes "github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xmlnames"
)

const (
	// XSDNamespace is an exported constant.
	XSDNamespace schematypes.NamespaceURI = "http://www.w3.org/2001/XMLSchema"
	// XMLNamespace is an exported constant.
	XMLNamespace schematypes.NamespaceURI = schematypes.NamespaceURI(xmlnames.XMLNamespace)
)

const (
	// TypeNameAnyType is an exported constant.
	TypeNameAnyType schematypes.TypeName = "anyType"
	// TypeNameAnySimpleType is an exported constant.
	TypeNameAnySimpleType schematypes.TypeName = "anySimpleType"

	// TypeNameString is an exported constant.
	TypeNameString schematypes.TypeName = "string"
	// TypeNameBoolean is an exported constant.
	TypeNameBoolean schematypes.TypeName = "boolean"
	// TypeNameDecimal is an exported constant.
	TypeNameDecimal schematypes.TypeName = "decimal"
	// TypeNameFloat is an exported constant.
	TypeNameFloat schematypes.TypeName = "float"
	// TypeNameDouble is an exported constant.
	TypeNameDouble schematypes.TypeName = "double"
	// TypeNameDuration is an exported constant.
	TypeNameDuration schematypes.TypeName = "duration"
	// TypeNameDateTime is an exported constant.
	TypeNameDateTime schematypes.TypeName = "dateTime"
	// TypeNameTime is an exported constant.
	TypeNameTime schematypes.TypeName = "time"
	// TypeNameDate is an exported constant.
	TypeNameDate schematypes.TypeName = "date"
	// TypeNameGYearMonth is an exported constant.
	TypeNameGYearMonth schematypes.TypeName = "gYearMonth"
	// TypeNameGYear is an exported constant.
	TypeNameGYear schematypes.TypeName = "gYear"
	// TypeNameGMonthDay is an exported constant.
	TypeNameGMonthDay schematypes.TypeName = "gMonthDay"
	// TypeNameGDay is an exported constant.
	TypeNameGDay schematypes.TypeName = "gDay"
	// TypeNameGMonth is an exported constant.
	TypeNameGMonth schematypes.TypeName = "gMonth"
	// TypeNameHexBinary is an exported constant.
	TypeNameHexBinary schematypes.TypeName = "hexBinary"
	// TypeNameBase64Binary is an exported constant.
	TypeNameBase64Binary schematypes.TypeName = "base64Binary"
	// TypeNameAnyURI is an exported constant.
	TypeNameAnyURI schematypes.TypeName = "anyURI"
	// TypeNameQName is an exported constant.
	TypeNameQName schematypes.TypeName = "QName"
	// TypeNameNOTATION is an exported constant.
	TypeNameNOTATION schematypes.TypeName = "NOTATION"

	// TypeNameNormalizedString is an exported constant.
	TypeNameNormalizedString schematypes.TypeName = "normalizedString"
	// TypeNameToken is an exported constant.
	TypeNameToken schematypes.TypeName = "token"
	// TypeNameLanguage is an exported constant.
	TypeNameLanguage schematypes.TypeName = "language"
	// TypeNameName is an exported constant.
	TypeNameName schematypes.TypeName = "Name"
	// TypeNameNCName is an exported constant.
	TypeNameNCName schematypes.TypeName = "NCName"
	// TypeNameID is an exported constant.
	TypeNameID schematypes.TypeName = "ID"
	// TypeNameIDREF is an exported constant.
	TypeNameIDREF schematypes.TypeName = "IDREF"
	// TypeNameIDREFS is an exported constant.
	TypeNameIDREFS schematypes.TypeName = "IDREFS"
	// TypeNameENTITY is an exported constant.
	TypeNameENTITY schematypes.TypeName = "ENTITY"
	// TypeNameENTITIES is an exported constant.
	TypeNameENTITIES schematypes.TypeName = "ENTITIES"
	// TypeNameNMTOKEN is an exported constant.
	TypeNameNMTOKEN schematypes.TypeName = "NMTOKEN"
	// TypeNameNMTOKENS is an exported constant.
	TypeNameNMTOKENS schematypes.TypeName = "NMTOKENS"

	// TypeNameInteger is an exported constant.
	TypeNameInteger schematypes.TypeName = "integer"
	// TypeNameLong is an exported constant.
	TypeNameLong schematypes.TypeName = "long"
	// TypeNameInt is an exported constant.
	TypeNameInt schematypes.TypeName = "int"
	// TypeNameShort is an exported constant.
	TypeNameShort schematypes.TypeName = "short"
	// TypeNameByte is an exported constant.
	TypeNameByte schematypes.TypeName = "byte"
	// TypeNameNonNegativeInteger is an exported constant.
	TypeNameNonNegativeInteger schematypes.TypeName = "nonNegativeInteger"
	// TypeNamePositiveInteger is an exported constant.
	TypeNamePositiveInteger schematypes.TypeName = "positiveInteger"
	// TypeNameUnsignedLong is an exported constant.
	TypeNameUnsignedLong schematypes.TypeName = "unsignedLong"
	// TypeNameUnsignedInt is an exported constant.
	TypeNameUnsignedInt schematypes.TypeName = "unsignedInt"
	// TypeNameUnsignedShort is an exported constant.
	TypeNameUnsignedShort schematypes.TypeName = "unsignedShort"
	// TypeNameUnsignedByte is an exported constant.
	TypeNameUnsignedByte schematypes.TypeName = "unsignedByte"
	// TypeNameNegativeInteger is an exported constant.
	TypeNameNegativeInteger schematypes.TypeName = "negativeInteger"
	// TypeNameNonPositiveInteger is an exported constant.
	TypeNameNonPositiveInteger schematypes.TypeName = "nonPositiveInteger"
)
