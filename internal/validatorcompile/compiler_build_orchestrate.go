package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

func Compile(sch *parser.Schema, registry *schema.Registry) (*CompiledValidators, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}

	comp := newCompiler(sch)
	comp.registry = registry
	comp.initRuntimeTypeIDs(registry)
	if err := comp.compileRegistry(registry); err != nil {
		return nil, err
	}
	if err := comp.compileDefaults(registry); err != nil {
		return nil, err
	}
	if err := comp.compileAttributeUses(registry); err != nil {
		return nil, err
	}
	return comp.result(registry), nil
}

func (c *compiler) initRuntimeTypeIDs(registry *schema.Registry) {
	if registry == nil {
		return
	}
	c.runtimeTypeIDs = make(map[schema.TypeID]runtime.TypeID, len(registry.TypeOrder))
	c.builtinTypeIDs = make(map[types.TypeName]runtime.TypeID, len(builtinTypeNames()))

	next := runtime.TypeID(1)
	for _, name := range builtinTypeNames() {
		c.builtinTypeIDs[name] = next
		next++
	}
	for _, entry := range registry.TypeOrder {
		c.runtimeTypeIDs[entry.ID] = next
		next++
	}
}
