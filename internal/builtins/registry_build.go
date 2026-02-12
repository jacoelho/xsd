package builtins

import schematypes "github.com/jacoelho/xsd/internal/types"

func newRegistry(items []*schematypes.BuiltinType) registry {
	byName := make(map[schematypes.TypeName]*schematypes.BuiltinType, len(items))
	ordered := make([]*schematypes.BuiltinType, 0, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}
		name := schematypes.TypeName(item.Name().Local)
		if _, exists := byName[name]; exists {
			continue
		}
		byName[name] = item
		ordered = append(ordered, item)
	}

	return registry{
		byName:  byName,
		ordered: ordered,
	}
}
