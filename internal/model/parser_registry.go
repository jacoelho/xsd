package model

import (
	"fmt"
	"time"

	"github.com/jacoelho/xsd/internal/durationlex"
	typedvaluecore "github.com/jacoelho/xsd/internal/typedvalue/internalcore"
	"github.com/jacoelho/xsd/internal/value"
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
	TypeNameDecimal:       parserFor(ParseDecimal, NewDecimalValue),
	TypeNameInteger:       parserFor(ParseInteger, NewIntegerValue),
	TypeNameDateTime:      parserFor(ParseDateTime, NewDateTimeValue),
	TypeNameTime:          parserFor(func(lexical string) (time.Time, error) { return value.ParseTime([]byte(lexical)) }, NewDateTimeValue),
	TypeNameDate:          parserFor(func(lexical string) (time.Time, error) { return value.ParseDate([]byte(lexical)) }, NewDateTimeValue),
	TypeNameDuration:      parserFor(durationlex.Parse, NewXSDDurationValue),
	TypeNameGYear:         parserFor(func(lexical string) (time.Time, error) { return value.ParseGYear([]byte(lexical)) }, NewDateTimeValue),
	TypeNameGYearMonth:    parserFor(func(lexical string) (time.Time, error) { return value.ParseGYearMonth([]byte(lexical)) }, NewDateTimeValue),
	TypeNameGMonth:        parserFor(func(lexical string) (time.Time, error) { return value.ParseGMonth([]byte(lexical)) }, NewDateTimeValue),
	TypeNameGMonthDay:     parserFor(func(lexical string) (time.Time, error) { return value.ParseGMonthDay([]byte(lexical)) }, NewDateTimeValue),
	TypeNameGDay:          parserFor(func(lexical string) (time.Time, error) { return value.ParseGDay([]byte(lexical)) }, NewDateTimeValue),
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
	TypeNameHexBinary:     parserFor(ParseHexBinary, NewHexBinaryValue),
	TypeNameBase64Binary:  parserFor(ParseBase64Binary, NewBase64BinaryValue),
}

var coreValueParsers = func() map[string]typedvaluecore.ValueParserFunc {
	parsers := make(map[string]typedvaluecore.ValueParserFunc, len(valueParsers))
	for typeName, parser := range valueParsers {
		name := string(typeName)
		parse := parser
		parsers[name] = func(lexical string, typ any) (any, error) {
			modelType, ok := typ.(Type)
			if !ok {
				return nil, fmt.Errorf("cannot parse value for type %T", typ)
			}
			return parse(lexical, modelType)
		}
	}
	return parsers
}()

// parseValueForType parses a lexical value using the registry for the given type name.
func parseValueForType(lexical string, typeName TypeName, typ Type) (TypedValue, error) {
	parsed, err := typedvaluecore.ParseValueForType(lexical, string(typeName), typ, coreValueParsers)
	if err != nil {
		return nil, err
	}
	typed, ok := parsed.(TypedValue)
	if !ok {
		return nil, fmt.Errorf("parser for type %s returned %T", typeName, parsed)
	}
	return typed, nil
}

// asSimpleType converts a Type to *SimpleType, creating a wrapper for BuiltinType if needed.
func asSimpleType(typ Type) *SimpleType {
	if st, ok := as[*SimpleType](typ); ok {
		return st
	}

	// for BuiltinType, create a SimpleType wrapper
	if bt, ok := as[*BuiltinType](typ); ok {
		if bt.simpleWrapper != nil {
			return bt.simpleWrapper
		}
		// avoid unsynchronized mutation; fallback to a new wrapper if needed
		return newBuiltinSimpleType(bt)
	}

	return nil
}
