package loader

import (
	"github.com/jacoelho/xsd/internal/facets"
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
		base := t.BaseType()
		if base != nil {
			if baseBuiltin, ok := base.(*types.BuiltinType); ok {
				baseCompiled, err := c.compileType(baseBuiltin.Name(), baseBuiltin)
				if err != nil {
					return nil, err
				}
				compiled.BaseType = baseCompiled
				compiled.DerivationMethod = types.DerivationRestriction
				compiled.DerivationChain = append([]*grammar.CompiledType{compiled}, baseCompiled.DerivationChain...)
			} else {
				compiled.DerivationChain = []*grammar.CompiledType{compiled}
			}
		} else {
			compiled.DerivationChain = []*grammar.CompiledType{compiled}
		}
		// check if this is the NOTATION built-in type
		compiled.IsNotationType = t.Name().Local == string(types.TypeNameNOTATION)
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

func (c *Compiler) compileSimpleType(compiled *grammar.CompiledType, st *types.SimpleType) error {
	resolvedBase := st.ResolvedBase
	if resolvedBase == nil && st.Restriction != nil && !st.Restriction.Base.IsZero() {
		if bt := types.GetBuiltinNS(st.Restriction.Base.Namespace, st.Restriction.Base.Local); bt != nil {
			resolvedBase = bt
		} else if base, ok := c.schema.TypeDefs[st.Restriction.Base]; ok {
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
	if st.ItemType != nil {
		itemCompiled, err := c.compileType(st.ItemType.Name(), st.ItemType)
		if err != nil {
			return err
		}
		compiled.ItemType = itemCompiled
		compiled.DerivationMethod = types.DerivationList
	}
	if compiled.ItemType == nil && st.Variety() == types.ListVariety && compiled.BaseType != nil {
		compiled.ItemType = compiled.BaseType.ItemType
	}

	// compile member types (for union)
	if len(st.MemberTypes) > 0 {
		compiled.MemberTypes = make([]*grammar.CompiledType, len(st.MemberTypes))
		for i, member := range st.MemberTypes {
			memberCompiled, err := c.compileType(member.Name(), member)
			if err != nil {
				return err
			}
			compiled.MemberTypes[i] = memberCompiled
		}
		compiled.DerivationMethod = types.DerivationUnion
	}

	compiled.Facets = c.collectFacets(st)

	return nil
}

func (c *Compiler) compileComplexType(compiled *grammar.CompiledType, ct *types.ComplexType) error {
	compiled.Abstract = ct.Abstract
	compiled.Mixed = ct.Mixed()
	compiled.Final = ct.Final
	compiled.Block = ct.Block
	compiled.DerivationMethod = ct.DerivationMethod

	resolvedBase := ct.ResolvedBase
	if resolvedBase == nil {
		resolvedBase = types.GetBuiltin(types.TypeNameAnyType)
	}
	if resolvedBase != nil {
		baseCompiled, err := c.compileType(resolvedBase.Name(), resolvedBase)
		if err != nil {
			return err
		}
		compiled.BaseType = baseCompiled
		if ct.ResolvedBase == nil && compiled.DerivationMethod == 0 {
			compiled.DerivationMethod = types.DerivationRestriction
		}
		compiled.DerivationChain = append([]*grammar.CompiledType{compiled}, baseCompiled.DerivationChain...)
	} else {
		compiled.DerivationChain = []*grammar.CompiledType{compiled}
	}

	// pre-merge all attributes from derivation chain
	compiled.AllAttributes = c.mergeAttributes(ct, compiled.DerivationChain)

	compiled.AnyAttribute = c.mergeAnyAttribute(compiled.DerivationChain)

	if ct.Content() != nil {
		compiled.ContentModel = c.compileContentModel(ct)

		// for complexContent extension, prepend base type's content model
		if cc, ok := ct.Content().(*types.ComplexContent); ok {
			if cc.Extension != nil && compiled.BaseType != nil &&
				compiled.BaseType.ContentModel != nil &&
				!compiled.BaseType.ContentModel.Empty {
				// combine base content + extension content
				baseParticles := compiled.BaseType.ContentModel.Particles
				if len(baseParticles) > 0 {
					if compiled.ContentModel == nil || compiled.ContentModel.Empty {
						// extension with no new content - use base content model
						compiled.ContentModel = &grammar.CompiledContentModel{
							Kind:      compiled.BaseType.ContentModel.Kind,
							Particles: baseParticles,
							Mixed:     compiled.Mixed,
						}
					} else {
						// combine: base particles + extension particles
						combined := make([]*grammar.CompiledParticle, 0, len(baseParticles)+len(compiled.ContentModel.Particles))
						combined = append(combined, baseParticles...)
						combined = append(combined, compiled.ContentModel.Particles...)
						compiled.ContentModel.Particles = combined
						compiled.ContentModel.Kind = types.Sequence
					}
				}
			}
		}
	}

	if compiled.ContentModel != nil {
		c.populateContentModelCaches(compiled.ContentModel)
	}

	// for simpleContent, set up text content validation type
	if sc, ok := ct.Content().(*types.SimpleContent); ok {
		c.setupSimpleContentType(compiled, sc)
	}

	return nil
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
				if facet, ok := f.(facets.Facet); ok {
					compiled.Facets = append(compiled.Facets, facet)
				}
			}
		}
	}
}

func (c *Compiler) findPrimitiveType(ct *grammar.CompiledType) *grammar.CompiledType {
	for _, t := range ct.DerivationChain {
		if t.Kind == grammar.TypeKindBuiltin {
			return t
		}
	}
	return nil
}

func (c *Compiler) collectFacets(st *types.SimpleType) []facets.Facet {
	var result []facets.Facet

	// collect facets from base type first (inherited facets)
	// per XSD spec, patterns from different derivation steps are ANDed together,
	// so inherited patterns become separate entries in the result.
	if st.ResolvedBase != nil {
		if baseST, ok := st.ResolvedBase.(*types.SimpleType); ok {
			result = append(result, c.collectFacets(baseST)...)
		}
	}

	// collect facets from this type's restriction
	// per XSD spec, patterns from the SAME derivation step are ORed together.
	// we group all pattern facets from this step into a PatternSet.
	if st.Restriction != nil {
		var stepPatterns []*facets.Pattern

		for _, f := range st.Restriction.Facets {
			if facet, ok := f.(facets.Facet); ok {
				// check if this is a pattern facet
				if patternFacet, ok := facet.(*facets.Pattern); ok {
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
			} else if df, ok := f.(*facets.DeferredFacet); ok {
				// convert deferred facets to real facets now that base type is resolved
				realFacet := c.constructDeferredFacet(df, st)
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
			result = append(result, &facets.PatternSet{Patterns: stepPatterns})
		}
	}
	return result
}

// constructDeferredFacet converts a DeferredFacet to a real Facet using the resolved base type.
func (c *Compiler) constructDeferredFacet(df *facets.DeferredFacet, st *types.SimpleType) facets.Facet {
	baseType := st.ResolvedBase
	if baseType == nil {
		return nil
	}

	var facet facets.Facet
	var err error

	switch df.FacetName {
	case "minInclusive":
		facet, err = facets.NewMinInclusive(df.FacetValue, baseType)
	case "maxInclusive":
		facet, err = facets.NewMaxInclusive(df.FacetValue, baseType)
	case "minExclusive":
		facet, err = facets.NewMinExclusive(df.FacetValue, baseType)
	case "maxExclusive":
		facet, err = facets.NewMaxExclusive(df.FacetValue, baseType)
	}

	if err != nil {
		// log or handle error - facet construction failed even with resolved type
		// this could happen for genuinely invalid schemas
		return nil
	}

	return facet
}
