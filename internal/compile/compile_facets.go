package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) compileFacets(parent *rawNode, st *runtime.SimpleType, base, literalType runtime.SimpleTypeID) error {
	return withSchemaCompileLocation(parent, c.compileFacetChildren(parent.Children, st, base, literalType, true))
}

func (c *compiler) compileFacetList(children []*rawNode, st *runtime.SimpleType, base, literalType runtime.SimpleTypeID) error {
	return c.compileFacetChildren(children, st, base, literalType, false)
}

func (c *compiler) compileFacetChildren(children []*rawNode, st *runtime.SimpleType, base, literalType runtime.SimpleTypeID, skipNonFacets bool) error {
	var state compiledFacetState
	for _, child := range children {
		if child.Name.Space != vocab.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation || child.Name.Local == vocab.XSDElemSimpleType {
			continue
		}
		if skipNonFacets && !IsFacetLocal(child.Name.Local) {
			continue
		}
		if err := c.compileFacetChild(child, st, base, literalType, &state); err != nil {
			return err
		}
	}
	if !state.sawFacet {
		return nil
	}
	state.apply(st)
	return c.validateCompiledFacetsBuild(*st, base, state.orderedStep)
}

func (c *compiler) validateUnavailableFacetChildren(
	children []*rawNode,
	st *runtime.SimpleType,
	base runtime.SimpleTypeID,
	skipNonFacets bool,
) error {
	audit := unavailableFacetAudit{compiler: c, probe: *st, base: base}
	for _, child := range children {
		if !isFacetCompilationChild(child, skipNonFacets) {
			continue
		}
		if err := audit.add(child); err != nil {
			return err
		}
	}
	if err := runtime.ValidateOrderedFacetStep(audit.ordered); err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, err.Error())
	}
	if len(audit.patterns) != 0 {
		runtime.AppendPatternFacetGroup(&audit.probe.Facets, audit.patterns)
	}
	if err := c.validateCompiledFacetsBuild(audit.probe, base, runtime.OrderedFacetStep{}); err != nil {
		return err
	}
	*st = audit.probe
	return nil
}

func isFacetCompilationChild(child *rawNode, skipNonFacets bool) bool {
	if child.Name.Space != vocab.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation || child.Name.Local == vocab.XSDElemSimpleType {
		return false
	}
	return !skipNonFacets || IsFacetLocal(child.Name.Local)
}

type unavailableFacetAudit struct {
	compiler *compiler
	patterns []runtime.StringPattern
	probe    runtime.SimpleType
	base     runtime.SimpleTypeID
	single   runtime.FacetMask
	ordered  runtime.OrderedFacetStep
}

func (a *unavailableFacetAudit) add(child *rawNode) error {
	compile, err := validateFacetChildSource(child, &a.probe, &a.single)
	if err != nil || !compile {
		return err
	}
	facet, err := facetAttrs(child)
	if err != nil {
		return err
	}
	return a.compile(child, facet)
}

func (a *unavailableFacetAudit) compile(child *rawNode, facet facetInput) error {
	switch child.Name.Local {
	case vocab.XSDFacetLength, vocab.XSDFacetMinLength, vocab.XSDFacetMaxLength, vocab.XSDFacetTotalDigits, vocab.XSDFacetFractionDigits:
		return compileSizeFacet(&a.probe, child, facet.value, facet.fixed)
	case vocab.XSDFacetMinInclusive:
		a.ordered.MinInclusive = true
	case vocab.XSDFacetMaxInclusive:
		a.ordered.MaxInclusive = true
	case vocab.XSDFacetMinExclusive:
		a.ordered.MinExclusive = true
	case vocab.XSDFacetMaxExclusive:
		a.ordered.MaxExclusive = true
	case vocab.XSDFacetPattern:
		return a.compilePattern(child, facet.value)
	case vocab.XSDFacetWhiteSpace:
		return a.compiler.compileWhitespaceFacet(&a.probe, a.base, child, facet.value, facet.fixed)
	}
	return nil
}

func (a *unavailableFacetAudit) compilePattern(child *rawNode, value string) error {
	if a.compiler.regexCategories == nil {
		a.compiler.regexCategories = make(RegexCategoryCache)
	}
	pattern, err := CompilePatternFacet(value, a.compiler.regexCategories)
	if err != nil {
		return withSchemaCompileLocation(child, err)
	}
	a.patterns = append(a.patterns, pattern)
	return nil
}

type compiledFacetState struct {
	inheritedEnumeration  []runtime.CompiledLiteral
	restrictedEnumeration []runtime.CompiledLiteral
	stepPatterns          []runtime.StringPattern
	orderedStep           runtime.OrderedFacetStep
	stepSingleFacets      runtime.FacetMask
	sawEnumeration        bool
	sawFacet              bool
}

func (s *compiledFacetState) beginStep(st *runtime.SimpleType) {
	if s.sawFacet {
		return
	}
	s.inheritedEnumeration = st.Facets.Enumeration
	s.sawFacet = true
}

func (s *compiledFacetState) apply(st *runtime.SimpleType) {
	if s.sawEnumeration {
		st.Facets.Enumeration = s.restrictedEnumeration
	} else {
		st.Facets.Enumeration = s.inheritedEnumeration
	}
	if len(st.Facets.Enumeration) != 0 {
		runtime.SetFacetPresent(&st.Facets, runtime.FacetEnumeration)
	} else {
		runtime.ClearFacet(&st.Facets, runtime.FacetEnumeration)
	}
	if len(s.stepPatterns) != 0 {
		runtime.AppendPatternFacetGroup(&st.Facets, s.stepPatterns)
	}
}

func (c *compiler) compileFacetChild(child *rawNode, st *runtime.SimpleType, base, literalType runtime.SimpleTypeID, state *compiledFacetState) error {
	compile, err := validateFacetChildSource(child, st, &state.stepSingleFacets)
	if err != nil || !compile {
		return err
	}
	state.beginStep(st)
	facet, err := facetAttrs(child)
	if err != nil {
		return err
	}
	return c.compileFacetValue(child, st, base, literalType, state, facet)
}

func validateFacetChildSource(child *rawNode, st *runtime.SimpleType, single *runtime.FacetMask) (bool, error) {
	_, hasValue := child.attr(vocab.XSDAttrValue)
	compile, err := ValidateFacetSource(FacetSource{
		Local: child.Name.Local, InXSDNamespace: child.Name.Space == vocab.XSDNamespaceURI, HasValue: hasValue,
		Variety: st.Variety, Primitive: st.Primitive,
	})
	if err != nil {
		return false, withSchemaCompileLocation(child, err)
	}
	if !compile {
		return false, nil
	}
	mask, _ := facetMaskForLocal(child.Name.Local)
	if mask != runtime.FacetPattern && mask != runtime.FacetEnumeration {
		if *single&mask != 0 {
			return false, withSchemaCompileLocation(child, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "duplicate "+child.Name.Local+" facet"))
		}
		*single |= mask
	}
	return true, nil
}

func (c *compiler) compileFacetValue(child *rawNode, st *runtime.SimpleType, base, literalType runtime.SimpleTypeID, state *compiledFacetState, facet facetInput) error {
	switch child.Name.Local {
	case vocab.XSDFacetLength, vocab.XSDFacetMinLength, vocab.XSDFacetMaxLength, vocab.XSDFacetTotalDigits, vocab.XSDFacetFractionDigits:
		return compileSizeFacet(st, child, facet.value, facet.fixed)
	case vocab.XSDFacetMinInclusive, vocab.XSDFacetMaxInclusive, vocab.XSDFacetMinExclusive, vocab.XSDFacetMaxExclusive:
		return c.compileBoundFacet(st, base, child, facet.value, facet.fixed, &state.orderedStep)
	case vocab.XSDFacetEnumeration:
		return c.compileEnumerationFacet(child, literalType, facet.value, state)
	case vocab.XSDFacetPattern:
		return c.compilePatternFacet(child, facet.value, state)
	case vocab.XSDFacetWhiteSpace:
		return c.compileWhitespaceFacet(st, base, child, facet.value, facet.fixed)
	}
	return nil
}

func (c *compiler) compileEnumerationFacet(child *rawNode, literalType runtime.SimpleTypeID, value string, state *compiledFacetState) error {
	literal, err := c.compileLiteral(literalType, value, c.schemaQNameResolver(child))
	if err != nil {
		return withSchemaCompileLocation(child, err)
	}
	state.restrictedEnumeration = append(state.restrictedEnumeration, literal)
	state.sawEnumeration = true
	return nil
}

func (c *compiler) compilePatternFacet(child *rawNode, value string, state *compiledFacetState) error {
	if c.regexCategories == nil {
		c.regexCategories = make(RegexCategoryCache)
	}
	pattern, err := CompilePatternFacet(value, c.regexCategories)
	if err != nil {
		return withSchemaCompileLocation(child, err)
	}
	state.stepPatterns = append(state.stepPatterns, pattern)
	return nil
}

type facetInput struct {
	value string
	fixed bool
}

func facetAttrs(n *rawNode) (facetInput, error) {
	value, _ := n.attr(vocab.XSDAttrValue)
	fixed, err := schemaBoolAttr(n, vocab.XSDAttrFixed)
	if err != nil {
		return facetInput{}, err
	}
	return facetInput{value: value, fixed: fixed}, nil
}

func compileSizeFacet(st *runtime.SimpleType, node *rawNode, value string, fixed bool) error {
	name := node.Name.Local
	size, err := ParseSizeFacetValue(name, value)
	if err != nil {
		return withSchemaCompileLocation(node, err)
	}
	var flag runtime.FacetMask
	switch name {
	case vocab.XSDFacetLength:
		st.Facets.Length = size
		flag = runtime.FacetLength
	case vocab.XSDFacetMinLength:
		st.Facets.MinLength = size
		flag = runtime.FacetMinLength
	case vocab.XSDFacetMaxLength:
		st.Facets.MaxLength = size
		flag = runtime.FacetMaxLength
	case vocab.XSDFacetTotalDigits:
		st.Facets.TotalDigits = size
		flag = runtime.FacetTotalDigits
	case vocab.XSDFacetFractionDigits:
		st.Facets.FractionDigits = size
		flag = runtime.FacetFractionDigits
	}
	runtime.SetFacet(&st.Facets, flag, fixed)
	return nil
}

func (c *compiler) compileBoundFacet(st *runtime.SimpleType, base runtime.SimpleTypeID, child *rawNode, value string, fixed bool, step *runtime.OrderedFacetStep) error {
	lit, err := c.compileLiteral(base, value, c.schemaQNameResolver(child))
	if err != nil {
		return err
	}
	var flag runtime.FacetMask
	switch child.Name.Local {
	case vocab.XSDFacetMinInclusive:
		flag = runtime.FacetMinInclusive
		step.MinInclusive = true
	case vocab.XSDFacetMaxInclusive:
		flag = runtime.FacetMaxInclusive
		step.MaxInclusive = true
	case vocab.XSDFacetMinExclusive:
		flag = runtime.FacetMinExclusive
		step.MinExclusive = true
	case vocab.XSDFacetMaxExclusive:
		flag = runtime.FacetMaxExclusive
		step.MaxExclusive = true
	}
	runtime.SetBoundFacet(&st.Facets, flag, lit, fixed)
	return nil
}

func (c *compiler) compileWhitespaceFacet(st *runtime.SimpleType, base runtime.SimpleTypeID, n *rawNode, value string, fixed bool) error {
	mode, err := ParseWhitespaceFacetValue(value, c.rt.simpleTypeWhitespace(base))
	if err != nil {
		return withSchemaCompileLocation(n, err)
	}
	st.Whitespace = mode
	runtime.SetWhiteSpaceFacetFixed(&st.Facets, fixed)
	return nil
}

func (c *compiler) compileLiteral(base runtime.SimpleTypeID, lexical string, resolve runtime.ResolveQNameParts) (runtime.CompiledLiteral, error) {
	recorder := valueConstraintResolver{resolve: resolve}
	replayResolve := resolve
	if resolve != nil {
		replayResolve = recorder.resolveQName
	}
	value, err := c.validateSimpleValue(base, lexical, replayResolve, runtime.SimpleNeedCanonical)
	if err != nil {
		return runtime.CompiledLiteral{}, FacetValueError(lexical, err)
	}
	return c.compiledLiteralForSimpleType(base, lexical, value.Canonical, recorder.names), nil
}
