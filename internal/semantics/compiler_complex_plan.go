package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

func (c *compiler) prepareComplexTypePlan(registry *analysis.Registry) error {
	if c.complexTypes != nil {
		return nil
	}
	complexTypes, err := c.buildComplexTypes(registry)
	if err != nil {
		return err
	}
	c.complexTypes = complexTypes
	return nil
}

func (c *compiler) buildComplexTypes(registry *analysis.Registry) (*ComplexTypes, error) {
	if c == nil {
		return nil, fmt.Errorf("compiler is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	plan, err := buildComplexTypePlanEntries(registry, ComplexTypePlanFuncs{
		AttributeUses: func(ct *model.ComplexType) ([]*model.AttributeDecl, *model.AnyAttribute, error) {
			return CollectAttributeUses(c.schema, ct)
		},
		ContentParticle: func(ct *model.ComplexType) model.Particle {
			return EffectiveContentParticle(c.schema, ct)
		},
		SimpleContentType: func(ct *model.ComplexType) (model.Type, error) {
			return c.simpleContentTextType(ct)
		},
	})
	if err != nil {
		return nil, err
	}
	return &ComplexTypes{plan: plan}, nil
}
