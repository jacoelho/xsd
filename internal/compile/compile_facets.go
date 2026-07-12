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
		if child.Name.Space != runtime.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation || child.Name.Local == vocab.XSDElemSimpleType {
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
	_, hasValue := child.attr(vocab.XSDAttrValue)
	compile, err := ValidateFacetSource(FacetSource{
		Local:          child.Name.Local,
		InXSDNamespace: child.Name.Space == vocab.XSDNamespaceURI,
		HasValue:       hasValue,
		Variety:        st.Variety,
		Primitive:      st.Primitive,
	})
	if err != nil {
		return withSchemaCompileLocation(child, err)
	}
	if !compile {
		return nil
	}
	mask, _ := facetMaskForLocal(child.Name.Local)
	if mask != runtime.FacetPattern && mask != runtime.FacetEnumeration {
		if state.stepSingleFacets&mask != 0 {
			return withSchemaCompileLocation(child, xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, "duplicate "+child.Name.Local+" facet"))
		}
		state.stepSingleFacets |= mask
	}
	state.beginStep(st)
	facet, err := facetAttrs(child)
	if err != nil {
		return err
	}
	switch child.Name.Local {
	case vocab.XSDFacetLength, vocab.XSDFacetMinLength, vocab.XSDFacetMaxLength, vocab.XSDFacetTotalDigits, vocab.XSDFacetFractionDigits:
		return compileSizeFacet(st, child, facet.value, facet.fixed)
	case vocab.XSDFacetMinInclusive, vocab.XSDFacetMaxInclusive, vocab.XSDFacetMinExclusive, vocab.XSDFacetMaxExclusive:
		return c.compileBoundFacet(st, base, child, facet.value, facet.fixed, &state.orderedStep)
	case vocab.XSDFacetEnumeration:
		lit, err := c.compileLiteral(literalType, facet.value, c.schemaQNameResolver(child))
		if err != nil {
			return withSchemaCompileLocation(child, err)
		}
		state.restrictedEnumeration = append(state.restrictedEnumeration, lit)
		state.sawEnumeration = true
	case vocab.XSDFacetPattern:
		if c.regexCategories == nil {
			c.regexCategories = make(RegexCategoryCache)
		}
		p, err := CompilePatternFacet(facet.value, c.regexCategories)
		if err != nil {
			return withSchemaCompileLocation(child, err)
		}
		state.stepPatterns = append(state.stepPatterns, p)
	case vocab.XSDFacetWhiteSpace:
		return c.compileWhitespaceFacet(st, base, child, facet.value, facet.fixed)
	}
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
