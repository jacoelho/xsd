package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileSimpleType(st *types.SimpleType) (runtime.ValidatorID, error) {
	if st == nil {
		return 0, nil
	}
	if types.IsPlaceholderSimpleType(st) {
		return 0, fmt.Errorf("placeholder simpleType")
	}
	if err := c.compileSimpleTypeDependencies(st); err != nil {
		return 0, err
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
	return c.compileSimpleTypeValidator(st, facetRef)
}

func (c *compiler) compileSimpleTypeDependencies(st *types.SimpleType) error {
	if base := c.res.baseType(st); base != nil {
		if _, err := c.compileType(base); err != nil {
			return err
		}
	}

	switch c.res.variety(st) {
	case types.ListVariety:
		item, ok := c.res.listItemTypeFromType(st)
		if !ok || item == nil {
			return fmt.Errorf("list type missing item type")
		}
		if _, err := c.compileType(item); err != nil {
			return err
		}
	case types.UnionVariety:
		members := c.res.unionMemberTypesFromType(st)
		if len(members) == 0 {
			return fmt.Errorf("union has no member types")
		}
		for _, member := range members {
			if _, err := c.compileType(member); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *compiler) compileSimpleTypeValidator(st *types.SimpleType, facetRef runtime.FacetProgramRef) (runtime.ValidatorID, error) {
	ws := c.res.whitespaceMode(st)
	switch c.res.variety(st) {
	case types.ListVariety:
		item, _ := c.res.listItemTypeFromType(st)
		itemID, err := c.compileType(item)
		if err != nil {
			return 0, err
		}
		return c.addListValidator(ws, facetRef, itemID), nil
	case types.UnionVariety:
		return c.compileUnionValidator(st, ws, facetRef)
	default:
		kind, err := c.validatorKind(st)
		if err != nil {
			return 0, err
		}
		return c.addAtomicValidator(kind, ws, facetRef, c.stringKindForType(st), c.integerKindForType(st)), nil
	}
}

func (c *compiler) compileUnionValidator(
	st *types.SimpleType,
	ws runtime.WhitespaceMode,
	facetRef runtime.FacetProgramRef,
) (runtime.ValidatorID, error) {
	members := c.res.unionMemberTypesFromType(st)
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
}
