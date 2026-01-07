package types

import (
	"fmt"

	lexicalparser "github.com/jacoelho/xsd/internal/parser/lexical"
)

// ValueParserFunc parses a lexical value and returns a TypedValue.
// The typ parameter is the type that should be used in the resulting TypedValue.
type ValueParserFunc func(lexical string, typ Type) (TypedValue, error)

func parserFor[T any](parse func(string) (T, error), newValue func(ParsedValue[T], *SimpleType) TypedValue) ValueParserFunc {
	return func(lexical string, typ Type) (TypedValue, error) {
		native, err := parse(lexical)
		if err != nil {
			return nil, err
		}
		st := asSimpleType(typ)
		if st == nil {
			return nil, fmt.Errorf("cannot create value for type %T", typ)
		}
		return newValue(NewParsedValue(lexical, native), st), nil
	}
}

// valueParsers maps type names to their parser functions.
// Compiled at initialization time - no init() function needed.
var valueParsers = map[TypeName]ValueParserFunc{
	TypeNameDecimal:       parserFor(lexicalparser.ParseDecimal, NewDecimalValue),
	TypeNameInteger:       parserFor(lexicalparser.ParseInteger, NewIntegerValue),
	TypeNameDateTime:      parserFor(lexicalparser.ParseDateTime, NewDateTimeValue),
	TypeNameBoolean:       parserFor(lexicalparser.ParseBoolean, NewBooleanValue),
	TypeNameFloat:         parserFor(lexicalparser.ParseFloat, NewFloatValue),
	TypeNameDouble:        parserFor(lexicalparser.ParseDouble, NewDoubleValue),
	TypeNameString:        parserFor(lexicalparser.ParseString, NewStringValue),
	TypeNameLong:          parserFor(lexicalparser.ParseLong, NewLongValue),
	TypeNameInt:           parserFor(lexicalparser.ParseInt, NewIntValue),
	TypeNameShort:         parserFor(lexicalparser.ParseShort, NewShortValue),
	TypeNameByte:          parserFor(lexicalparser.ParseByte, NewByteValue),
	TypeNameUnsignedLong:  parserFor(lexicalparser.ParseUnsignedLong, NewUnsignedLongValue),
	TypeNameUnsignedInt:   parserFor(lexicalparser.ParseUnsignedInt, NewUnsignedIntValue),
	TypeNameUnsignedShort: parserFor(lexicalparser.ParseUnsignedShort, NewUnsignedShortValue),
	TypeNameUnsignedByte:  parserFor(lexicalparser.ParseUnsignedByte, NewUnsignedByteValue),
}

// ParseValueForType parses a lexical value using the registry for the given type name.
func ParseValueForType(lexical string, typeName TypeName, typ Type) (TypedValue, error) {
	if parser, ok := valueParsers[typeName]; ok {
		return parser(lexical, typ)
	}
	return nil, fmt.Errorf("no parser for type %s", typeName)
}

// asSimpleType converts a Type to *SimpleType, creating a wrapper for BuiltinType if needed.
func asSimpleType(typ Type) *SimpleType {
	if st, ok := as[*SimpleType](typ); ok {
		return st
	}

	// for BuiltinType, create a SimpleType wrapper
	if bt, ok := as[*BuiltinType](typ); ok {
		st := &SimpleType{
			QName:   bt.qname,
			variety: AtomicVariety,
		}
		st.MarkBuiltin()
		return st
	}

	return nil
}