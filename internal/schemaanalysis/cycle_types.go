package schemaanalysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func detectTypeCycles(schema *parser.Schema) error {
	states := make(map[model.QName]visitState)

	var visit func(name model.QName, typ model.Type) error
	visit = func(name model.QName, typ model.Type) error {
		if name.IsZero() {
			return nil
		}
		switch states[name] {
		case stateVisiting:
			return fmt.Errorf("type cycle detected at %s", name)
		case stateDone:
			return nil
		}
		states[name] = stateVisiting
		baseType, _, err := baseTypeFor(schema, typ)
		if err != nil {
			return fmt.Errorf("type %s: %w", name, err)
		}
		if baseType != nil {
			baseName := baseType.Name()
			if !baseName.IsZero() && baseName.Namespace != model.XSDNamespace {
				if err := visit(baseName, baseType); err != nil {
					return err
				}
			}
		}
		states[name] = stateDone
		return nil
	}

	for _, decl := range schema.GlobalDecls {
		if decl.Kind != parser.GlobalDeclType {
			continue
		}
		typ := schema.TypeDefs[decl.Name]
		if typ == nil {
			return fmt.Errorf("missing global type %s", decl.Name)
		}
		if err := visit(decl.Name, typ); err != nil {
			return err
		}
	}
	return nil
}
