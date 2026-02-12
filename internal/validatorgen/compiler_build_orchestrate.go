package validatorgen

import (
	"fmt"

	schema "github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtimeids"
)

func Compile(sch *parser.Schema, registry *schema.Registry) (*CompiledValidators, error) {
	return CompileWithComplexTypePlan(sch, registry, nil)
}

// CompileWithComplexTypePlan compiles validators, optionally reusing a precomputed complex-type plan.
func CompileWithComplexTypePlan(
	sch *parser.Schema,
	registry *schema.Registry,
	complexTypes *complextypeplan.Plan,
) (*CompiledValidators, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}

	comp := newCompiler(sch)
	comp.registry = registry
	comp.complexTypes = complexTypes
	comp.initRuntimeTypeIDs(registry)
	if err := comp.compileRegistry(registry); err != nil {
		return nil, err
	}
	if err := comp.prepareComplexTypePlan(registry); err != nil {
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
	plan, err := runtimeids.Build(registry)
	if err != nil {
		return
	}
	c.runtimeTypeIDs = plan.TypeIDs
	c.builtinTypeIDs = plan.BuiltinTypeIDs
}
