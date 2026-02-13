package validatorgen

import (
	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
)

func builtinForType(typ model.Type) *model.BuiltinType {
	if typ == nil {
		return nil
	}
	if bt, ok := model.AsBuiltinType(typ); ok {
		return bt
	}
	if st, ok := model.AsSimpleType(typ); ok && st.IsBuiltin() {
		return builtins.Get(model.TypeName(st.Name().Local))
	}
	return nil
}

func isIntegerTypeName(name string) bool {
	switch name {
	case "integer", "long", "int", "short", "byte",
		"unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte",
		"nonNegativeInteger", "positiveInteger", "negativeInteger", "nonPositiveInteger":
		return true
	default:
		return false
	}
}

func isAnySimpleType(typ model.Type) bool {
	bt := builtinForType(typ)
	if bt == nil {
		return false
	}
	return bt.Name().Local == string(builtins.TypeNameAnySimpleType)
}
