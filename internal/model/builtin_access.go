package model

import "fmt"

// GetBuiltin returns a built-in XSD type by local type name.
func GetBuiltin(name TypeName) *BuiltinType {
	return getBuiltin(name)
}

// GetBuiltinNS returns a built-in XSD type for an expanded name.
func GetBuiltinNS(namespace NamespaceURI, local string) *BuiltinType {
	return getBuiltinNS(namespace, local)
}

// MustBuiltin returns a built-in type and panics when unknown.
func MustBuiltin(name TypeName) *BuiltinType {
	item := GetBuiltin(name)
	if item != nil {
		return item
	}
	panic(fmt.Sprintf("model: unknown built-in type %s", name))
}

// IsBuiltin reports whether a QName resolves to a built-in type.
func IsBuiltin(qname QName) bool {
	return GetBuiltinNS(qname.Namespace, qname.Local) != nil
}

// IsBuiltinListTypeName reports whether name is one of the built-in list simple model.
func IsBuiltinListTypeName(name string) bool {
	return isBuiltinListTypeName(name)
}

// BuiltinListItemTypeName returns the built-in item type for a built-in list type.
func BuiltinListItemTypeName(name string) (TypeName, bool) {
	return builtinListItemTypeName(name)
}
