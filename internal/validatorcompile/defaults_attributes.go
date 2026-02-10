package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	schema "github.com/jacoelho/xsd/internal/semantic"
)

func (c *compiler) compileAttributeDefaults(registry *schema.Registry) error {
	for _, entry := range registry.AttributeOrder {
		decl := entry.Decl
		if decl == nil {
			continue
		}
		if st, ok := model.AsSimpleType(decl.Type); ok && model.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("attribute %s type not resolved", entry.QName)
		}
		var typ model.Type
		if decl.HasDefault || decl.HasFixed {
			var err error
			typ, err = c.valueTypeForAttribute(decl)
			if err != nil {
				return fmt.Errorf("attribute %s: %w", entry.QName, err)
			}
		}
		if decl.HasDefault {
			value, err := c.compileDefaultFixedValue(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("attribute %s default: %w", entry.QName, err)
			}
			c.attributes.storeDefault(entry.ID, value)
		}
		if decl.HasFixed {
			value, err := c.compileDefaultFixedValue(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("attribute %s fixed: %w", entry.QName, err)
			}
			c.attributes.storeFixed(entry.ID, value)
		}
	}
	return nil
}

func (c *compiler) compileAttributeUseDefaultFixed(decl *model.AttributeDecl) error {
	if st, ok := model.AsSimpleType(decl.Type); ok && model.IsPlaceholderSimpleType(st) {
		return fmt.Errorf("attribute use %s type not resolved", decl.Name)
	}
	if !decl.HasDefault && !decl.HasFixed {
		return nil
	}
	typ, err := c.valueTypeForAttribute(decl)
	if err != nil {
		return fmt.Errorf("attribute use %s: %w", decl.Name, err)
	}
	if decl.HasDefault {
		if !c.attrUses.defaults.contains(decl) {
			value, err := c.compileDefaultFixedValue(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("attribute use %s default: %w", decl.Name, err)
			}
			c.attrUses.storeDefault(decl, value)
		}
	}
	if decl.HasFixed {
		if !c.attrUses.fixed.contains(decl) {
			value, err := c.compileDefaultFixedValue(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("attribute use %s fixed: %w", decl.Name, err)
			}
			c.attrUses.storeFixed(decl, value)
		}
	}
	return nil
}
