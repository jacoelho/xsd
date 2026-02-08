package runtimecompile

import "github.com/jacoelho/xsd/internal/types"

func builtinForType(typ types.Type) *types.BuiltinType {
	if typ == nil {
		return nil
	}
	if bt, ok := types.AsBuiltinType(typ); ok {
		return bt
	}
	if st, ok := types.AsSimpleType(typ); ok && st.IsBuiltin() {
		return types.GetBuiltin(types.TypeName(st.Name().Local))
	}
	return nil
}

func isBuiltinListName(name string) bool {
	return types.IsBuiltinListTypeName(name)
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

func isAnySimpleType(typ types.Type) bool {
	bt := builtinForType(typ)
	if bt == nil {
		return false
	}
	return bt.Name().Local == string(types.TypeNameAnySimpleType)
}
