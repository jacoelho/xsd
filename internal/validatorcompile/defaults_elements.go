package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	schema "github.com/jacoelho/xsd/internal/semantic"
)

func (c *compiler) compileElementDefaults(registry *schema.Registry) error {
	for _, entry := range registry.ElementOrder {
		decl := entry.Decl
		if decl == nil || decl.IsReference {
			continue
		}
		if st, ok := model.AsSimpleType(decl.Type); ok && model.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("element %s type not resolved", entry.QName)
		}
		var typ model.Type
		if decl.HasDefault || decl.HasFixed {
			var err error
			typ, err = c.valueTypeForElement(decl)
			if err != nil {
				return fmt.Errorf("element %s: %w", entry.QName, err)
			}
		}
		if decl.HasDefault {
			value, err := c.compileDefaultFixedValue(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("element %s default: %w", entry.QName, err)
			}
			c.elements.storeDefault(entry.ID, value)
		}
		if decl.HasFixed {
			value, err := c.compileDefaultFixedValue(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("element %s fixed: %w", entry.QName, err)
			}
			c.elements.storeFixed(entry.ID, value)
		}
	}
	return nil
}
