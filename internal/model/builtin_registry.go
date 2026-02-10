package model

import "slices"

type builtinRegistry struct {
	byName  map[string]*BuiltinType
	ordered []*BuiltinType
}

func newBuiltinRegistry(byName map[string]*BuiltinType) *builtinRegistry {
	cloned := make(map[string]*BuiltinType, len(byName))
	names := make([]string, 0, len(byName))
	for name, builtin := range byName {
		cloned[name] = builtin
		names = append(names, name)
	}
	slices.Sort(names)
	ordered := make([]*BuiltinType, 0, len(names))
	for _, name := range names {
		if cloned[name] != nil {
			ordered = append(ordered, cloned[name])
		}
	}
	return &builtinRegistry{
		ordered: ordered,
		byName:  cloned,
	}
}

func (r *builtinRegistry) get(name TypeName) *BuiltinType {
	if r == nil {
		return nil
	}
	return r.byName[string(name)]
}

func (r *builtinRegistry) getNS(namespace NamespaceURI, local string) *BuiltinType {
	if r == nil || namespace != XSDNamespace {
		return nil
	}
	return r.byName[local]
}
