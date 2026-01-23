package types

import "fmt"

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
	TypeNameDecimal:       parserFor(ParseDecimal, NewDecimalValue),
	TypeNameInteger:       parserFor(ParseInteger, NewIntegerValue),
	TypeNameDateTime:      parserFor(ParseDateTime, NewDateTimeValue),
	TypeNameTime:          parserFor(ParseTime, NewDateTimeValue),
	TypeNameDate:          parserFor(ParseDate, NewDateTimeValue),
	TypeNameGYear:         parserFor(ParseGYear, NewDateTimeValue),
	TypeNameGYearMonth:    parserFor(ParseGYearMonth, NewDateTimeValue),
	TypeNameGMonth:        parserFor(ParseGMonth, NewDateTimeValue),
	TypeNameGMonthDay:     parserFor(ParseGMonthDay, NewDateTimeValue),
	TypeNameGDay:          parserFor(ParseGDay, NewDateTimeValue),
	TypeNameBoolean:       parserFor(ParseBoolean, NewBooleanValue),
	TypeNameFloat:         parserFor(ParseFloat, NewFloatValue),
	TypeNameDouble:        parserFor(ParseDouble, NewDoubleValue),
	TypeNameString:        parserFor(ParseString, NewStringValue),
	TypeNameLong:          parserFor(ParseLong, NewLongValue),
	TypeNameInt:           parserFor(ParseInt, NewIntValue),
	TypeNameShort:         parserFor(ParseShort, NewShortValue),
	TypeNameByte:          parserFor(ParseByte, NewByteValue),
	TypeNameUnsignedLong:  parserFor(ParseUnsignedLong, NewUnsignedLongValue),
	TypeNameUnsignedInt:   parserFor(ParseUnsignedInt, NewUnsignedIntValue),
	TypeNameUnsignedShort: parserFor(ParseUnsignedShort, NewUnsignedShortValue),
	TypeNameUnsignedByte:  parserFor(ParseUnsignedByte, NewUnsignedByteValue),
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
		if bt.simpleWrapper == nil {
			bt.simpleWrapper = newBuiltinSimpleType(bt)
		}
		return bt.simpleWrapper
	}

	return nil
}
