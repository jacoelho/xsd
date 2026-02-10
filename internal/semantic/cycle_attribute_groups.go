package semantic

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func detectAttributeGroupCycles(schema *parser.Schema) error {
	states := make(map[model.QName]visitState)

	var visit func(name model.QName, group *model.AttributeGroup) error
	visit = func(name model.QName, group *model.AttributeGroup) error {
		switch states[name] {
		case stateVisiting:
			return fmt.Errorf("attributeGroup cycle detected at %s", name)
		case stateDone:
			return nil
		}
		states[name] = stateVisiting
		for _, ref := range group.AttrGroups {
			target := schema.AttributeGroups[ref]
			if target == nil {
				return fmt.Errorf("attributeGroup %s ref %s not found", name, ref)
			}
			if err := visit(ref, target); err != nil {
				return err
			}
		}
		states[name] = stateDone
		return nil
	}

	for _, decl := range schema.GlobalDecls {
		if decl.Kind != parser.GlobalDeclAttributeGroup {
			continue
		}
		group := schema.AttributeGroups[decl.Name]
		if group == nil {
			return fmt.Errorf("missing attributeGroup %s", decl.Name)
		}
		if err := visit(decl.Name, group); err != nil {
			return err
		}
	}
	return nil
}
