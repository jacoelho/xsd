package semantics

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// Resolver resolves all QName references in a schema.
// Runs exactly once after parsing. Detects cycles during resolution.
type Resolver struct {
	schema *parser.Schema

	// Cycle detection during resolution (cleared after resolution)
	detector *CycleDetector[model.QName]

	// Pointer-based tracking for anonymous types (which have empty QNames) to
	// avoid false cycle matches while still detecting self-references.
	anonymousTypeGuard *analysis.Pointer[model.Type]
}

// NewResolver creates a new resolver for the given schema.
func NewResolver(sch *parser.Schema) *Resolver {
	return &Resolver{
		schema:             sch,
		detector:           NewCycleDetector[model.QName](),
		anonymousTypeGuard: analysis.NewPointer[model.Type](),
	}
}

// ResolveAndValidateSchema runs schema semantic preparation checks in compile order.
// It returns validation errors separately from hard preparation failures.
func ResolveAndValidateSchema(sch *parser.Schema) ([]error, error) {
	if sch == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if err := ResolveGroupReferences(sch); err != nil {
		return nil, fmt.Errorf("resolve group references: %w", err)
	}
	if structureErrs := ValidateStructure(sch); len(structureErrs) > 0 {
		return structureErrs, nil
	}
	if err := NewResolver(sch).Resolve(); err != nil {
		return nil, fmt.Errorf("resolve type references: %w", err)
	}
	if refErrs := ValidateReferences(sch); len(refErrs) > 0 {
		return refErrs, nil
	}
	if deferredRangeErrs := ValidateDeferredRangeFacetValues(sch); len(deferredRangeErrs) > 0 {
		return deferredRangeErrs, nil
	}
	if parser.HasPlaceholders(sch) {
		return nil, fmt.Errorf("schema has unresolved placeholders")
	}
	return nil, nil
}

// Resolve resolves all references in the schema.
// Returns an error if there are unresolvable references or invalid cycles.
func (r *Resolver) Resolve() error {
	if r == nil || r.schema == nil {
		return fmt.Errorf("nil schema")
	}
	index := buildIterationIndex(r.schema)
	if err := r.resolveSimpleTypesPhase(index); err != nil {
		return err
	}
	if err := r.resolveComplexTypesPhase(index); err != nil {
		return err
	}
	if err := r.resolveGroupsPhase(index); err != nil {
		return err
	}
	if err := r.resolveElementsPhase(index); err != nil {
		return err
	}
	if err := r.resolveAttributesPhase(index); err != nil {
		return err
	}
	return r.resolveAttributeGroupsPhase(index)
}

func (r *Resolver) resolveSimpleTypesPhase(index *iterationIndex) error {
	for _, qname := range index.typeQNames {
		typ := r.schema.TypeDefs[qname]
		if st, ok := typ.(*model.SimpleType); ok {
			if err := r.resolveSimpleType(qname, st); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Resolver) resolveComplexTypesPhase(index *iterationIndex) error {
	for _, qname := range index.typeQNames {
		typ := r.schema.TypeDefs[qname]
		if ct, ok := typ.(*model.ComplexType); ok {
			if err := r.resolveComplexType(qname, ct); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Resolver) resolveGroupsPhase(index *iterationIndex) error {
	for _, qname := range index.groupQNames {
		grp := r.schema.Groups[qname]
		if err := analysis.ResolveNamed[model.QName](r.detector, qname, func() error {
			return r.resolveParticles(grp.Particles)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveElementsPhase(index *iterationIndex) error {
	for _, qname := range index.elementQNames {
		elem := r.schema.ElementDecls[qname]
		if elem.Type == nil {
			continue
		}
		if err := r.resolveElementType(elem, qname, elementTypeOptions{
			simpleContext:  "element %s type: %w",
			complexContext: "element %s type: %w",
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveAttributesPhase(index *iterationIndex) error {
	for _, qname := range index.attributeQNames {
		attr := r.schema.AttributeDecls[qname]
		if err := r.resolveAttributeType(attr); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) resolveAttributeGroupsPhase(index *iterationIndex) error {
	for _, qname := range index.attributeGroupQNames {
		ag := r.schema.AttributeGroups[qname]
		if err := r.resolveAttributeGroup(qname, ag); err != nil {
			return err
		}
	}
	return nil
}

func (r *Resolver) lookupType(qname, referrer model.QName) (model.Type, error) {
	if bt := model.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		return bt, nil
	}

	if qname == referrer {
		return nil, fmt.Errorf("circular type definition: %s references itself", qname)
	}

	typ, ok := r.schema.TypeDefs[qname]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTypeNotFound, qname)
	}

	if r.detector.IsResolving(qname) {
		if referrer.IsZero() {
			return typ, nil
		}
		return nil, fmt.Errorf("circular reference detected: %s", qname.String())
	}

	if err := r.resolveLookupType(qname, typ); err != nil {
		return nil, err
	}

	return typ, nil
}

func (r *Resolver) resolveLookupType(qname model.QName, typ model.Type) error {
	switch t := typ.(type) {
	case *model.SimpleType:
		return r.resolveSimpleType(qname, t)
	case *model.ComplexType:
		return r.resolveComplexType(qname, t)
	}
	return nil
}

type elementTypeOptions struct {
	simpleContext  string
	complexContext string
	allowResolving bool
}

func (r *Resolver) resolveElementType(elem *model.ElementDecl, elemName model.QName, opts elementTypeOptions) error {
	switch t := elem.Type.(type) {
	case *model.SimpleType:
		return r.resolveSimpleElementType(elem, elemName, t, opts)
	case *model.ComplexType:
		return r.resolveComplexElementType(elemName, t, opts)
	}
	return nil
}

func (r *Resolver) resolveSimpleElementType(elem *model.ElementDecl, elemName model.QName, st *model.SimpleType, opts elementTypeOptions) error {
	if model.IsPlaceholderSimpleType(st) {
		actualType, err := r.lookupType(st.QName, model.QName{})
		if err != nil {
			return fmt.Errorf(opts.simpleContext, elemName, err)
		}
		elem.Type = actualType
		return nil
	}
	if err := r.resolveSimpleType(st.QName, st); err != nil {
		return fmt.Errorf(opts.simpleContext, elemName, err)
	}
	return nil
}

func (r *Resolver) resolveComplexElementType(elemName model.QName, ct *model.ComplexType, opts elementTypeOptions) error {
	if opts.allowResolving && !ct.QName.IsZero() && r.detector.IsResolving(ct.QName) {
		return nil
	}
	if err := r.resolveComplexType(ct.QName, ct); err != nil {
		return fmt.Errorf(opts.complexContext, elemName, err)
	}
	return nil
}

func (r *Resolver) resolveParticles(particles []model.Particle) error {
	queue := slices.Clone(particles)

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]

		switch particle := p.(type) {
		case *model.GroupRef:
			if err := r.resolveGroupRefParticle(particle); err != nil {
				return err
			}
		case *model.ModelGroup:
			queue = append(queue, particle.Particles...)
		case *model.ElementDecl:
			if err := r.resolveElementDeclParticle(particle); err != nil {
				return err
			}
		case *model.AnyElement:
		}
	}
	return nil
}

func (r *Resolver) resolveGroupRefParticle(ref *model.GroupRef) error {
	group, ok := r.schema.Groups[ref.RefQName]
	if !ok {
		return fmt.Errorf("group %s not found", ref.RefQName)
	}
	return analysis.ResolveNamed[model.QName](r.detector, ref.RefQName, func() error {
		return r.resolveParticles(group.Particles)
	})
}

func (r *Resolver) resolveElementDeclParticle(elem *model.ElementDecl) error {
	if elem.IsReference || elem.Type == nil {
		return nil
	}
	return r.resolveElementType(elem, elem.Name, elementTypeOptions{
		simpleContext:  "element %s type: %w",
		complexContext: "element %s anonymous type: %w",
		allowResolving: true,
	})
}

func (r *Resolver) resolveContentParticles(content model.Content) error {
	return model.WalkContentParticles(content, func(particle model.Particle) error {
		return r.resolveParticles([]model.Particle{particle})
	})
}
