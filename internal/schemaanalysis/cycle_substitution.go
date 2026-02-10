package schemaanalysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func detectSubstitutionGroupCycles(schema *parser.Schema) error {
	states := make(map[model.QName]visitState)

	var visit func(name model.QName) error
	visit = func(name model.QName) error {
		switch states[name] {
		case stateVisiting:
			return fmt.Errorf("substitution group cycle detected at %s", name)
		case stateDone:
			return nil
		}
		states[name] = stateVisiting
		decl := schema.ElementDecls[name]
		if decl == nil {
			return fmt.Errorf("element %s not found", name)
		}
		if !decl.SubstitutionGroup.IsZero() {
			head := decl.SubstitutionGroup
			if _, ok := schema.ElementDecls[head]; !ok {
				return fmt.Errorf("element %s substitutionGroup %s not found", name, head)
			}
			if err := visit(head); err != nil {
				return err
			}
		}
		states[name] = stateDone
		return nil
	}

	for _, decl := range schema.GlobalDecls {
		if decl.Kind != parser.GlobalDeclElement {
			continue
		}
		if err := visit(decl.Name); err != nil {
			return err
		}
	}
	return nil
}
