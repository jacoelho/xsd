package builtins

import schematypes "github.com/jacoelho/xsd/internal/types"

// Get returns the built-in type by local type name.
func Get(name schematypes.TypeName) *schematypes.BuiltinType {
	return defaultRegistry.byName[name]
}

// MustGet returns the built-in type and panics when unknown.
func MustGet(name schematypes.TypeName) *schematypes.BuiltinType {
	item := Get(name)
	if item != nil {
		return item
	}
	panic("builtins: unknown type " + string(name))
}

// GetNS returns the built-in type for an expanded name.
func GetNS(namespace schematypes.NamespaceURI, local string) *schematypes.BuiltinType {
	if namespace != XSDNamespace {
		return nil
	}
	return Get(schematypes.TypeName(local))
}

// List returns built-in types in deterministic order.
func List() []*schematypes.BuiltinType {
	if len(defaultRegistry.ordered) == 0 {
		return nil
	}
	items := make([]*schematypes.BuiltinType, len(defaultRegistry.ordered))
	copy(items, defaultRegistry.ordered)
	return items
}

// IsBuiltin reports whether a QName resolves to a built-in type.
func IsBuiltin(qname schematypes.QName) bool {
	return GetNS(qname.Namespace, qname.Local) != nil
}
