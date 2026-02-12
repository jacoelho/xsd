package analysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/ids"
	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
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
		Element: func(name model.QName, decl *model.ElementDecl) error {
			if decl == nil {
				return fmt.Errorf("missing global element %s", name)
			}
			return b.visitGlobalElement(decl)
		},
		Type: func(name model.QName, typ model.Type) error {
			if typ == nil {
				return fmt.Errorf("missing global type %s", name)
			}
			return b.visitGlobalType(name, typ)
		},
		Attribute: func(name model.QName, attr *model.AttributeDecl) error {
			if attr == nil {
				return fmt.Errorf("missing global attribute %s", name)
			}
			return b.visitGlobalAttribute(name, attr)
		},
		AttributeGroup: func(name model.QName, group *model.AttributeGroup) error {
			if group == nil {
				return fmt.Errorf("missing attributeGroup %s", name)
			}
			return b.visitAttributeGroup(group)
		},
		Group: func(name model.QName, group *model.ModelGroup) error {
			if group == nil {
				return fmt.Errorf("missing group %s", name)
			}
			return b.visitGroup(group)
		},
		Notation: func(model.QName, *model.NotationDecl) error {
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
	typeIDs  map[model.Type]ids.TypeID
	nextType ids.TypeID
	nextElem ids.ElemID
	nextAttr ids.AttrID
}

func newBuilder(schema *parser.Schema) *builder {
	return &builder{
		schema:   schema,
		registry: newRegistry(),
		typeIDs:  make(map[model.Type]ids.TypeID),
		nextType: 1,
		nextElem: 1,
		nextAttr: 1,
	}
}
