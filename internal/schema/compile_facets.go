package schema

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xsdregex"
	"github.com/jacoelho/xsd/xsderrors"
)

// compileFacetChildren admits source facet syntax and records one local facet
// step in ValueSpec. Effective inheritance, typed literal evaluation, bound
// ordering, and fixed-facet derivation are owned by value.Builder.Complete.
func (c *compiler) compileFacetChildren(parent *schemaNode, st *simpleType, base SimpleTypeID) error {
	state := compiledFacetState{unavailable: c.rt.simpleTypeUnavailable[base]}
	for child := range parent.xsdChildren() {
		if !isFacetLocal(child.local) {
			continue
		}
		if err := c.compileFacetChild(child, st, base, &state); err != nil {
			return err
		}
	}
	if !state.unavailable {
		state.apply(st)
	}
	step := orderedFacetStep{
		MinInclusive: state.stepSingleFacets&valuepkg.FacetMinInclusive != 0,
		MinExclusive: state.stepSingleFacets&valuepkg.FacetMinExclusive != 0,
		MaxInclusive: state.stepSingleFacets&valuepkg.FacetMaxInclusive != 0,
		MaxExclusive: state.stepSingleFacets&valuepkg.FacetMaxExclusive != 0,
	}
	if err := validateOrderedFacetStep(step); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, err.Error())
	}
	if state.unavailable {
		state.apply(st)
	}
	return nil
}

type compiledFacetState struct {
	valueEnumeration []valuepkg.LiteralSpec
	valuePatterns    []*valuepkg.Pattern
	stepSingleFacets valuepkg.FacetMask
	// unavailable marks a base lost to optional-import recovery. Bound and
	// enumeration literals are then skipped rather than evaluated, while
	// source checks and facets decidable without the base still run.
	unavailable bool
}

func (s *compiledFacetState) apply(st *simpleType) {
	if len(s.valueEnumeration) != 0 {
		st.ValueSpec.Facets.Enumeration = slices.Clone(s.valueEnumeration)
		st.ValueSpec.Facets.Present |= valuepkg.FacetEnumeration
	}
	if len(s.valuePatterns) != 0 {
		// Patterns declared by one restriction step form one OR group. The
		// value builder intersects groups inherited through the base chain.
		st.ValueSpec.Facets.Patterns = append(st.ValueSpec.Facets.Patterns, slices.Clone(s.valuePatterns))
		st.ValueSpec.Facets.Present |= valuepkg.FacetPattern
	}
}

func (c *compiler) compileFacetChild(child *schemaNode, st *simpleType, base SimpleTypeID, state *compiledFacetState) error {
	flag, err := validateFacetChildSource(child, st, &state.stepSingleFacets)
	if err != nil || flag == 0 {
		return err
	}
	facet, err := facetAttrs(child, flag)
	if err != nil {
		return err
	}
	return c.compileFacetValue(child, st, base, state, facet)
}

func validateFacetChildSource(child *schemaNode, st *simpleType, single *valuepkg.FacetMask) (valuepkg.FacetMask, error) {
	source := semanticSource[*schemaFacetSource](&child.semantic)
	if source == nil {
		return 0, withSchemaCompileLocation(child, xsderrors.InternalInvariant("facet node has no typed facet source"))
	}
	compile, err := validateFacetSource(facetSource{
		Local: child.local, InXSDNamespace: true, HasValue: source.Value.Present,
		Variety: st.ValueSpec.Variety, Primitive: st.ValueSpec.Primitive,
	})
	if err != nil {
		return 0, withSchemaCompileLocation(child, err)
	}
	if !compile {
		return 0, nil
	}
	mask, _ := facetMaskForLocal(child.local)
	if mask != valuepkg.FacetPattern && mask != valuepkg.FacetEnumeration {
		if *single&mask != 0 {
			return 0, withSchemaCompileLocation(child, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "duplicate "+facetLabel(child.local)+" facet"))
		}
		*single |= mask
	}
	return mask, nil
}

func (c *compiler) compileFacetValue(child *schemaNode, st *simpleType, base SimpleTypeID, state *compiledFacetState, facet facetInput) error {
	switch facet.flag {
	case valuepkg.FacetLength, valuepkg.FacetMinLength, valuepkg.FacetMaxLength, valuepkg.FacetTotalDigits, valuepkg.FacetFractionDigits:
		return compileSizeFacet(st, child, facet)
	case valuepkg.FacetMinInclusive, valuepkg.FacetMaxInclusive, valuepkg.FacetMinExclusive, valuepkg.FacetMaxExclusive:
		return c.compileBoundFacet(st, base, child, facet, state)
	case valuepkg.FacetEnumeration:
		if state.unavailable {
			return nil
		}
		return c.compileEnumerationFacet(child, base, facet.value, state)
	case valuepkg.FacetPattern:
		return c.compilePatternFacet(child, facet.value, state)
	case valuepkg.FacetWhiteSpace:
		return c.compileWhitespaceFacet(st, child, facet)
	}
	return nil
}

func (c *compiler) compileEnumerationFacet(child *schemaNode, literalType SimpleTypeID, lexical string, state *compiledFacetState) error {
	literal, err := c.compileLiteral(literalType, lexical, schemaQNameResolver(child))
	if err != nil {
		return withSchemaCompileLocation(child, err)
	}
	state.valueEnumeration = append(state.valueEnumeration, literal)
	return nil
}

func (c *compiler) compilePatternFacet(child *schemaNode, patternText string, state *compiledFacetState) error {
	if pattern, ok := c.patterns[patternText]; ok {
		state.valuePatterns = append(state.valuePatterns, pattern)
		return nil
	}
	pattern, err := xsdregex.Compile(patternText)
	if err != nil {
		return withSchemaCompileLocation(child, schemaRegexError(err))
	}
	n := pattern.RetainedBytes()
	// The charged total never exceeds the normalized, non-negative limit.
	remaining := uint64(c.limits.MaxSchemaPatternRetainedBytes - c.patternRetainedBytes) //nolint:gosec // Normalization rejects negative limits and charges stay within them.
	if n > remaining {
		return withSchemaCompileLocation(child, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit,
			fmt.Sprintf("MaxSchemaPatternRetainedBytes=%d exceeded", c.limits.MaxSchemaPatternRetainedBytes)))
	}
	c.patternRetainedBytes += int64(n) //nolint:gosec // n <= remaining <= MaxInt64.
	if c.patterns == nil {
		c.patterns = make(map[string]*xsdregex.Pattern)
	}
	c.patterns[patternText] = pattern
	state.valuePatterns = append(state.valuePatterns, pattern)
	return nil
}

type facetInput struct {
	value string
	flag  valuepkg.FacetMask
	fixed bool
}

func facetAttrs(n *schemaNode, flag valuepkg.FacetMask) (facetInput, error) {
	source := semanticSource[*schemaFacetSource](&n.semantic)
	if source == nil {
		return facetInput{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("facet node has no typed facet source"))
	}
	value := source.Value.Value
	fixed, err := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrFixed, Value: source.Fixed.Value, HasValue: source.Fixed.Present})
	if err != nil {
		return facetInput{}, err
	}
	return facetInput{value: value, flag: flag, fixed: fixed}, nil
}

func compileSizeFacet(st *simpleType, node *schemaNode, facet facetInput) error {
	name := node.local
	size, err := parseSizeFacetValue(name, facet.value)
	if err != nil {
		return withSchemaCompileLocation(node, err)
	}
	setValueCardinalityFacet(&st.ValueSpec.Facets, facet.flag, size)
	if facet.fixed {
		st.ValueSpec.Facets.Fixed |= facet.flag
	}
	return nil
}

func (c *compiler) compileBoundFacet(st *simpleType, base SimpleTypeID, child *schemaNode, facet facetInput, state *compiledFacetState) error {
	var literal valuepkg.LiteralSpec
	if !state.unavailable {
		var err error
		literal, err = c.compileLiteral(base, facet.value, schemaQNameResolver(child))
		if err != nil {
			return err
		}
	}
	if state.unavailable {
		return nil
	}
	setValueBoundFacet(&st.ValueSpec.Facets, facet, literal)
	return nil
}

func (*compiler) compileWhitespaceFacet(st *simpleType, n *schemaNode, facet facetInput) error {
	mode, err := parseWhitespaceFacetValue(facet.value)
	if err != nil {
		return withSchemaCompileLocation(n, err)
	}
	st.ValueSpec.Whitespace = mode
	st.ValueSpec.WhitespacePresent = true
	if facet.fixed {
		st.ValueSpec.Facets.Fixed |= valuepkg.FacetWhiteSpace
	}
	return nil
}

func setValueCardinalityFacet(f *valuepkg.FacetSpec, flag valuepkg.FacetMask, size uint32) {
	f.Present |= flag
	cardinality := valuepkg.CardinalityFacet{Value: size, Present: true}
	switch flag {
	case valuepkg.FacetLength:
		f.Length = cardinality
	case valuepkg.FacetMinLength:
		f.MinLength = cardinality
	case valuepkg.FacetMaxLength:
		f.MaxLength = cardinality
	case valuepkg.FacetTotalDigits:
		f.TotalDigits = cardinality
	case valuepkg.FacetFractionDigits:
		f.FractionDigits = cardinality
	case valuepkg.FacetMinInclusive, valuepkg.FacetMaxInclusive, valuepkg.FacetMinExclusive, valuepkg.FacetMaxExclusive,
		valuepkg.FacetEnumeration, valuepkg.FacetPattern, valuepkg.FacetWhiteSpace:
	}
}

func setValueBoundFacet(f *valuepkg.FacetSpec, facet facetInput, literal valuepkg.LiteralSpec) {
	bound := valuepkg.BoundFacet{LiteralSpec: literal, Present: true}
	switch facet.flag {
	case valuepkg.FacetMinInclusive:
		f.MinInclusive = bound
	case valuepkg.FacetMaxInclusive:
		f.MaxInclusive = bound
	case valuepkg.FacetMinExclusive:
		f.MinExclusive = bound
	case valuepkg.FacetMaxExclusive:
		f.MaxExclusive = bound
	case valuepkg.FacetLength, valuepkg.FacetMinLength, valuepkg.FacetMaxLength, valuepkg.FacetTotalDigits, valuepkg.FacetFractionDigits,
		valuepkg.FacetEnumeration, valuepkg.FacetPattern, valuepkg.FacetWhiteSpace:
	}
	f.Present |= facet.flag
	if facet.fixed {
		f.Fixed |= facet.flag
	}
}

func (c *compiler) compileLiteral(base SimpleTypeID, lexical string, resolve valuepkg.QNameResolver) (valuepkg.LiteralSpec, error) {
	_, names, err := c.validateValue(base, lexical, resolve, valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	if err != nil {
		return valuepkg.LiteralSpec{}, schemaValueError(c.limits, xsderrors.CodeSchemaFacet, "invalid facet value "+diagnostictext.Quote(lexical), err)
	}
	literal := valuepkg.LiteralSpec{
		Lexical: lexical,
		Type:    base,
		Resolver: valuepkg.Resolver{Notation: func(namespace, local string) bool {
			q, ok := c.rt.lookupQName(namespace, local)
			return ok && c.rt.notationDeclared(q)
		}},
	}
	if len(names) != 0 {
		replay, replayErr := newValueConstraintNameReplay(names)
		if replayErr != nil {
			return valuepkg.LiteralSpec{}, schemaValueError(c.limits, xsderrors.CodeSchemaFacet, "invalid facet value "+diagnostictext.Quote(lexical), replayErr)
		}
		literal.Resolver.QName = replay.ResolveQName
	}
	return literal, nil
}

func schemaRegexError(err error) error {
	if err == nil {
		return nil
	}
	if xsderrors.IsUnsupported(err) {
		return err
	}
	if xsdregex.IsLimit(err) {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "pattern exceeds fixed compile limit: "+err.Error())
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "invalid pattern: "+err.Error())
}
