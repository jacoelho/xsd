package validatorcompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	model "github.com/jacoelho/xsd/internal/model"
	schema "github.com/jacoelho/xsd/internal/semantic"
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
		if name == builtins.TypeNameAnyType {
			continue
		}
		bt := builtins.Get(name)
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
		st, ok := model.AsSimpleType(entry.Type)
		if !ok {
			continue
		}
		if model.IsPlaceholderSimpleType(st) {
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
		ct, ok := model.AsComplexType(entry.Type)
		if !ok {
			continue
		}
		if _, ok := ct.Content().(*model.SimpleContent); !ok {
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
