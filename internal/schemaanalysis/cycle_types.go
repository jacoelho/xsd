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
		base := typeBaseQName(typ)
		if !base.IsZero() && base.Namespace != model.XSDNamespace {
			baseType := schema.TypeDefs[base]
			if baseType == nil {
				return fmt.Errorf("type %s base %s not found", name, base)
			}
			if err := visit(base, baseType); err != nil {
				return err
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

func typeBaseQName(typ model.Type) model.QName {
	switch typed := typ.(type) {
	case *model.SimpleType:
		if typed.Restriction == nil {
			return model.QName{}
		}
		return typed.Restriction.Base
	case *model.ComplexType:
		if typed.Content() == nil {
			return model.QName{}
		}
		return typed.Content().BaseTypeQName()
	default:
		return model.QName{}
	}
}
