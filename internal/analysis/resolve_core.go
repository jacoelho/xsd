package analysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/ids"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolveguard"
	model "github.com/jacoelho/xsd/internal/types"
)

// ResolvedReferences records resolved references without mutating the parsed schema.
type ResolvedReferences struct {
	ElementRefs   map[model.QName]ids.ElemID
	AttributeRefs map[model.QName]ids.AttrID
	GroupRefs     map[model.QName]model.QName
}

// ResolveReferences validates and resolves QName references in the parsed schema.
func ResolveReferences(schema *parser.Schema, registry *Registry) (*ResolvedReferences, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if err := RequireResolved(schema); err != nil {
		return nil, err
	}
	if err := validateSchemaInput(schema); err != nil {
		return nil, err
	}

	resolver := newReferenceResolver(schema, registry)
	if err := resolver.resolveGlobalDeclarations(); err != nil {
		return nil, err
	}

	return resolver.refs, nil
}

type referenceResolver struct {
	schema           *parser.Schema
	registry         *Registry
	refs             *ResolvedReferences
	elementState     *resolveguard.Pointer[*model.ElementDecl]
	modelGroupState  *resolveguard.Pointer[*model.ModelGroup]
	simpleTypeState  *resolveguard.Pointer[*model.SimpleType]
	complexTypeState *resolveguard.Pointer[*model.ComplexType]
}

func newReferenceResolver(schema *parser.Schema, registry *Registry) *referenceResolver {
	return &referenceResolver{
		schema:   schema,
		registry: registry,
		refs: &ResolvedReferences{
			ElementRefs:   make(map[model.QName]ids.ElemID),
			AttributeRefs: make(map[model.QName]ids.AttrID),
			GroupRefs:     make(map[model.QName]model.QName),
		},
		elementState:     resolveguard.NewPointer[*model.ElementDecl](),
		modelGroupState:  resolveguard.NewPointer[*model.ModelGroup](),
		simpleTypeState:  resolveguard.NewPointer[*model.SimpleType](),
		complexTypeState: resolveguard.NewPointer[*model.ComplexType](),
	}
}

func (r *referenceResolver) resolveGlobalDeclarations() error {
	return globaldecl.ForEach(r.schema, globaldecl.Handlers{
		Element: func(name model.QName, decl *model.ElementDecl) error {
			if decl == nil {
				return fmt.Errorf("missing global element %s", name)
			}
			return r.resolveGlobalElement(decl)
		},
		Type: func(name model.QName, typ model.Type) error {
			if typ == nil {
				return fmt.Errorf("missing global type %s", name)
			}
			if err := r.resolveType(typ); err != nil {
				return fmt.Errorf("type %s: %w", name, err)
			}
			return nil
		},
		Attribute: func(name model.QName, attr *model.AttributeDecl) error {
			if attr == nil {
				return fmt.Errorf("missing global attribute %s", name)
			}
			if err := r.resolveAttribute(attr); err != nil {
				return fmt.Errorf("attribute %s: %w", name, err)
			}
			return nil
		},
		AttributeGroup: func(name model.QName, group *model.AttributeGroup) error {
			if group == nil {
				return fmt.Errorf("missing attributeGroup %s", name)
			}
			return r.resolveAttributeGroup(name, group)
		},
		Group: func(name model.QName, group *model.ModelGroup) error {
			if group == nil {
				return fmt.Errorf("missing group %s", name)
			}
			return r.resolveModelGroup(group)
		},
		Notation: func(model.QName, *model.NotationDecl) error {
			return nil
		},
	})
}
