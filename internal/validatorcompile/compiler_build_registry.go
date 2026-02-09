package validatorcompile

import (
	"fmt"

	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *compiler) compileRegistry(registry *schema.Registry) error {
	if err := c.compileBuiltinRegistry(); err != nil {
		return err
	}
	if err := c.compileSimpleTypeRegistry(registry); err != nil {
		return err
	}
	if err := c.compileSimpleContentRegistry(registry); err != nil {
		return err
	}
	return nil
}

func (c *compiler) compileBuiltinRegistry() error {
	for _, name := range builtinTypeNames() {
		if name == types.TypeNameAnyType {
			continue
		}
		bt := types.GetBuiltin(name)
		if bt == nil {
			continue
		}
		if _, err := c.compileType(bt); err != nil {
			return fmt.Errorf("builtin type %s: %w", name, err)
		}
	}
	return nil
}

func (c *compiler) compileSimpleTypeRegistry(registry *schema.Registry) error {
	for _, entry := range registry.TypeOrder {
		st, ok := types.AsSimpleType(entry.Type)
		if !ok {
			continue
		}
		if types.IsPlaceholderSimpleType(st) {
			return fmt.Errorf("type %s: unresolved placeholder", entry.QName)
		}
		_, err := c.compileType(st)
		if err != nil {
			return fmt.Errorf("type %s: %w", entry.QName, err)
		}
	}
	return nil
}

func (c *compiler) compileSimpleContentRegistry(registry *schema.Registry) error {
	for _, entry := range registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok {
			continue
		}
		if _, ok := ct.Content().(*types.SimpleContent); !ok {
			continue
		}
		textType, err := c.simpleContentTextType(ct)
		if err != nil {
			return fmt.Errorf("type %s: %w", entry.QName, err)
		}
		if textType == nil {
			return fmt.Errorf("type %s: simpleContent base missing", entry.QName)
		}
		if _, err := c.compileType(textType); err != nil {
			return fmt.Errorf("type %s: %w", entry.QName, err)
		}
	}
	return nil
}
