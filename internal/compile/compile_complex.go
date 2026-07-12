package compile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) compileComplexByQName(q runtime.QName) (runtime.ComplexTypeID, error) {
	if id, ok := c.complexDone[q]; ok {
		return id, nil
	}
	label := c.rt.formatName(q)
	if c.compilingComplex[q] {
		err := CheckSchemaComponentCycle(SchemaComponentComplexType, true, label)
		if raw, ok := c.complexRaw[q]; ok {
			return runtime.NoComplexType, withSchemaCompileLocation(raw.node, err)
		}
		return runtime.NoComplexType, err
	}
	raw, ok := c.complexRaw[q]
	if err := CheckSchemaComponentExists(SchemaComponentComplexType, ok, label); err != nil {
		return runtime.NoComplexType, err
	}
	c.compilingComplex[q] = true
	defer delete(c.compilingComplex, q)
	id, err := c.registerGlobalComplexType(q, runtime.ComplexType{Name: q, Content: runtime.NoContentModel, Attrs: runtime.NoAttributeUseSet, TextType: runtime.NoSimpleType, Base: runtime.ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.complexDone[q] = id
	ct, err := c.compileComplexType(raw.node, raw.ctx, q, false)
	if err != nil {
		return runtime.NoComplexType, err
	}
	block, err := complexBlockMaskWithDefault(raw.node, raw.ctx.blockDefault)
	if err != nil {
		return runtime.NoComplexType, err
	}
	final, err := derivationMaskWithDefaultChecked(raw.node, raw.ctx.finalDefault, ComplexTypeFinalDerivation)
	if err != nil {
		return runtime.NoComplexType, err
	}
	ct.Name = q
	ct.Block = block
	ct.Final = final
	c.completeComplexType(id, ct)
	return id, nil
}

func (c *compiler) compileAnonymousComplex(n *rawNode, ctx *schemaContext) (runtime.ComplexTypeID, error) {
	if err := checkLocalComplexTypeAttributes(n); err != nil {
		return runtime.NoComplexType, err
	}
	deferCompletion, err := c.shouldDeferAnonymousComplex(n, ctx)
	if err != nil {
		return runtime.NoComplexType, err
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$complex%d", c.rt.ComplexTypeCount()))
	if err != nil {
		return runtime.NoComplexType, err
	}
	id, err := c.addComplexType(runtime.ComplexType{Name: q, Content: runtime.NoContentModel, Attrs: runtime.NoAttributeUseSet, TextType: runtime.NoSimpleType, Base: runtime.ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return runtime.NoComplexType, err
	}
	if deferCompletion {
		c.deferredAnonymousComplex = append(c.deferredAnonymousComplex, deferredAnonymousComplex{
			node: n,
			ctx:  ctx,
			name: q,
			id:   id,
		})
		return id, nil
	}
	return c.completeAnonymousComplex(id, q, n, ctx)
}

func (c *compiler) completeAnonymousComplex(id runtime.ComplexTypeID, q runtime.QName, n *rawNode, ctx *schemaContext) (runtime.ComplexTypeID, error) {
	ct, err := c.compileComplexType(n, ctx, q, true)
	if err != nil {
		return runtime.NoComplexType, err
	}
	final, err := derivationMaskWithDefaultChecked(n, ctx.finalDefault, ComplexTypeFinalDerivation)
	if err != nil {
		return runtime.NoComplexType, err
	}
	ct.Name = q
	ct.Final = final
	c.completeComplexType(id, ct)
	return id, nil
}

func (c *compiler) drainDeferredAnonymousComplex() error {
	for len(c.deferredAnonymousComplex) != 0 {
		pending := c.deferredAnonymousComplex
		c.deferredAnonymousComplex = nil
		for _, item := range pending {
			if _, err := c.completeAnonymousComplex(item.id, item.name, item.node, item.ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *compiler) shouldDeferAnonymousComplex(n *rawNode, ctx *schemaContext) (bool, error) {
	if err := checkComplexTypeChildren(n); err != nil {
		return false, err
	}
	if cc := n.firstXS(vocab.XSDElemComplexContent); cc != nil {
		source, err := checkComplexContentSyntax(cc)
		if err != nil {
			return false, err
		}
		base, err := c.contentDerivationBaseQName(vocab.XSDElemComplexContent, source.kind, source.node, ctx)
		if err != nil {
			return false, err
		}
		return c.compilingComplex[base], nil
	}
	if sc := n.firstXS(vocab.XSDElemSimpleContent); sc != nil {
		source, err := checkSimpleContentSyntax(sc)
		if err != nil {
			return false, err
		}
		base, err := c.contentDerivationBaseQName(vocab.XSDElemSimpleContent, source.kind, source.node, ctx)
		if err != nil {
			return false, err
		}
		return c.compilingComplex[base], nil
	}
	return false, nil
}

func schemaBoolAttr(n *rawNode, name string) (bool, error) {
	return schemaBoolAttrDefault(n, name, false)
}

func schemaBoolAttrDefault(n *rawNode, name string, def bool) (bool, error) {
	v, ok := n.attr(name)
	parsed, err := ParseBooleanAttr(BooleanAttr{
		Name:     name,
		Value:    v,
		HasValue: ok,
		Default:  def,
	})
	if err != nil {
		return false, withSchemaCompileLocation(n, err)
	}
	return parsed, nil
}

func (c *compiler) compileComplexType(n *rawNode, ctx *schemaContext, name runtime.QName, anonymous bool) (runtime.ComplexType, error) {
	if err := checkComplexTypeChildren(n); err != nil {
		return runtime.ComplexType{}, err
	}
	mixed, err := schemaBoolAttr(n, vocab.XSDAttrMixed)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	abstract, err := schemaBoolAttr(n, vocab.XSDAttrAbstract)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	block, err := complexBlockMaskWithDefault(n, ctx.blockDefault)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct := runtime.ComplexType{
		Name:        name,
		Content:     runtime.NoContentModel,
		Attrs:       runtime.NoAttributeUseSet,
		TextType:    runtime.NoSimpleType,
		ContentKind: runtime.ElementContentKind(mixed),
		Abstract:    abstract,
		Base:        runtime.ComplexRef(c.rt.builtinIDs().AnyType),
		Derivation:  runtime.DerivationKindRestriction,
		Block:       block,
	}
	if cc := n.firstXS(vocab.XSDElemComplexContent); cc != nil {
		return c.compileComplexContent(cc, ctx, ct, anonymous)
	}
	if sc := n.firstXS(vocab.XSDElemSimpleContent); sc != nil {
		return c.compileSimpleContent(sc, ctx, ct, anonymous)
	}
	for _, child := range n.Children {
		if child.Name.Space != runtime.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
			continue
		}
		switch child.Name.Local {
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
			if occurrenceErr := validateModelOccurrence(child, c.limits); occurrenceErr != nil {
				return runtime.ComplexType{}, occurrenceErr
			}
			modelID, modelErr := c.compileModel(child, ctx)
			if modelErr != nil {
				return runtime.ComplexType{}, modelErr
			}
			ct.Content = modelID
		}
	}
	if ct.Content == runtime.NoContentModel {
		ct.Content, err = c.addModel(runtime.ContentModel{Kind: runtime.ModelEmpty, Mixed: mixed})
		if err != nil {
			return runtime.ComplexType{}, err
		}
	}
	attrs, err := c.compileAttributeUses(n, ctx, nil, runtime.NoWildcard, AttributeMergeNormal)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Attrs = attrs
	return ct, nil
}

func (c *compiler) compileComplexContent(n *rawNode, ctx *schemaContext, ct runtime.ComplexType, anonymous bool) (runtime.ComplexType, error) {
	source, err := checkComplexContentSyntax(n)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	mixed, err := schemaBoolAttrDefault(n, vocab.XSDAttrMixed, ct.Mixed())
	if err != nil {
		return runtime.ComplexType{}, err
	}
	return c.compileComplexContentDerivation(source.node, source.kind, ctx, ct, mixed, anonymous)
}

func (c *compiler) compileComplexContentDerivation(child *rawNode, kind ContentDerivationKind, ctx *schemaContext, ct runtime.ComplexType, mixed, anonymous bool) (runtime.ComplexType, error) {
	baseID, base, err := c.complexContentBase(child, kind, ctx, anonymous)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	extension := kind == ContentDerivationExtension
	if err := c.validateComplexContentMixedDerivationBase(child, base, extension, mixed); err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Base = runtime.ComplexRef(baseID)
	if extension {
		if err := checkComplexContentExtensionChildren(child); err != nil {
			return runtime.ComplexType{}, err
		}
		return c.compileComplexContentExtension(child, ctx, ct, baseID, base, mixed)
	}
	if err := checkComplexContentRestrictionChildren(child); err != nil {
		return runtime.ComplexType{}, err
	}
	return c.compileComplexContentRestriction(child, ctx, ct, base, mixed)
}

func (c *compiler) complexContentBase(child *rawNode, kind ContentDerivationKind, ctx *schemaContext, anonymous bool) (runtime.ComplexTypeID, runtime.ComplexType, error) {
	baseQName, err := c.contentDerivationBaseQName(vocab.XSDElemComplexContent, kind, child, ctx)
	if err != nil {
		return runtime.NoComplexType, runtime.ComplexType{}, err
	}
	if c.compilingComplex[baseQName] && !anonymous {
		cycleErr := CheckSchemaComponentCycle(SchemaComponentComplexType, true, c.rt.formatName(baseQName))
		return runtime.NoComplexType, runtime.ComplexType{}, withSchemaCompileLocation(child, cycleErr)
	}
	baseID, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return runtime.NoComplexType, runtime.ComplexType{}, withSchemaCompileLocation(child, err)
	}
	return baseID, c.rt.complexType(baseID), nil
}

func (c *compiler) contentDerivationBaseQName(container string, kind ContentDerivationKind, child *rawNode, ctx *schemaContext) (runtime.QName, error) {
	baseLex, ok := child.attr(vocab.XSDAttrBase)
	if err := checkContentDerivationBase(container, kind, child, ok); err != nil {
		return runtime.QName{}, err
	}
	return c.resolveQNameChecked(child, ctx, baseLex)
}

func (c *compiler) compileComplexContentExtension(child *rawNode, ctx *schemaContext, ct runtime.ComplexType, baseID runtime.ComplexTypeID, base runtime.ComplexType, mixed bool) (runtime.ComplexType, error) {
	if err := CheckComplexTypeFinalAllows(base.Final, runtime.DerivationExtension, ComplexTypeFinalBaseExtension); err != nil {
		return runtime.ComplexType{}, withSchemaCompileLocation(child, err)
	}
	if base.SimpleContent() {
		return c.compileSimpleValueComplexExtension(child, ctx, ct, base, mixed)
	}
	ct.Derivation = runtime.DerivationKindExtension
	ct.ExplicitDerivation = true
	ct.Content = base.Content
	ct.Attrs = base.Attrs
	if modelNode := firstModelChild(child); modelNode != nil {
		content, err := c.compileComplexExtensionModel(modelNode, ctx, baseID, base, mixed)
		if err != nil {
			return runtime.ComplexType{}, err
		}
		ct.Content = content
	}
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, AttributeMergeNormal)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Attrs = attrs
	ct.ContentKind = runtime.ElementContentKind(base.Mixed() || mixed)
	return ct, nil
}

func (c *compiler) compileSimpleValueComplexExtension(child *rawNode, ctx *schemaContext, ct, base runtime.ComplexType, mixed bool) (runtime.ComplexType, error) {
	if err := ValidateComplexExtensionContentAdmission(ComplexExtensionContentAdmission{
		BaseSimpleContent: true,
		HasModelChild:     firstModelChild(child) != nil,
	}); err != nil {
		return runtime.ComplexType{}, withSchemaCompileLocation(child, err)
	}
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, AttributeMergeNormal)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Derivation = runtime.DerivationKindExtension
	ct.Content, err = c.addModel(runtime.ContentModel{Kind: runtime.ModelEmpty})
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Attrs = attrs
	ct.TextType = base.TextType
	ct.ContentKind = runtime.SimpleContentKind(mixed)
	ct.ExplicitDerivation = true
	return ct, nil
}

func (c *compiler) compileComplexExtensionModel(modelNode *rawNode, ctx *schemaContext, baseID runtime.ComplexTypeID, base runtime.ComplexType, mixed bool) (runtime.ContentModelID, error) {
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return runtime.NoContentModel, err
	}
	ext, err := c.compileModel(modelNode, ctx)
	if err != nil {
		return runtime.NoContentModel, err
	}
	if err := c.validateComplexExtensionModelAdmission(baseID, base, ext, mixed); err != nil {
		return runtime.NoContentModel, withSchemaCompileLocation(modelNode, err)
	}
	return ExtendSequenceModel(&c.rt, c.addModel, base.Content, ext)
}

func (c *compiler) validateComplexContentMixedDerivationBase(child *rawNode, base runtime.ComplexType, extension, mixed bool) error {
	if err := CheckComplexContentMixedDerivationBase(&c.rt, base, extension, mixed); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (c *compiler) compileComplexContentRestriction(child *rawNode, ctx *schemaContext, ct, base runtime.ComplexType, mixed bool) (runtime.ComplexType, error) {
	if err := CheckComplexTypeFinalAllows(base.Final, runtime.DerivationRestriction, ComplexTypeFinalBaseRestriction); err != nil {
		return runtime.ComplexType{}, withSchemaCompileLocation(child, err)
	}
	if err := CheckComplexContentRestrictionBase(base); err != nil {
		return runtime.ComplexType{}, withSchemaCompileLocation(child, err)
	}
	ct.Derivation = runtime.DerivationKindRestriction
	ct.ExplicitDerivation = true
	content, err := c.compileComplexRestrictionModel(child, ctx, ct)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Content = content
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, AttributeMergeRestriction)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Attrs = attrs
	ct.ContentKind = runtime.ElementContentKind(mixed)
	return ct, nil
}

func (c *compiler) compileComplexRestrictionModel(child *rawNode, ctx *schemaContext, ct runtime.ComplexType) (runtime.ContentModelID, error) {
	modelNode := firstModelChild(child)
	if modelNode == nil {
		return c.addModel(runtime.ContentModel{Kind: runtime.ModelEmpty, Mixed: ct.Mixed()})
	}
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return runtime.NoContentModel, err
	}
	return c.compileModel(modelNode, ctx)
}

func (c *compiler) compileSimpleContent(n *rawNode, ctx *schemaContext, ct runtime.ComplexType, anonymous bool) (runtime.ComplexType, error) {
	source, err := checkSimpleContentSyntax(n)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	child := source.node
	baseLex, ok := child.attr(vocab.XSDAttrBase)
	if baseErr := checkContentDerivationBase(vocab.XSDElemSimpleContent, source.kind, child, ok); baseErr != nil {
		return runtime.ComplexType{}, baseErr
	}
	baseQName, err := c.resolveQNameChecked(child, ctx, baseLex)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	isRestriction := source.kind == ContentDerivationRestriction
	var textType runtime.SimpleTypeID
	if c.simpleTypeQNameKnown(baseQName) {
		ct, textType, err = c.compileSimpleContentSimpleBase(child, source.kind, baseQName, ct)
	} else {
		ct, textType, err = c.compileSimpleContentComplexBase(child, source.kind, baseQName, ct, anonymous)
	}
	if err != nil {
		return runtime.ComplexType{}, err
	}
	mergeMode := AttributeMergeNormal
	derivation := runtime.DerivationKindExtension
	if isRestriction {
		if validationErr := checkSimpleContentRestrictionChildren(child); validationErr != nil {
			return runtime.ComplexType{}, validationErr
		}
		textType, err = c.compileSimpleContentRestrictionType(child, ctx, textType)
		if err != nil {
			return runtime.ComplexType{}, err
		}
		mergeMode = AttributeMergeRestriction
		derivation = runtime.DerivationKindRestriction
	} else if validationErr := checkSimpleContentExtensionChildren(child); validationErr != nil {
		return runtime.ComplexType{}, validationErr
	}
	inheritedUses, inheritedWildcard := c.rt.attributeUsesAndWildcard(ct.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, inheritedUses, inheritedWildcard, mergeMode)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Attrs = attrs
	ct.Content, err = c.addModel(runtime.ContentModel{Kind: runtime.ModelEmpty})
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.TextType = textType
	// xs:simpleContent has no mixed attribute; ct.Mixed() carries mixed="true"
	// from the enclosing complexType element, which downstream complexContent
	// mixed-derivation checks read.
	ct.ContentKind = runtime.SimpleContentKind(ct.Mixed())
	ct.Derivation = derivation
	ct.ExplicitDerivation = true
	return ct, nil
}

func (c *compiler) compileSimpleContentSimpleBase(child *rawNode, kind ContentDerivationKind, baseQName runtime.QName, ct runtime.ComplexType) (runtime.ComplexType, runtime.SimpleTypeID, error) {
	if err := CheckSimpleContentSimpleBase(kind); err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	simpleID, err := c.compileSimpleByQName(baseQName)
	if err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if err := CheckSimpleBaseComplexExtensionFinalAllows(c.rt.simpleTypeFinal(simpleID)); err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	ct.Base = runtime.SimpleRef(simpleID)
	return ct, simpleID, nil
}

func (c *compiler) compileSimpleContentComplexBase(child *rawNode, kind ContentDerivationKind, baseQName runtime.QName, ct runtime.ComplexType, anonymous bool) (runtime.ComplexType, runtime.SimpleTypeID, error) {
	if c.compilingComplex[baseQName] && !anonymous {
		err := CheckSchemaComponentCycle(SchemaComponentComplexType, true, c.rt.formatName(baseQName))
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if err := CheckSimpleContentComplexBaseExists(c.complexTypeQNameKnown(baseQName)); err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	baseComplex, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	base := c.rt.complexType(baseComplex)
	if err := CheckSimpleContentDerivationBase(&c.rt, base, kind == ContentDerivationRestriction); err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	switch kind {
	case ContentDerivationNone:
		return runtime.ComplexType{}, runtime.NoSimpleType, xsderrors.InternalInvariant("simpleContent complex base derivation missing")
	case ContentDerivationExtension:
		if err := CheckComplexTypeFinalAllows(base.Final, runtime.DerivationExtension, ComplexTypeFinalBaseExtension); err != nil {
			return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
		}
	case ContentDerivationRestriction:
		if err := CheckComplexTypeFinalAllows(base.Final, runtime.DerivationRestriction, ComplexTypeFinalBaseRestriction); err != nil {
			return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
		}
	}
	ct.Base = runtime.ComplexRef(baseComplex)
	ct.Attrs = base.Attrs
	return ct, base.TextType, nil
}

func (c *compiler) compileSimpleContentRestrictionType(child *rawNode, ctx *schemaContext, baseTextType runtime.SimpleTypeID) (runtime.SimpleTypeID, error) {
	textType := baseTextType
	facetChildren := facetChildren(child)
	if stNode := child.firstXS(vocab.XSDElemSimpleType); stNode != nil {
		simpleID, err := c.compileAnonymousSimple(stNode, ctx)
		if err != nil {
			return runtime.NoSimpleType, err
		}
		textType = simpleID
	}
	if err := CheckSimpleContentRestrictionTextTypePresent(textType); err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if len(facetChildren) != 0 {
		simpleID, err := c.compileSimpleContentFacetRestriction(facetChildren, textType)
		if err != nil {
			return runtime.NoSimpleType, err
		}
		textType = simpleID
	}
	if err := CheckSimpleContentRestrictionTextType(&c.rt, textType, baseTextType); err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	return textType, nil
}

func (c *compiler) compileSimpleContentFacetRestriction(facetChildren []*rawNode, baseID runtime.SimpleTypeID) (runtime.SimpleTypeID, error) {
	if err := CheckSimpleRestrictionBase(baseID, c.rt.builtinIDs().AnySimpleType); err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinal(baseID), runtime.DerivationRestriction, SimpleTypeFinalBaseRestriction); err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$simple%d", c.rt.SimpleTypeCount()))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	st := c.rt.derivedSimpleType(baseID, q)
	if err = c.compileFacetList(facetChildren, &st, baseID, baseID); err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	st.Identity = c.rt.DerivedSimpleIdentity(st)
	st.Fast = runtime.DeriveSimpleFastPathForSimpleType(st)
	return c.addSimpleType(st)
}

func facetChildren(n *rawNode) []*rawNode {
	var out []*rawNode
	for _, child := range n.Children {
		if child.Name.Space == vocab.XSDNamespaceURI && IsFacetLocal(child.Name.Local) {
			out = append(out, child)
		}
	}
	return out
}

func firstModelChild(n *rawNode) *rawNode {
	for child := range n.xsdChildren() {
		switch child.Name.Local {
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
			return child
		}
	}
	return nil
}
