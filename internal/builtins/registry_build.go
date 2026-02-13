package builtins

import "github.com/jacoelho/xsd/internal/types"

func newRegistry(items []*types.BuiltinType) registry {
	byName := make(map[types.TypeName]*types.BuiltinType, len(items))
	ordered := make([]*types.BuiltinType, 0, len(items))

	for _, item := range items {
		if item == nil {
			continue
		}
		name := types.TypeName(item.Name().Local)
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
