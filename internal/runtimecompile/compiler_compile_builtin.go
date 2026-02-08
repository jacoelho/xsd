package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

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
