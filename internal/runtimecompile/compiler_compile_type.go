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
