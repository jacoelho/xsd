package validatorbuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *artifactCompiler) compileType(typ model.Type) (runtime.ValidatorID, error) {
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
	case *model.SimpleType:
		id, err := c.compileSimpleType(t)
		if err != nil {
			return 0, err
		}
		c.validatorByType[key] = id
		return id, nil
	case *model.BuiltinType:
		id, err := c.compileBuiltin(t)
		if err != nil {
			return 0, err
		}
		c.validatorByType[key] = id
		return id, nil
	}
	return 0, nil
}

func (c *artifactCompiler) canonicalTypeKey(typ model.Type) model.Type {
	if st, ok := model.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := model.GetBuiltin(model.TypeName(st.Name().Local)); builtin != nil {
			return builtin
		}
	}
	return typ
}
