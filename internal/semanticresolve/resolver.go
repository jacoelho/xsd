package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// Resolver resolves all QName references in a schema.
// Runs exactly once after parsing. Detects cycles during resolution.
type Resolver struct {
	schema *parser.Schema

	// Cycle detection during resolution (cleared after resolution)
	detector *CycleDetector[types.QName]

	// Pointer-based tracking for anonymous types (which have empty QNames) to
	// avoid false cycle matches while still detecting self-references.
	resolvingPtrs map[types.Type]bool
	resolvedPtrs  map[types.Type]bool
}

// NewResolver creates a new resolver for the given schema.
func NewResolver(sch *parser.Schema) *Resolver {
	return &Resolver{
		schema:        sch,
		detector:      NewCycleDetector[types.QName](),
		resolvingPtrs: make(map[types.Type]bool),
		resolvedPtrs:  make(map[types.Type]bool),
	}
}

// Resolve resolves all references in the schema.
// Returns an error if there are unresolvable references or invalid cycles.
func (r *Resolver) Resolve() error {
	// order matters: resolve in dependency order

	// 1. Simple types (only depend on built-ins or other simple types)
	for _, qname := range sortedQNames(r.schema.TypeDefs) {
		typ := r.schema.TypeDefs[qname]
		if st, ok := typ.(*types.SimpleType); ok {
			if err := r.resolveSimpleType(qname, st); err != nil {
				return err
			}
		}
	}

	// 2. Complex types (may depend on simple types)
	for _, qname := range sortedQNames(r.schema.TypeDefs) {
		typ := r.schema.TypeDefs[qname]
		if ct, ok := typ.(*types.ComplexType); ok {
			if err := r.resolveComplexType(qname, ct); err != nil {
				return err
			}
		}
	}

	// 3. Groups (reference types and other groups)
	for _, qname := range sortedQNames(r.schema.Groups) {
		grp := r.schema.Groups[qname]
		if err := r.resolveGroup(qname, grp); err != nil {
			return err
		}
	}

	// 4. Elements (reference types and groups)
	for _, qname := range sortedQNames(r.schema.ElementDecls) {
		elem := r.schema.ElementDecls[qname]
		if err := r.resolveElement(qname, elem); err != nil {
			return err
		}
	}

	// 5. Attributes
	for _, qname := range sortedQNames(r.schema.AttributeDecls) {
		attr := r.schema.AttributeDecls[qname]
		if err := r.resolveAttribute(attr); err != nil {
			return err
		}
	}

	// 6. Attribute groups
	for _, qname := range sortedQNames(r.schema.AttributeGroups) {
		ag := r.schema.AttributeGroups[qname]
		if err := r.resolveAttributeGroup(qname, ag); err != nil {
			return err
		}
	}

	return nil
}
