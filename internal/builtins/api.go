package builtins

import (
	"github.com/jacoelho/xsd/internal/builtinlist"
	model "github.com/jacoelho/xsd/internal/model"
)

type TypeName = model.TypeName
type NamespaceURI = model.NamespaceURI
type BuiltinType = model.BuiltinType
type SimpleType = model.SimpleType

const (
	XSDNamespace = model.XSDNamespace
	XMLNamespace = model.XMLNamespace

	TypeNameAnyType       = model.TypeNameAnyType
	TypeNameAnySimpleType = model.TypeNameAnySimpleType

	TypeNameString       = model.TypeNameString
	TypeNameBoolean      = model.TypeNameBoolean
	TypeNameDecimal      = model.TypeNameDecimal
	TypeNameFloat        = model.TypeNameFloat
	TypeNameDouble       = model.TypeNameDouble
	TypeNameDuration     = model.TypeNameDuration
	TypeNameDateTime     = model.TypeNameDateTime
	TypeNameTime         = model.TypeNameTime
	TypeNameDate         = model.TypeNameDate
	TypeNameGYearMonth   = model.TypeNameGYearMonth
	TypeNameGYear        = model.TypeNameGYear
	TypeNameGMonthDay    = model.TypeNameGMonthDay
	TypeNameGDay         = model.TypeNameGDay
	TypeNameGMonth       = model.TypeNameGMonth
	TypeNameHexBinary    = model.TypeNameHexBinary
	TypeNameBase64Binary = model.TypeNameBase64Binary
	TypeNameAnyURI       = model.TypeNameAnyURI
	TypeNameQName        = model.TypeNameQName
	TypeNameNOTATION     = model.TypeNameNOTATION

	TypeNameNormalizedString = model.TypeNameNormalizedString
	TypeNameToken            = model.TypeNameToken
	TypeNameLanguage         = model.TypeNameLanguage
	TypeNameName             = model.TypeNameName
	TypeNameNCName           = model.TypeNameNCName
	TypeNameID               = model.TypeNameID
	TypeNameIDREF            = model.TypeNameIDREF
	TypeNameIDREFS           = model.TypeNameIDREFS
	TypeNameENTITY           = model.TypeNameENTITY
	TypeNameENTITIES         = model.TypeNameENTITIES
	TypeNameNMTOKEN          = model.TypeNameNMTOKEN
	TypeNameNMTOKENS         = model.TypeNameNMTOKENS

	TypeNameInteger            = model.TypeNameInteger
	TypeNameLong               = model.TypeNameLong
	TypeNameInt                = model.TypeNameInt
	TypeNameShort              = model.TypeNameShort
	TypeNameByte               = model.TypeNameByte
	TypeNameNonNegativeInteger = model.TypeNameNonNegativeInteger
	TypeNamePositiveInteger    = model.TypeNamePositiveInteger
	TypeNameUnsignedLong       = model.TypeNameUnsignedLong
	TypeNameUnsignedInt        = model.TypeNameUnsignedInt
	TypeNameUnsignedShort      = model.TypeNameUnsignedShort
	TypeNameUnsignedByte       = model.TypeNameUnsignedByte
	TypeNameNegativeInteger    = model.TypeNameNegativeInteger
	TypeNameNonPositiveInteger = model.TypeNameNonPositiveInteger
)

func Get(name TypeName) *BuiltinType {
	return model.GetBuiltin(name)
}

func GetNS(namespace NamespaceURI, local string) *BuiltinType {
	return model.GetBuiltinNS(namespace, local)
}

func NewSimpleType(name TypeName) (*SimpleType, error) {
	return model.NewBuiltinSimpleType(name)
}

func IsBuiltinListTypeName(name string) bool {
	return builtinlist.IsTypeName(name)
}

func BuiltinListItemTypeName(name string) (TypeName, bool) {
	item, ok := builtinlist.ItemTypeName(name)
	return TypeName(item), ok
}
