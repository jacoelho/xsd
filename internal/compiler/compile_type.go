package compiler

import (
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *Compiler) compileType(qname types.QName, typ types.Type) (*grammar.CompiledType, error) {
	// check pointer-based cache first (handles all types including anonymous)
	// this prevents infinite recursion for circular references
	if compiled, ok := c.typesByPtr[typ]; ok {
		return compiled, nil
	}

	// check QName cache for named types (for cross-schema references)
	if !qname.IsZero() {
		if compiled, ok := c.types[qname]; ok {
			return compiled, nil
		}
	}

	compiled := &grammar.CompiledType{
		QName:    qname,
		Original: typ,
	}

	// add to pointer cache immediately (before recursive compilation)
	c.typesByPtr[typ] = compiled

	// add to QName-based caches only for the first compilation of a QName
	// (in redefine context, we want the redefined type in the grammar, not the original)
	if !qname.IsZero() {
		if _, exists := c.types[qname]; !exists {
			c.types[qname] = compiled
			c.grammar.Types[qname] = compiled
		}
	} else {
		c.anonymousTypes = append(c.anonymousTypes, compiled)
	}

	switch t := typ.(type) {
	case *types.BuiltinType:
		compiled.Kind = grammar.TypeKindBuiltin
		compiled.DerivationChain = []*grammar.CompiledType{compiled}
		if baseBuiltin, ok := t.BaseType().(*types.BuiltinType); ok {
			baseCompiled, err := c.compileType(baseBuiltin.Name(), baseBuiltin)
			if err != nil {
				return nil, err
			}
			compiled.BaseType = baseCompiled
			compiled.DerivationMethod = types.DerivationRestriction
			compiled.DerivationChain = append([]*grammar.CompiledType{compiled}, baseCompiled.DerivationChain...)
		}
		// check if this is the NOTATION built-in type
		compiled.IsNotationType = t.Name().Local == string(types.TypeNameNOTATION)
		compiled.IsQNameOrNotationType = types.IsQNameOrNotation(t.Name())
		// precompute ID type name for ID/IDREF/IDREFS tracking
		compiled.IDTypeName = getIDTypeName(t.Name().Local)

	case *types.SimpleType:
		compiled.Kind = grammar.TypeKindSimple
		if err := c.compileSimpleType(compiled, t); err != nil {
			return nil, err
		}

	case *types.ComplexType:
		compiled.Kind = grammar.TypeKindComplex
		if err := c.compileComplexType(compiled, t); err != nil {
			return nil, err
		}
	}

	return compiled, nil
}

func (c *Compiler) compileSimpleType(compiled *grammar.CompiledType, simpleType *types.SimpleType) error {
	resolvedBase := simpleType.ResolvedBase
	if resolvedBase == nil && simpleType.Restriction != nil && !simpleType.Restriction.Base.IsZero() {
		if builtinType := types.GetBuiltinNS(simpleType.Restriction.Base.Namespace, simpleType.Restriction.Base.Local); builtinType != nil {
			resolvedBase = builtinType
		} else if base, ok := c.schema.TypeDefs[simpleType.Restriction.Base]; ok {
			resolvedBase = base
		}
	}
	if resolvedBase == nil {
		resolvedBase = types.GetBuiltin(types.TypeNameAnySimpleType)
	}
	if resolvedBase != nil {
		baseCompiled, err := c.compileType(resolvedBase.Name(), resolvedBase)
		if err != nil {
			return err
		}
		compiled.BaseType = baseCompiled
		compiled.DerivationChain = append([]*grammar.CompiledType{compiled}, baseCompiled.DerivationChain...)
		// simple types derived from a base are always restrictions
		compiled.DerivationMethod = types.DerivationRestriction
		// propagate NOTATION type flag from base
		compiled.IsNotationType = baseCompiled.IsNotationType
		// propagate ID type name from base
		compiled.IDTypeName = baseCompiled.IDTypeName
	} else {
		compiled.DerivationChain = []*grammar.CompiledType{compiled}
	}

	// compute primitive type (for atomic types)
	compiled.PrimitiveType = c.findPrimitiveType(compiled)
	if compiled.PrimitiveType != nil && compiled.PrimitiveType.QName.Local == string(types.TypeNameNOTATION) {
		compiled.IsNotationType = true
	}

	// compile item type (for list)
	if simpleType.ItemType != nil {
		itemCompiled, err := c.compileType(simpleType.ItemType.Name(), simpleType.ItemType)
		if err != nil {
			return err
		}
		compiled.ItemType = itemCompiled
		compiled.DerivationMethod = types.DerivationList
	}
	if compiled.ItemType == nil && simpleType.Variety() == types.ListVariety && compiled.BaseType != nil {
		compiled.ItemType = compiled.BaseType.ItemType
	}

	// compile member types (for union)
	if len(simpleType.MemberTypes) > 0 {
		compiled.MemberTypes = make([]*grammar.CompiledType, len(simpleType.MemberTypes))
		for i, member := range simpleType.MemberTypes {
			memberCompiled, err := c.compileType(member.Name(), member)
			if err != nil {
				return err
			}
			compiled.MemberTypes[i] = memberCompiled
		}
		if simpleType.Union != nil {
			compiled.DerivationMethod = types.DerivationUnion
		}
	}
	if len(compiled.MemberTypes) == 0 && simpleType.Variety() == types.UnionVariety && compiled.BaseType != nil {
		if len(compiled.BaseType.MemberTypes) > 0 {
			compiled.MemberTypes = append([]*grammar.CompiledType(nil), compiled.BaseType.MemberTypes...)
		}
	}

	compiled.IsQNameOrNotationType = c.isQNameOrNotationType(compiled)
	simpleType.SetQNameOrNotationType(compiled.IsQNameOrNotationType)

	compiled.Facets = c.collectFacets(simpleType)
	if compiled.IsQNameOrNotationType {
		if err := c.resolveQNameEnumerationFacets(compiled); err != nil {
			return err
		}
	}

	// precompute caches to avoid lazy writes during validation.
	types.PrecomputeSimpleTypeCaches(simpleType)

	return nil
}

func (c *Compiler) compileComplexType(compiled *grammar.CompiledType, complexType *types.ComplexType) error {
	compiled.Abstract = complexType.Abstract
	compiled.Mixed = complexType.Mixed()
	compiled.Final = complexType.Final
	compiled.Block = complexType.Block
	compiled.DerivationMethod = complexType.DerivationMethod

	resolvedBase := complexType.ResolvedBase
	if resolvedBase == nil {
		resolvedBase = types.GetBuiltin(types.TypeNameAnyType)
	}
	if resolvedBase != nil {
		baseCompiled, err := c.compileType(resolvedBase.Name(), resolvedBase)
		if err != nil {
			return err
		}
		compiled.BaseType = baseCompiled
		if complexType.ResolvedBase == nil && compiled.DerivationMethod == 0 {
			compiled.DerivationMethod = types.DerivationRestriction
		}
		compiled.DerivationChain = append([]*grammar.CompiledType{compiled}, baseCompiled.DerivationChain...)
	} else {
		compiled.DerivationChain = []*grammar.CompiledType{compiled}
	}

	// pre-merge all attributes from derivation chain
	compiled.AllAttributes = c.mergeAttributes(compiled.DerivationChain)

	compiled.AnyAttribute = c.mergeAnyAttribute(compiled.DerivationChain)

	if complexType.Content() != nil {
		compiled.ContentModel = c.compileContentModel(complexType)
		c.applyComplexContentExtension(compiled, complexType)
	}

	if compiled.ContentModel != nil {
		c.populateContentModelCaches(compiled.ContentModel)
	}

	// for simpleContent, set up text content validation type
	if sc, ok := complexType.Content().(*types.SimpleContent); ok {
		c.setupSimpleContentType(compiled, sc)
	}

	return nil
}

func (c *Compiler) applyComplexContentExtension(compiled *grammar.CompiledType, complexType *types.ComplexType) {
	cc, ok := complexType.Content().(*types.ComplexContent)
	if !ok || cc.Extension == nil {
		return
	}
	if compiled.BaseType == nil || compiled.BaseType.ContentModel == nil || compiled.BaseType.ContentModel.Empty {
		return
	}
	baseParticles := compiled.BaseType.ContentModel.Particles
	if len(baseParticles) == 0 {
		return
	}
	if compiled.ContentModel == nil || compiled.ContentModel.Empty {
		compiled.ContentModel = &grammar.CompiledContentModel{
			Kind:      compiled.BaseType.ContentModel.Kind,
			Particles: baseParticles,
			Mixed:     compiled.Mixed,
		}
		return
	}
	combined := make([]*grammar.CompiledParticle, 0, len(baseParticles)+len(compiled.ContentModel.Particles))
	combined = append(combined, baseParticles...)
	combined = append(combined, compiled.ContentModel.Particles...)
	compiled.ContentModel.Particles = combined
	compiled.ContentModel.Kind = types.Sequence
}

func (c *Compiler) setupSimpleContentType(compiled *grammar.CompiledType, sc *types.SimpleContent) {
	baseType := compiled.BaseType
	if baseType == nil {
		return
	}

	if sc.Extension != nil {
		// extension: inherit base type's simple content type or use base directly if simple
		if baseType.Kind == grammar.TypeKindSimple || baseType.Kind == grammar.TypeKindBuiltin {
			compiled.SimpleContentType = baseType
		} else if baseType.SimpleContentType != nil {
			// base is complex with simpleContent - inherit its SimpleContentType
			compiled.SimpleContentType = baseType.SimpleContentType
		}
	} else if sc.Restriction != nil {
		// restriction: use base's SimpleContentType and add our facets
		if baseType.SimpleContentType != nil {
			compiled.SimpleContentType = baseType.SimpleContentType
		} else if baseType.Kind == grammar.TypeKindSimple || baseType.Kind == grammar.TypeKindBuiltin {
			compiled.SimpleContentType = baseType
		}
		if sc.Restriction.Facets != nil {
			for _, f := range sc.Restriction.Facets {
				if facet, ok := f.(types.Facet); ok {
					compiled.Facets = append(compiled.Facets, facet)
				}
			}
		}
	}
}

func (c *Compiler) findPrimitiveType(compiledType *grammar.CompiledType) *grammar.CompiledType {
	for _, t := range compiledType.DerivationChain {
		if t.Kind == grammar.TypeKindBuiltin {
			return t
		}
	}
	return nil
}

func (c *Compiler) isQNameOrNotationType(compiledType *grammar.CompiledType) bool {
	if compiledType == nil || compiledType.ItemType != nil {
		return false
	}
	if compiledType.PrimitiveType != nil && types.IsQNameOrNotation(compiledType.PrimitiveType.QName) {
		return true
	}
	if compiledType.BaseType != nil {
		return compiledType.BaseType.IsQNameOrNotationType
	}
	return types.IsQNameOrNotation(compiledType.QName)
}

func (c *Compiler) collectFacets(simpleType *types.SimpleType) []types.Facet {
	var result []types.Facet

	// collect facets from base type first (inherited facets)
	// per XSD spec, patterns from different derivation steps are ANDed together,
	// so inherited patterns become separate entries in the result.
	if simpleType.ResolvedBase != nil {
		if baseSimpleType, ok := simpleType.ResolvedBase.(*types.SimpleType); ok {
			result = append(result, c.collectFacets(baseSimpleType)...)
		}
	}

	// collect facets from this type's restriction
	// per XSD spec, patterns from the SAME derivation step are ORed together.
	// we group all pattern facets from this step into a PatternSet.
	if simpleType.Restriction != nil {
		var stepPatterns []*types.Pattern

		for _, f := range simpleType.Restriction.Facets {
			switch facet := f.(type) {
			case types.Facet:
				// check if this is a pattern facet
				if patternFacet, ok := facet.(*types.Pattern); ok {
					if err := patternFacet.ValidateSyntax(); err != nil {
						// skip invalid patterns (schema validation should have caught this)
						continue
					}
					stepPatterns = append(stepPatterns, patternFacet)
				} else {
					// non-pattern facet: compile if needed and add directly
					if compilable, ok := facet.(interface{ ValidateSyntax() error }); ok {
						if err := compilable.ValidateSyntax(); err != nil {
							continue
						}
					}
					result = append(result, facet)
				}
			case *types.DeferredFacet:
				// convert deferred facets to real facets now that base type is resolved
				realFacet := c.constructDeferredFacet(facet, simpleType)
				if realFacet != nil {
					result = append(result, realFacet)
				}
			}
		}

		// group patterns from this step into a PatternSet (OR semantics)
		if len(stepPatterns) == 1 {
			// single pattern - no need for PatternSet
			result = append(result, stepPatterns[0])
		} else if len(stepPatterns) > 1 {
			// multiple patterns - group into PatternSet for OR semantics
			result = append(result, &types.PatternSet{Patterns: stepPatterns})
		}
	}
	return result
}

func (c *Compiler) resolveQNameEnumerationFacets(compiled *grammar.CompiledType) error {
	if compiled == nil || len(compiled.Facets) == 0 {
		return nil
	}
	for _, facet := range compiled.Facets {
		enum, ok := facet.(*types.Enumeration)
		if !ok || len(enum.Values) == 0 {
			continue
		}
		if len(enum.QNameValues) == len(enum.Values) {
			continue
		}
		qnames, err := enum.ResolveQNameValues()
		if err != nil {
			return err
		}
		enum.QNameValues = qnames
	}
	return nil
}

// constructDeferredFacet converts a DeferredFacet to a real Facet using the resolved base type.
func (c *Compiler) constructDeferredFacet(df *types.DeferredFacet, simpleType *types.SimpleType) types.Facet {
	baseType := simpleType.ResolvedBase
	if baseType == nil {
		return nil
	}

	var facet types.Facet
	var err error

	switch df.FacetName {
	case "minInclusive":
		facet, err = types.NewMinInclusive(df.FacetValue, baseType)
	case "maxInclusive":
		facet, err = types.NewMaxInclusive(df.FacetValue, baseType)
	case "minExclusive":
		facet, err = types.NewMinExclusive(df.FacetValue, baseType)
	case "maxExclusive":
		facet, err = types.NewMaxExclusive(df.FacetValue, baseType)
	}

	if err != nil {
		// log or handle error - facet construction failed even with resolved type
		// this could happen for genuinely invalid schemas
		return nil
	}

	return facet
}
