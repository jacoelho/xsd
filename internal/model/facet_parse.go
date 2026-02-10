package model

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/durationlex"
	"github.com/jacoelho/xsd/internal/value/temporal"
)

type unsupportedComparablePrimitiveError struct {
	primitive string
}

func (e unsupportedComparablePrimitiveError) Error() string {
	return fmt.Sprintf("unsupported primitive type %s", e.primitive)
}

func parseLexicalToComparable(lexical string, typ Type) (ComparableValue, error) {
	if typ == nil {
		return nil, ErrCannotDeterminePrimitiveType
	}

	typeName := typ.Name().Local
	if IsIntegerTypeName(typeName) {
		intVal, err := ParseInteger(lexical)
		if err != nil {
			return nil, err
		}
		return ComparableInt{Value: intVal, Typ: typ}, nil
	}

	primitiveName, err := comparablePrimitiveName(typ)
	if err != nil {
		return nil, err
	}

	switch primitiveName {
	case "decimal":
		if isIntegerDerivedType(typ) {
			intVal, err := ParseInteger(lexical)
			if err != nil {
				return nil, err
			}
			return ComparableInt{Value: intVal, Typ: typ}, nil
		}
		rat, err := ParseDecimal(lexical)
		if err != nil {
			return nil, err
		}
		return ComparableDec{Value: rat, Typ: typ}, nil

	case "float":
		floatVal, err := ParseFloat(lexical)
		if err != nil {
			return nil, err
		}
		return ComparableFloat32{Value: floatVal, Typ: typ}, nil

	case "double":
		doubleVal, err := ParseDouble(lexical)
		if err != nil {
			return nil, err
		}
		return ComparableFloat64{Value: doubleVal, Typ: typ}, nil

	case "duration":
		xsdDur, err := durationlex.Parse(lexical)
		if err != nil {
			return nil, err
		}
		return ComparableXSDDuration{Value: xsdDur, Typ: typ}, nil
	default:
		if !IsDateTimeTypeName(primitiveName) {
			return nil, unsupportedComparablePrimitiveError{primitive: primitiveName}
		}
		timeVal, err := temporal.ParsePrimitive(primitiveName, []byte(lexical))
		if err != nil {
			return nil, err
		}
		return ComparableTime{
			Value:        timeVal.Time,
			Typ:          typ,
			TimezoneKind: TimezoneKind(lexical),
			Kind:         timeVal.Kind,
			LeapSecond:   timeVal.LeapSecond,
		}, nil
	}
}

func comparablePrimitiveName(typ Type) (string, error) {
	if typ == nil {
		return "", ErrCannotDeterminePrimitiveType
	}
	primitiveType := typ.PrimitiveType()
	if primitiveType == nil {
		return "", ErrCannotDeterminePrimitiveType
	}
	return primitiveType.Name().Local, nil
}

func comparableParseErrorCategory(typ Type) string {
	if typ == nil {
		return "string"
	}
	typeName := typ.Name().Local
	if IsIntegerTypeName(typeName) {
		return "integer"
	}

	primitiveName, err := comparablePrimitiveName(typ)
	if err != nil {
		return "string"
	}
	if primitiveName == "decimal" && isIntegerDerivedType(typ) {
		return "integer"
	}
	switch primitiveName {
	case "decimal":
		return "decimal"
	case "float":
		return "float"
	case "double":
		return "double"
	case "duration":
		return "duration"
	default:
		if IsDateTimeTypeName(primitiveName) {
			return "date/time"
		}
		return "string"
	}
}
