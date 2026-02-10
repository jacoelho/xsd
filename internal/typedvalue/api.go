package typedvalue

import (
	"fmt"
	"reflect"

	model "github.com/jacoelho/xsd/internal/model"
	typedvaluecore "github.com/jacoelho/xsd/internal/typedvalue/internalcore"
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
	return typedvaluecore.NormalizeValue(lexical, typ, typedvaluecore.NormalizeOps{
		IsNilType: func(typ any) bool {
			t, ok := typ.(model.Type)
			if !ok {
				return true
			}
			return isNilModelType(t)
		},
		IsBuiltinType: func(typ any) bool {
			t, ok := typ.(model.Type)
			return ok && !isNilModelType(t) && t.IsBuiltin()
		},
		TypeNameLocal: func(typ any) string {
			t, ok := typ.(model.Type)
			if !ok || isNilModelType(t) {
				return ""
			}
			return t.Name().Local
		},
		PrimitiveType: func(typ any) any {
			t, ok := typ.(model.Type)
			if !ok || isNilModelType(t) {
				return nil
			}
			return t.PrimitiveType()
		},
		WhiteSpaceMode: func(typ any) int {
			t, ok := typ.(model.Type)
			if !ok || isNilModelType(t) {
				return int(model.WhiteSpacePreserve)
			}
			return int(t.WhiteSpace())
		},
		ApplyWhiteSpace: func(lexical string, whiteSpaceMode int) string {
			return model.ApplyWhiteSpace(lexical, model.WhiteSpace(whiteSpaceMode))
		},
		TrimXMLWhitespace: model.TrimXMLWhitespace,
	})
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

func isNilModelType(typ model.Type) bool {
	if typ == nil {
		return true
	}
	value := reflect.ValueOf(typ)
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan:
		return value.IsNil()
	default:
		return false
	}
}
