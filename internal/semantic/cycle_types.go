package semantic

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func detectTypeCycles(schema *parser.Schema) error {
	states := make(map[types.QName]visitState)

	var visit func(name types.QName, typ types.Type) error
	visit = func(name types.QName, typ types.Type) error {
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
		if !base.IsZero() && base.Namespace != types.XSDNamespace {
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

func typeBaseQName(typ types.Type) types.QName {
	switch typed := typ.(type) {
	case *types.SimpleType:
		if typed.Restriction == nil {
			return types.QName{}
		}
		return typed.Restriction.Base
	case *types.ComplexType:
		if typed.Content() == nil {
			return types.QName{}
		}
		return typed.Content().BaseTypeQName()
	default:
		return types.QName{}
	}
}
