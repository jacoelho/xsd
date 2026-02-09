package validatorcompile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

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
	if schemaID, ok := c.registry.LookupAnonymousTypeID(typ); ok {
		if id, ok := c.runtimeTypeIDs[schemaID]; ok {
			return id, true
		}
	}
	return 0, false
}
