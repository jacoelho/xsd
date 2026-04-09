package validatorbuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/parser"
)

func compileValidatorArtifactsWithPlan(
	sch *parser.Schema,
	registry *analysis.Registry,
	complexTypes *complexplan.ComplexTypes,
) (*ValidatorArtifacts, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if complexTypes == nil {
		return nil, fmt.Errorf("complex types are nil")
	}

	comp := newArtifactCompiler(sch)
	comp.registry = registry
	comp.complexTypes = complexTypes
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

func (c *artifactCompiler) initRuntimeTypeIDs(registry *analysis.Registry) {
	if registry == nil {
		return
	}
	plan, err := analysis.BuildRuntimeIDPlan(registry)
	if err != nil {
		return
	}
	c.runtimeTypeIDs = plan.TypeIDs
	c.builtinTypeIDs = plan.BuiltinTypeIDs
}
