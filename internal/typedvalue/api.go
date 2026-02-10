package typedvalue

import (
	"fmt"

	model "github.com/jacoelho/xsd/internal/model"
)

var supportedTypeNames = map[model.TypeName]struct{}{
	model.TypeNameDecimal:       {},
	model.TypeNameInteger:       {},
	model.TypeNameDateTime:      {},
	model.TypeNameTime:          {},
	model.TypeNameDate:          {},
	model.TypeNameDuration:      {},
	model.TypeNameGYear:         {},
	model.TypeNameGYearMonth:    {},
	model.TypeNameGMonth:        {},
	model.TypeNameGMonthDay:     {},
	model.TypeNameGDay:          {},
	model.TypeNameBoolean:       {},
	model.TypeNameFloat:         {},
	model.TypeNameDouble:        {},
	model.TypeNameString:        {},
	model.TypeNameLong:          {},
	model.TypeNameInt:           {},
	model.TypeNameShort:         {},
	model.TypeNameByte:          {},
	model.TypeNameUnsignedLong:  {},
	model.TypeNameUnsignedInt:   {},
	model.TypeNameUnsignedShort: {},
	model.TypeNameUnsignedByte:  {},
	model.TypeNameHexBinary:     {},
	model.TypeNameBase64Binary:  {},
}

// Normalize applies XSD whitespace normalization for a lexical value and type.
func Normalize(lexical string, typ model.Type) (string, error) {
	return model.NormalizeTypeValue(lexical, typ)
}

// ParseForType parses a lexical value for a supported XSD type name.
func ParseForType(lexical string, typeName model.TypeName, typ model.Type) (model.TypedValue, error) {
	if _, ok := supportedTypeNames[typeName]; !ok {
		return nil, fmt.Errorf("no parser for type %s", typeName)
	}
	if simpleType, ok := model.AsSimpleType(typ); ok && simpleType != nil {
		return simpleType.ParseValue(lexical)
	}
	if builtin, ok := model.AsBuiltinType(typ); ok && builtin != nil {
		return builtin.ParseValue(lexical)
	}
	return nil, fmt.Errorf("cannot parse value for type %T", typ)
}
