package builtins

import (
	schematypes "github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xmlnames"
)

const (
	XSDNamespace schematypes.NamespaceURI = "http://www.w3.org/2001/XMLSchema"
	XMLNamespace schematypes.NamespaceURI = schematypes.NamespaceURI(xmlnames.XMLNamespace)
)

const (
	TypeNameAnyType       schematypes.TypeName = "anyType"
	TypeNameAnySimpleType schematypes.TypeName = "anySimpleType"

	TypeNameString       schematypes.TypeName = "string"
	TypeNameBoolean      schematypes.TypeName = "boolean"
	TypeNameDecimal      schematypes.TypeName = "decimal"
	TypeNameFloat        schematypes.TypeName = "float"
	TypeNameDouble       schematypes.TypeName = "double"
	TypeNameDuration     schematypes.TypeName = "duration"
	TypeNameDateTime     schematypes.TypeName = "dateTime"
	TypeNameTime         schematypes.TypeName = "time"
	TypeNameDate         schematypes.TypeName = "date"
	TypeNameGYearMonth   schematypes.TypeName = "gYearMonth"
	TypeNameGYear        schematypes.TypeName = "gYear"
	TypeNameGMonthDay    schematypes.TypeName = "gMonthDay"
	TypeNameGDay         schematypes.TypeName = "gDay"
	TypeNameGMonth       schematypes.TypeName = "gMonth"
	TypeNameHexBinary    schematypes.TypeName = "hexBinary"
	TypeNameBase64Binary schematypes.TypeName = "base64Binary"
	TypeNameAnyURI       schematypes.TypeName = "anyURI"
	TypeNameQName        schematypes.TypeName = "QName"
	TypeNameNOTATION     schematypes.TypeName = "NOTATION"

	TypeNameNormalizedString schematypes.TypeName = "normalizedString"
	TypeNameToken            schematypes.TypeName = "token"
	TypeNameLanguage         schematypes.TypeName = "language"
	TypeNameName             schematypes.TypeName = "Name"
	TypeNameNCName           schematypes.TypeName = "NCName"
	TypeNameID               schematypes.TypeName = "ID"
	TypeNameIDREF            schematypes.TypeName = "IDREF"
	TypeNameIDREFS           schematypes.TypeName = "IDREFS"
	TypeNameENTITY           schematypes.TypeName = "ENTITY"
	TypeNameENTITIES         schematypes.TypeName = "ENTITIES"
	TypeNameNMTOKEN          schematypes.TypeName = "NMTOKEN"
	TypeNameNMTOKENS         schematypes.TypeName = "NMTOKENS"

	TypeNameInteger            schematypes.TypeName = "integer"
	TypeNameLong               schematypes.TypeName = "long"
	TypeNameInt                schematypes.TypeName = "int"
	TypeNameShort              schematypes.TypeName = "short"
	TypeNameByte               schematypes.TypeName = "byte"
	TypeNameNonNegativeInteger schematypes.TypeName = "nonNegativeInteger"
	TypeNamePositiveInteger    schematypes.TypeName = "positiveInteger"
	TypeNameUnsignedLong       schematypes.TypeName = "unsignedLong"
	TypeNameUnsignedInt        schematypes.TypeName = "unsignedInt"
	TypeNameUnsignedShort      schematypes.TypeName = "unsignedShort"
	TypeNameUnsignedByte       schematypes.TypeName = "unsignedByte"
	TypeNameNegativeInteger    schematypes.TypeName = "negativeInteger"
	TypeNameNonPositiveInteger schematypes.TypeName = "nonPositiveInteger"
)
