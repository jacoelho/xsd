package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
)

func builtinTypeNames() []model.TypeName {
	return []model.TypeName{
		builtins.TypeNameAnyType,
		builtins.TypeNameAnySimpleType,
		builtins.TypeNameString,
		builtins.TypeNameBoolean,
		builtins.TypeNameDecimal,
		builtins.TypeNameFloat,
		builtins.TypeNameDouble,
		builtins.TypeNameDuration,
		builtins.TypeNameDateTime,
		builtins.TypeNameTime,
		builtins.TypeNameDate,
		builtins.TypeNameGYearMonth,
		builtins.TypeNameGYear,
		builtins.TypeNameGMonthDay,
		builtins.TypeNameGDay,
		builtins.TypeNameGMonth,
		builtins.TypeNameHexBinary,
		builtins.TypeNameBase64Binary,
		builtins.TypeNameAnyURI,
		builtins.TypeNameQName,
		builtins.TypeNameNOTATION,
		builtins.TypeNameNormalizedString,
		builtins.TypeNameToken,
		builtins.TypeNameLanguage,
		builtins.TypeNameName,
		builtins.TypeNameNCName,
		builtins.TypeNameID,
		builtins.TypeNameIDREF,
		builtins.TypeNameIDREFS,
		builtins.TypeNameENTITY,
		builtins.TypeNameENTITIES,
		builtins.TypeNameNMTOKEN,
		builtins.TypeNameNMTOKENS,
		builtins.TypeNameInteger,
		builtins.TypeNameLong,
		builtins.TypeNameInt,
		builtins.TypeNameShort,
		builtins.TypeNameByte,
		builtins.TypeNameNonNegativeInteger,
		builtins.TypeNamePositiveInteger,
		builtins.TypeNameUnsignedLong,
		builtins.TypeNameUnsignedInt,
		builtins.TypeNameUnsignedShort,
		builtins.TypeNameUnsignedByte,
		builtins.TypeNameNonPositiveInteger,
		builtins.TypeNameNegativeInteger,
	}
}
