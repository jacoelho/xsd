package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *compiler) typeIDForType(typ model.Type) (runtime.TypeID, bool) {
	if c == nil || c.registry == nil || typ == nil {
		return 0, false
	}
	if bt, ok := model.AsBuiltinType(typ); ok && bt != nil {
		if id, ok := c.builtinTypeIDs[model.TypeName(bt.Name().Local)]; ok {
			return id, true
		}
	}
	if st, ok := model.AsSimpleType(typ); ok && st != nil {
		if st.IsBuiltin() {
			if builtin := builtins.Get(builtins.TypeName(st.Name().Local)); builtin != nil {
				if id, ok := c.builtinTypeIDs[model.TypeName(builtin.Name().Local)]; ok {
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
	if schemaID, ok := c.registry.LookupAnonymousTypeID(typ); ok {
		if id, ok := c.runtimeTypeIDs[schemaID]; ok {
			return id, true
		}
	}
	return 0, false
}
