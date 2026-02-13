package builtins

import "github.com/jacoelho/xsd/internal/types"

// Get returns the built-in type by local type name.
func Get(name types.TypeName) *types.BuiltinType {
	return defaultRegistry.byName[name]
}

// MustGet returns the built-in type and panics when unknown.
func MustGet(name types.TypeName) *types.BuiltinType {
	item := Get(name)
	if item != nil {
		return item
	}
	panic("builtins: unknown type " + string(name))
}

// GetNS returns the built-in type for an expanded name.
func GetNS(namespace types.NamespaceURI, local string) *types.BuiltinType {
	if namespace != XSDNamespace {
		return nil
	}
	return Get(types.TypeName(local))
}

// List returns built-in types in deterministic order.
func List() []*types.BuiltinType {
	if len(defaultRegistry.ordered) == 0 {
		return nil
	}
	items := make([]*types.BuiltinType, len(defaultRegistry.ordered))
	copy(items, defaultRegistry.ordered)
	return items
}

// IsBuiltin reports whether a QName resolves to a built-in type.
func IsBuiltin(qname types.QName) bool {
	return GetNS(qname.Namespace, qname.Local) != nil
}
