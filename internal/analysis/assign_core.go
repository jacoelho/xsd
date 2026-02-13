package analysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
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
	if err := globaldecl.ForEach(schema, globaldecl.Handlers{
		Element: func(name types.QName, decl *types.ElementDecl) error {
			if decl == nil {
				return fmt.Errorf("missing global element %s", name)
			}
			return b.visitGlobalElement(decl)
		},
		Type: func(name types.QName, typ types.Type) error {
			if typ == nil {
				return fmt.Errorf("missing global type %s", name)
			}
			return b.visitGlobalType(name, typ)
		},
		Attribute: func(name types.QName, attr *types.AttributeDecl) error {
			if attr == nil {
				return fmt.Errorf("missing global attribute %s", name)
			}
			return b.visitGlobalAttribute(name, attr)
		},
		AttributeGroup: func(name types.QName, group *types.AttributeGroup) error {
			if group == nil {
				return fmt.Errorf("missing attributeGroup %s", name)
			}
			return b.visitAttributeGroup(group)
		},
		Group: func(name types.QName, group *types.ModelGroup) error {
			if group == nil {
				return fmt.Errorf("missing group %s", name)
			}
			return b.visitGroup(group)
		},
		Notation: func(types.QName, *types.NotationDecl) error {
			return nil
		},
	}); err != nil {
		return nil, err
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
	typeIDs  map[types.Type]ids.TypeID
	nextType ids.TypeID
	nextElem ids.ElemID
	nextAttr ids.AttrID
}

func newBuilder(schema *parser.Schema) *builder {
	return &builder{
		schema:   schema,
		registry: newRegistry(),
		typeIDs:  make(map[types.Type]ids.TypeID),
		nextType: 1,
		nextElem: 1,
		nextAttr: 1,
	}
}
