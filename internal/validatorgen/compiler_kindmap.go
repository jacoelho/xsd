package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

type mapResolver map[string]string

func (c *compiler) validatorKind(st *model.SimpleType) (runtime.ValidatorKind, error) {
	primName, err := c.res.primitiveName(st)
	if err != nil {
		return 0, err
	}
	if primName == "decimal" && c.res.isIntegerDerived(st) {
		return runtime.VInteger, nil
	}
	return builtinValidatorKind(primName)
}

func builtinValidatorKind(name string) (runtime.ValidatorKind, error) {
	switch name {
	case "anySimpleType":
		return runtime.VString, nil
	case "string", "normalizedString", "token", "language", "Name", "NCName", "ID", "IDREF", "ENTITY", "NMTOKEN":
		return runtime.VString, nil
	case "boolean":
		return runtime.VBoolean, nil
	case "decimal":
		return runtime.VDecimal, nil
	case "integer", "long", "int", "short", "byte", "unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte", "nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		return runtime.VInteger, nil
	case "float":
		return runtime.VFloat, nil
	case "double":
		return runtime.VDouble, nil
	case "duration":
		return runtime.VDuration, nil
	case "dateTime":
		return runtime.VDateTime, nil
	case "time":
		return runtime.VTime, nil
	case "date":
		return runtime.VDate, nil
	case "gYearMonth":
		return runtime.VGYearMonth, nil
	case "gYear":
		return runtime.VGYear, nil
	case "gMonthDay":
		return runtime.VGMonthDay, nil
	case "gDay":
		return runtime.VGDay, nil
	case "gMonth":
		return runtime.VGMonth, nil
	case "anyURI":
		return runtime.VAnyURI, nil
	case "QName":
		return runtime.VQName, nil
	case "NOTATION":
		return runtime.VNotation, nil
	case "hexBinary":
		return runtime.VHexBinary, nil
	case "base64Binary":
		return runtime.VBase64Binary, nil
	default:
		return 0, fmt.Errorf("unsupported validator kind %s", name)
	}
}

func (c *compiler) stringKindForType(typ model.Type) runtime.StringKind {
	if c == nil || c.res == nil {
		return runtime.StringAny
	}
	name, ok := c.res.builtinNameForType(typ)
	if !ok {
		return runtime.StringAny
	}
	return stringKindForBuiltin(string(name))
}

func (c *compiler) integerKindForType(typ model.Type) runtime.IntegerKind {
	if c == nil || c.res == nil {
		return runtime.IntegerAny
	}
	name, ok := c.res.builtinNameForType(typ)
	if !ok {
		return runtime.IntegerAny
	}
	return integerKindForBuiltin(string(name))
}

func stringKindForBuiltin(name string) runtime.StringKind {
	switch name {
	case "normalizedString":
		return runtime.StringNormalized
	case "token":
		return runtime.StringToken
	case "language":
		return runtime.StringLanguage
	case "Name":
		return runtime.StringName
	case "NCName":
		return runtime.StringNCName
	case "ID":
		return runtime.StringID
	case "IDREF":
		return runtime.StringIDREF
	case "ENTITY":
		return runtime.StringEntity
	case "NMTOKEN":
		return runtime.StringNMTOKEN
	default:
		return runtime.StringAny
	}
}

func integerKindForBuiltin(name string) runtime.IntegerKind {
	switch name {
	case "long":
		return runtime.IntegerLong
	case "int":
		return runtime.IntegerInt
	case "short":
		return runtime.IntegerShort
	case "byte":
		return runtime.IntegerByte
	case "nonNegativeInteger":
		return runtime.IntegerNonNegative
	case "positiveInteger":
		return runtime.IntegerPositive
	case "nonPositiveInteger":
		return runtime.IntegerNonPositive
	case "negativeInteger":
		return runtime.IntegerNegative
	case "unsignedLong":
		return runtime.IntegerUnsignedLong
	case "unsignedInt":
		return runtime.IntegerUnsignedInt
	case "unsignedShort":
		return runtime.IntegerUnsignedShort
	case "unsignedByte":
		return runtime.IntegerUnsignedByte
	default:
		return runtime.IntegerAny
	}
}

func (m mapResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	if m == nil {
		return nil, false
	}
	ns, ok := m[string(prefix)]
	if !ok {
		return nil, false
	}
	return []byte(ns), true
}
