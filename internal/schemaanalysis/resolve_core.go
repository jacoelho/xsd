package schemaanalysis

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// ResolvedReferences records resolved references without mutating the parsed schema.
type ResolvedReferences struct {
	ElementRefs   map[model.QName]ElemID
	AttributeRefs map[model.QName]AttrID
	GroupRefs     map[model.QName]model.QName
}

type resolveState uint8

const (
	resolveUnseen resolveState = iota
	resolveResolving
	resolveResolved
)

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
	elementState     map[*model.ElementDecl]resolveState
	modelGroupState  map[*model.ModelGroup]resolveState
	simpleTypeState  map[*model.SimpleType]resolveState
	complexTypeState map[*model.ComplexType]resolveState
}

func newReferenceResolver(schema *parser.Schema, registry *Registry) *referenceResolver {
	return &referenceResolver{
		schema:   schema,
		registry: registry,
		refs: &ResolvedReferences{
			ElementRefs:   make(map[model.QName]ElemID),
			AttributeRefs: make(map[model.QName]AttrID),
			GroupRefs:     make(map[model.QName]model.QName),
		},
		elementState:     make(map[*model.ElementDecl]resolveState),
		modelGroupState:  make(map[*model.ModelGroup]resolveState),
		simpleTypeState:  make(map[*model.SimpleType]resolveState),
		complexTypeState: make(map[*model.ComplexType]resolveState),
	}
}

func (r *referenceResolver) resolveGlobalDeclarations() error {
	for _, decl := range r.schema.GlobalDecls {
		if err := r.resolveGlobalDeclaration(decl); err != nil {
			return err
		}
	}
	return nil
}

func (r *referenceResolver) resolveGlobalDeclaration(decl parser.GlobalDecl) error {
	switch decl.Kind {
	case parser.GlobalDeclElement:
		declared := r.schema.ElementDecls[decl.Name]
		if declared == nil {
			return fmt.Errorf("missing global element %s", decl.Name)
		}
		return r.resolveGlobalElement(declared)
	case parser.GlobalDeclType:
		typeDef := r.schema.TypeDefs[decl.Name]
		if typeDef == nil {
			return fmt.Errorf("missing global type %s", decl.Name)
		}
		if err := r.resolveType(typeDef); err != nil {
			return fmt.Errorf("type %s: %w", decl.Name, err)
		}
		return nil
	case parser.GlobalDeclAttribute:
		attr := r.schema.AttributeDecls[decl.Name]
		if attr == nil {
			return fmt.Errorf("missing global attribute %s", decl.Name)
		}
		if err := r.resolveAttribute(attr); err != nil {
			return fmt.Errorf("attribute %s: %w", decl.Name, err)
		}
		return nil
	case parser.GlobalDeclAttributeGroup:
		group := r.schema.AttributeGroups[decl.Name]
		if group == nil {
			return fmt.Errorf("missing attributeGroup %s", decl.Name)
		}
		return r.resolveAttributeGroup(decl.Name, group)
	case parser.GlobalDeclGroup:
		group := r.schema.Groups[decl.Name]
		if group == nil {
			return fmt.Errorf("missing group %s", decl.Name)
		}
		return r.resolveModelGroup(group)
	case parser.GlobalDeclNotation:
		return nil
	default:
		return fmt.Errorf("unknown global declaration kind %d", decl.Kind)
	}
}
