package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileType(typ types.Type) (runtime.ValidatorID, error) {
	if typ == nil {
		return 0, nil
	}
	key := c.canonicalTypeKey(typ)
	if id, ok := c.validatorByType[key]; ok {
		return id, nil
	}
	if c.compiling[key] {
		return 0, fmt.Errorf("validator cycle detected")
	}
	c.compiling[key] = true
	defer delete(c.compiling, key)

	switch t := key.(type) {
	case *types.SimpleType:
		id, err := c.compileSimpleType(t)
		if err != nil {
			return 0, err
		}
		c.validatorByType[key] = id
		return id, nil
	case *types.BuiltinType:
		id, err := c.compileBuiltin(t)
		if err != nil {
			return 0, err
		}
		c.validatorByType[key] = id
		return id, nil
	default:
		return 0, nil
	}
}

func (c *compiler) canonicalTypeKey(typ types.Type) types.Type {
	if st, ok := types.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
			return builtin
		}
	}
	return typ
}

func (c *compiler) compileBuiltin(bt *types.BuiltinType) (runtime.ValidatorID, error) {
	name := bt.Name().Local
	ws := c.res.whitespaceMode(bt)

	if itemName, ok := types.BuiltinListItemTypeName(name); ok {
		itemType := types.GetBuiltin(itemName)
		if itemType == nil {
			return 0, fmt.Errorf("builtin list %s: item type %s not found", name, itemName)
		}
		itemID, err := c.compileType(itemType)
		if err != nil {
			return 0, err
		}
		start := len(c.facets)
		c.facets = append(c.facets, runtime.FacetInstr{Op: runtime.FMinLength, Arg0: 1})
		facetRef := runtime.FacetProgramRef{Off: uint32(start), Len: 1}
		return c.addListValidator(ws, facetRef, itemID), nil
	}

	kind, err := builtinValidatorKind(name)
	if err != nil {
		return 0, err
	}
	return c.addAtomicValidator(kind, ws, runtime.FacetProgramRef{}, stringKindForBuiltin(name), integerKindForBuiltin(name)), nil
}

func (c *compiler) compileSimpleType(st *types.SimpleType) (runtime.ValidatorID, error) {
	if st == nil {
		return 0, nil
	}
	if types.IsPlaceholderSimpleType(st) {
		return 0, fmt.Errorf("placeholder simpleType")
	}

	if base := c.res.baseType(st); base != nil {
		if _, err := c.compileType(base); err != nil {
			return 0, err
		}
	}

	switch c.res.variety(st) {
	case types.ListVariety:
		item, ok := c.res.listItemType(st)
		if !ok || item == nil {
			return 0, fmt.Errorf("list type missing item type")
		}
		if _, err := c.compileType(item); err != nil {
			return 0, err
		}
	case types.UnionVariety:
		members := c.res.unionMemberTypes(st)
		if len(members) == 0 {
			return 0, fmt.Errorf("union has no member types")
		}
		for _, member := range members {
			if _, err := c.compileType(member); err != nil {
				return 0, err
			}
		}
	}

	facets, err := c.collectFacets(st)
	if err != nil {
		return 0, err
	}
	partialFacets := filterFacets(facets, func(f types.Facet) bool {
		_, ok := f.(*types.Enumeration)
		return !ok
	})

	facetRef, err := c.compileFacetProgram(st, facets, partialFacets)
	if err != nil {
		return 0, err
	}

	ws := c.res.whitespaceMode(st)
	switch c.res.variety(st) {
	case types.ListVariety:
		item, _ := c.res.listItemType(st)
		itemID, err := c.compileType(item)
		if err != nil {
			return 0, err
		}
		return c.addListValidator(ws, facetRef, itemID), nil
	case types.UnionVariety:
		members := c.res.unionMemberTypes(st)
		memberIDs := make([]runtime.ValidatorID, 0, len(members))
		memberTypeIDs := make([]runtime.TypeID, 0, len(members))
		for _, member := range members {
			id, err := c.compileType(member)
			if err != nil {
				return 0, err
			}
			memberIDs = append(memberIDs, id)
			typeID, ok := c.typeIDForType(member)
			if !ok {
				return 0, fmt.Errorf("union member type id not found")
			}
			memberTypeIDs = append(memberTypeIDs, typeID)
		}
		typeID, _ := c.typeIDForType(st)
		return c.addUnionValidator(ws, facetRef, memberIDs, memberTypeIDs, st.QName.String(), typeID)
	default:
		kind, err := c.validatorKind(st)
		if err != nil {
			return 0, err
		}
		return c.addAtomicValidator(kind, ws, facetRef, c.stringKindForType(st), c.integerKindForType(st)), nil
	}
}

func (c *compiler) typeIDForType(typ types.Type) (runtime.TypeID, bool) {
	if c == nil || c.registry == nil || typ == nil {
		return 0, false
	}
	if bt, ok := types.AsBuiltinType(typ); ok && bt != nil {
		if id, ok := c.builtinTypeIDs[types.TypeName(bt.Name().Local)]; ok {
			return id, true
		}
	}
	if st, ok := types.AsSimpleType(typ); ok && st != nil {
		if st.IsBuiltin() {
			if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
				if id, ok := c.builtinTypeIDs[types.TypeName(builtin.Name().Local)]; ok {
					return id, true
				}
			}
		}
		if name := st.Name(); !name.IsZero() {
			if schemaID, ok := c.registry.Types[name]; ok {
				if id, ok := c.runtimeTypeIDs[schemaID]; ok {
					return id, true
				}
			}
		}
	}
	if schemaID, ok := c.registry.AnonymousTypes[typ]; ok {
		if id, ok := c.runtimeTypeIDs[schemaID]; ok {
			return id, true
		}
	}
	return 0, false
}
