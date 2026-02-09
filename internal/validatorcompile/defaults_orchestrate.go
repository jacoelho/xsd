package validatorcompile

import (
	"fmt"

	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileDefaults(registry *schema.Registry) error {
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

func (c *compiler) compileAttributeUses(registry *schema.Registry) error {
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	for _, entry := range registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		attrs, _, err := CollectAttributeUses(c.schema, ct)
		if err != nil {
			return err
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
