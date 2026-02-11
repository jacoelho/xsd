package validatorgen

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/complextypeplan"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
	"github.com/jacoelho/xsd/internal/typechain"
)

func (c *compiler) prepareComplexTypePlan(registry *schema.Registry) error {
	if c.complexTypes != nil {
		return nil
	}
	plan, err := c.buildComplexTypePlan(registry)
	if err != nil {
		return err
	}
	c.complexTypes = plan
	return nil
}

func (c *compiler) buildComplexTypePlan(registry *schema.Registry) (*complextypeplan.Plan, error) {
	if c == nil {
		return nil, fmt.Errorf("compiler is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	plan, err := complextypeplan.Build(registry, complextypeplan.ComputeFuncs{
		AttributeUses: func(ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error) {
			return CollectAttributeUses(c.schema, ct)
		},
		ContentParticle: func(ct *model.ComplexType) model.Particle {
			return typechain.EffectiveContentParticle(c.schema, ct)
		},
		SimpleContentType: func(ct *model.ComplexType) (model.Type, error) {
			return c.simpleContentTextType(ct)
		},
	})
	if err != nil {
		return nil, err
	}
	return plan, nil
}

// BuildComplexTypePlan precomputes shared complex-type artifacts for compile/build phases.
func BuildComplexTypePlan(sch *parser.Schema, registry *schema.Registry) (*complextypeplan.Plan, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	comp := newCompiler(sch)
	comp.registry = registry
	return comp.buildComplexTypePlan(registry)
}
