package valuebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

type mapResolver map[string]string

func validatorKind(spec schemair.SimpleTypeSpec) (runtime.ValidatorKind, error) {
	primitive := primitiveForSpec(spec)
	if primitive == "decimal" && spec.IntegerDerived {
		return runtime.VInteger, nil
	}
	return builtinValidatorKind(primitive)
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

func specBuiltinName(spec schemair.SimpleTypeSpec) string {
	if spec.BuiltinBase != "" {
		return spec.BuiltinBase
	}
	if spec.Name.Local != "" {
		return spec.Name.Local
	}
	return primitiveForSpec(spec)
}

func primitiveForSpec(spec schemair.SimpleTypeSpec) string {
	if spec.Primitive != "" {
		return spec.Primitive
	}
	if spec.Name.Local == "anyType" || spec.Name.Local == "anySimpleType" {
		return "anySimpleType"
	}
	if spec.BuiltinBase != "" {
		return spec.BuiltinBase
	}
	return spec.Name.Local
}

func runtimeWhitespace(mode schemair.WhitespaceMode) runtime.WhitespaceMode {
	switch mode {
	case schemair.WhitespaceReplace:
		return runtime.WSReplace
	case schemair.WhitespaceCollapse:
		return runtime.WSCollapse
	default:
		return runtime.WSPreserve
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

func formatName(name schemair.Name) string {
	if name.Namespace == "" {
		return name.Local
	}
	if name.Local == "" {
		return name.Namespace
	}
	return "{" + name.Namespace + "}" + name.Local
}
