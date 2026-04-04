package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
)

func (c *compiler) compileDefaults(registry *analysis.Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	if err := c.compileElementDefaults(registry); err != nil {
		return err
	}
	if err := c.compileAttributeDefaults(registry); err != nil {
		return err
	}
	return nil
}

func (c *compiler) compileAttributeUses(registry *analysis.Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	for _, entry := range registry.TypeOrder {
		ct, ok := model.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		attrs, _, ok := c.complexTypes.AttributeUses(ct)
		if !ok {
			return fmt.Errorf("complex type %s missing derived attributes", ct.QName)
		}
		for _, decl := range attrs {
			if decl == nil {
				continue
			}
			if err := c.compileAttributeUseDefaultFixed(decl); err != nil {
				return err
			}
		}
	}
	return nil
}
