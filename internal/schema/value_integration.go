package schema

import (
	"errors"
	"strconv"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

// newSchemaValueBuilder creates the one mutable value owner used while a
// schema is being compiled. The schema graph limits are the admission bounds
// for value types and construction storage; the value package still owns the
// detailed type and facet checks.
func newSchemaValueBuilder(limits Limits) *value.Builder {
	// Schema component admission bounds the number of value records. The
	// intrinsic types, including the anonymous XSI list, do not consume that
	// source-node allowance.
	maxTypes := uint64(builtinSchemaSimpleTypeCount)
	maxTypes = addBounded(maxTypes, nonNegativeLimit(limits.MaxSchemaInstantiatedNodes))
	maxTypes = min(maxTypes, uint64(^uint32(0)))

	// A value dependency chain is bounded by the compiler's component-depth
	// contract, independently of XML nesting depth. One value can revisit a
	// bounded type graph through list items and union members, so evaluation
	// work is the bounded token payload multiplied by the admitted type graph.
	maxDepth := maxComponentDependencyDepth
	maxStorage := valueStorageBound(limits)
	maxWork := valueEvalWorkBound(limits)
	return value.NewBuilder(value.BuilderOptions{
		MaxDepth:            uint16(maxDepth),
		MaxTypes:            uint32(maxTypes),
		MaxStorageBytes:     maxStorage,
		MaxConstructionWork: maxWork,
		MaxPatternWork:      limits.MaxSchemaPatternWork,
		MaxTotalPatternWork: limits.MaxSchemaTotalPatternWork,
	})
}

func valueEvalWorkBound(limits Limits) uint64 {
	input := uint64(max(0, limits.MaxSchemaTokenBytes)) + 1
	types := addBounded(uint64(builtinSchemaSimpleTypeCount), nonNegativeLimit(limits.MaxSchemaInstantiatedNodes))
	return multiplyBounded(input, types)
}

func addBounded(a, b uint64) uint64 {
	if a > ^uint64(0)-b {
		return ^uint64(0)
	}
	return a + b
}

func valueStorageBound(limits Limits) uint64 {
	// typeSpecStorage charges a fixed record estimate plus copied lexical and
	// pattern bytes. Source bytes cover the latter; a fixed per admitted node
	// term covers the record and slice metadata. Checked arithmetic keeps the
	// bound itself from becoming an accidental overflow.
	const perNode = uint64(1024)
	nodes := nonNegativeLimit(limits.MaxSchemaInstantiatedNodes)
	sourceBytes := uint64(max(0, limits.MaxSchemaTotalBytes))
	bound := addBounded(multiplyBounded(sourceBytes, 2), multiplyBounded(nodes, perNode))
	if bound < uint64(builtinSchemaSimpleTypeCount)+1 {
		return uint64(builtinSchemaSimpleTypeCount) + 1
	}
	return bound
}

func nonNegativeLimit(limit int) uint64 {
	if limit < 0 {
		return 0
	}
	// NormalizeOptions rejects negative limits before this helper is reached.
	return uint64(limit)
}

func multiplyBounded(a, b uint64) uint64 {
	if a != 0 && b > ^uint64(0)/a {
		return ^uint64(0)
	}
	return a * b
}

func (c *compiler) ensureValueBuilder() *value.Builder {
	if c.rt.valueBuilder == nil {
		c.rt.valueBuilder = newSchemaValueBuilder(c.limits)
	}
	return c.rt.valueBuilder
}

// schemaValueError is the single translation of value-owned evaluation
// failures into compile diagnostics. Precedence follows the validate-side
// model: metadata, then limits (aggregate pattern work, per-literal pattern
// work, any other limit), then unsupported. Unclassified failures keep
// their reason under code, so no raw error escapes compilation.
func schemaValueError(limits Limits, code xsderrors.Code, subject string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, value.ErrMetadata):
		return xsderrors.InternalInvariant(subject + ": " + err.Error())
	case errors.Is(err, value.ErrPatternTotalLimit):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, subject+": MaxSchemaTotalPatternWork="+strconv.FormatUint(limits.MaxSchemaTotalPatternWork, 10)+" exceeded")
	case errors.Is(err, value.ErrPatternLimit):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, subject+": MaxSchemaPatternWork="+strconv.FormatUint(limits.MaxSchemaPatternWork, 10)+" exceeded")
	case errors.Is(err, value.ErrLimit):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, subject+": "+err.Error())
	case xsderrors.IsUnsupported(err):
		return err
	}
	if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
		return err
	}
	return xsderrors.SchemaCompile(code, subject+": "+err.Error())
}

func (c *compiler) completeValueType(id SimpleTypeID, st simpleType) error {
	if id < value.BuiltinTypeCount {
		return nil
	}
	builder := c.ensureValueBuilder()
	spec := st.ValueSpec
	if st.Missing {
		// A missing schema component remains unavailable to callers, but its
		// reserved ID still needs a complete value record so later IDs stay
		// canonical and the builder can seal atomically.
		spec = value.TypeSpec{
			Variety:           value.Atomic,
			Primitive:         value.PrimitiveString,
			Whitespace:        value.WhitespaceCollapse,
			WhitespacePresent: true,
			Base:              value.BuiltinType(value.PrimitiveString),
			ListItem:          value.NoType,
		}
	}
	if err := builder.Complete(id, spec); err != nil {
		return schemaValueError(c.limits, xsderrors.CodeSchemaFacet, "simple type", err)
	}
	constrained, ok := builder.UnionHasFacets(id)
	if !ok {
		return xsderrors.InternalInvariant("completed simple type has no value definition")
	}
	c.rt.SimpleTypes[id].unionHasFacets = constrained
	return nil
}

func (c *compiler) validateValue(id SimpleTypeID, lexical string, resolve value.QNameResolver, needs value.Needs) (value.Value, []resolvedValueName, error) {
	recorder := valueConstraintResolver{resolve: resolve}
	resolver := value.Resolver{Notation: func(namespace, local string) bool {
		q, ok := c.rt.lookupQName(namespace, local)
		return ok && c.rt.notationDeclared(q)
	}}
	if resolve != nil {
		resolver.QName = recorder.resolveQName
	}
	validated, err := c.ensureValueBuilder().Validate(id, lexical, resolver, needs, nil)
	return validated, recorder.names, err
}

// ValueProgram returns the immutable value program used by validation.
func (rt *Schema) ValueProgram() *value.Program {
	if rt == nil {
		return nil
	}
	return rt.program.Value
}

// NotationDeclared reports whether one expanded name is a schema notation.
func (rt *Schema) NotationDeclared(namespace, local string) bool {
	if rt == nil {
		return false
	}
	return rt.program.Notations[ExpandedName{Namespace: namespace, Local: local}]
}
