package runtimecompile

import (
	"fmt"

	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileAttributeDefaults(registry *schema.Registry) error {
	for _, entry := range registry.AttributeOrder {
		decl := entry.Decl
		if decl == nil {
			continue
		}
		if st, ok := types.AsSimpleType(decl.Type); ok && types.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("attribute %s type not resolved", entry.QName)
		}
		var typ types.Type
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

func (c *compiler) compileAttributeUseDefaultFixed(decl *types.AttributeDecl) error {
	if st, ok := types.AsSimpleType(decl.Type); ok && types.IsPlaceholderSimpleType(st) {
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
		if !c.attrUses.containsDefault(decl) {
			value, err := c.compileDefaultFixedValue(decl.Default, typ, decl.DefaultContext)
			if err != nil {
				return fmt.Errorf("attribute use %s default: %w", decl.Name, err)
			}
			c.attrUses.storeDefault(decl, value)
		}
	}
	if decl.HasFixed {
		if !c.attrUses.containsFixed(decl) {
			value, err := c.compileDefaultFixedValue(decl.Fixed, typ, decl.FixedContext)
			if err != nil {
				return fmt.Errorf("attribute use %s fixed: %w", decl.Name, err)
			}
			c.attrUses.storeFixed(decl, value)
		}
	}
	return nil
}
