package builtins

import (
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xmlnames"
)

const (
	XSDNamespace types.NamespaceURI = "http://www.w3.org/2001/XMLSchema"
	XMLNamespace types.NamespaceURI = types.NamespaceURI(xmlnames.XMLNamespace)
)

const (
	TypeNameAnyType       types.TypeName = "anyType"
	TypeNameAnySimpleType types.TypeName = "anySimpleType"

	TypeNameString       types.TypeName = "string"
	TypeNameBoolean      types.TypeName = "boolean"
	TypeNameDecimal      types.TypeName = "decimal"
	TypeNameFloat        types.TypeName = "float"
	TypeNameDouble       types.TypeName = "double"
	TypeNameDuration     types.TypeName = "duration"
	TypeNameDateTime     types.TypeName = "dateTime"
	TypeNameTime         types.TypeName = "time"
	TypeNameDate         types.TypeName = "date"
	TypeNameGYearMonth   types.TypeName = "gYearMonth"
	TypeNameGYear        types.TypeName = "gYear"
	TypeNameGMonthDay    types.TypeName = "gMonthDay"
	TypeNameGDay         types.TypeName = "gDay"
	TypeNameGMonth       types.TypeName = "gMonth"
	TypeNameHexBinary    types.TypeName = "hexBinary"
	TypeNameBase64Binary types.TypeName = "base64Binary"
	TypeNameAnyURI       types.TypeName = "anyURI"
	TypeNameQName        types.TypeName = "QName"
	TypeNameNOTATION     types.TypeName = "NOTATION"

	TypeNameNormalizedString types.TypeName = "normalizedString"
	TypeNameToken            types.TypeName = "token"
	TypeNameLanguage         types.TypeName = "language"
	TypeNameName             types.TypeName = "Name"
	TypeNameNCName           types.TypeName = "NCName"
	TypeNameID               types.TypeName = "ID"
	TypeNameIDREF            types.TypeName = "IDREF"
	TypeNameIDREFS           types.TypeName = "IDREFS"
	TypeNameENTITY           types.TypeName = "ENTITY"
	TypeNameENTITIES         types.TypeName = "ENTITIES"
	TypeNameNMTOKEN          types.TypeName = "NMTOKEN"
	TypeNameNMTOKENS         types.TypeName = "NMTOKENS"

	TypeNameInteger            types.TypeName = "integer"
	TypeNameLong               types.TypeName = "long"
	TypeNameInt                types.TypeName = "int"
	TypeNameShort              types.TypeName = "short"
	TypeNameByte               types.TypeName = "byte"
	TypeNameNonNegativeInteger types.TypeName = "nonNegativeInteger"
	TypeNamePositiveInteger    types.TypeName = "positiveInteger"
	TypeNameUnsignedLong       types.TypeName = "unsignedLong"
	TypeNameUnsignedInt        types.TypeName = "unsignedInt"
	TypeNameUnsignedShort      types.TypeName = "unsignedShort"
	TypeNameUnsignedByte       types.TypeName = "unsignedByte"
	TypeNameNegativeInteger    types.TypeName = "negativeInteger"
	TypeNameNonPositiveInteger types.TypeName = "nonPositiveInteger"
)
