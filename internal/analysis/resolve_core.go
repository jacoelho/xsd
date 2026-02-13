package analysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolveguard"
	"github.com/jacoelho/xsd/internal/types"
)

// ResolvedReferences records resolved references without mutating the parsed schema.
type ResolvedReferences struct {
	ElementRefs   map[types.QName]ids.ElemID
	AttributeRefs map[types.QName]ids.AttrID
	GroupRefs     map[types.QName]types.QName
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
	elementState     *resolveguard.Pointer[*types.ElementDecl]
	modelGroupState  *resolveguard.Pointer[*types.ModelGroup]
	simpleTypeState  *resolveguard.Pointer[*types.SimpleType]
	complexTypeState *resolveguard.Pointer[*types.ComplexType]
}

func newReferenceResolver(schema *parser.Schema, registry *Registry) *referenceResolver {
	return &referenceResolver{
		schema:   schema,
		registry: registry,
		refs: &ResolvedReferences{
			ElementRefs:   make(map[types.QName]ids.ElemID),
			AttributeRefs: make(map[types.QName]ids.AttrID),
			GroupRefs:     make(map[types.QName]types.QName),
		},
		elementState:     resolveguard.NewPointer[*types.ElementDecl](),
		modelGroupState:  resolveguard.NewPointer[*types.ModelGroup](),
		simpleTypeState:  resolveguard.NewPointer[*types.SimpleType](),
		complexTypeState: resolveguard.NewPointer[*types.ComplexType](),
	}
}

func (r *referenceResolver) resolveGlobalDeclarations() error {
	return globaldecl.ForEach(r.schema, globaldecl.Handlers{
		Element: func(name types.QName, decl *types.ElementDecl) error {
			if decl == nil {
				return fmt.Errorf("missing global element %s", name)
			}
			return r.resolveGlobalElement(decl)
		},
		Type: func(name types.QName, typ types.Type) error {
			if typ == nil {
				return fmt.Errorf("missing global type %s", name)
			}
			if err := r.resolveType(typ); err != nil {
				return fmt.Errorf("type %s: %w", name, err)
			}
			return nil
		},
		Attribute: func(name types.QName, attr *types.AttributeDecl) error {
			if attr == nil {
				return fmt.Errorf("missing global attribute %s", name)
			}
			if err := r.resolveAttribute(attr); err != nil {
				return fmt.Errorf("attribute %s: %w", name, err)
			}
			return nil
		},
		AttributeGroup: func(name types.QName, group *types.AttributeGroup) error {
			if group == nil {
				return fmt.Errorf("missing attributeGroup %s", name)
			}
			return r.resolveAttributeGroup(name, group)
		},
		Group: func(name types.QName, group *types.ModelGroup) error {
			if group == nil {
				return fmt.Errorf("missing group %s", name)
			}
			return r.resolveModelGroup(group)
		},
		Notation: func(types.QName, *types.NotationDecl) error {
			return nil
		},
	})
}
