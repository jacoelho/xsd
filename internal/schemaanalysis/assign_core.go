package schemaanalysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// AssignIDs walks the parsed schema in deterministic order and assigns IDs.
func AssignIDs(schema *parser.Schema) (*Registry, error) {
	if err := RequireResolved(schema); err != nil {
		return nil, err
	}
	if err := validateSchemaInput(schema); err != nil {
		return nil, err
	}

	b := newBuilder(schema)
	for _, decl := range schema.GlobalDecls {
		if err := b.visitGlobalDeclaration(decl); err != nil {
			return nil, err
		}
	}

	return b.registry, nil
}

func hasGlobalDecls(schema *parser.Schema) bool {
	return len(schema.ElementDecls) > 0 || len(schema.TypeDefs) > 0 ||
		len(schema.AttributeDecls) > 0 || len(schema.AttributeGroups) > 0 ||
		len(schema.Groups) > 0 || len(schema.NotationDecls) > 0
}

type builder struct {
	schema   *parser.Schema
	registry *Registry
	typeIDs  map[model.Type]TypeID
	nextType TypeID
	nextElem ElemID
	nextAttr AttrID
}

func newBuilder(schema *parser.Schema) *builder {
	return &builder{
		schema:   schema,
		registry: newRegistry(),
		typeIDs:  make(map[model.Type]TypeID),
		nextType: 1,
		nextElem: 1,
		nextAttr: 1,
	}
}

func (b *builder) visitGlobalDeclaration(decl parser.GlobalDecl) error {
	switch decl.Kind {
	case parser.GlobalDeclElement:
		declared := b.schema.ElementDecls[decl.Name]
		if declared == nil {
			return fmt.Errorf("missing global element %s", decl.Name)
		}
		return b.visitGlobalElement(declared)
	case parser.GlobalDeclType:
		typeDef := b.schema.TypeDefs[decl.Name]
		if typeDef == nil {
			return fmt.Errorf("missing global type %s", decl.Name)
		}
		return b.visitGlobalType(decl.Name, typeDef)
	case parser.GlobalDeclAttribute:
		attr := b.schema.AttributeDecls[decl.Name]
		if attr == nil {
			return fmt.Errorf("missing global attribute %s", decl.Name)
		}
		return b.visitGlobalAttribute(decl.Name, attr)
	case parser.GlobalDeclAttributeGroup:
		group := b.schema.AttributeGroups[decl.Name]
		if group == nil {
			return fmt.Errorf("missing attributeGroup %s", decl.Name)
		}
		return b.visitAttributeGroup(group)
	case parser.GlobalDeclGroup:
		group := b.schema.Groups[decl.Name]
		if group == nil {
			return fmt.Errorf("missing group %s", decl.Name)
		}
		return b.visitGroup(group)
	case parser.GlobalDeclNotation:
		return nil
	default:
		return fmt.Errorf("unknown global declaration kind %d", decl.Kind)
	}
}
