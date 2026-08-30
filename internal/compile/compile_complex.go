package compile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type complexTypeScope uint8

const (
	complexTypeScopeInvalid complexTypeScope = iota
	complexTypeScopeGlobal
	complexTypeScopeAnonymous
)

func validateComplexTypeScope(scope complexTypeScope) error {
	switch scope {
	case complexTypeScopeGlobal, complexTypeScopeAnonymous:
		return nil
	case complexTypeScopeInvalid:
		return xsderrors.InternalInvariant("invalid complex type compilation scope")
	default:
		return xsderrors.InternalInvariant("unknown complex type compilation scope")
	}
}

func (c *compiler) compileComplexByQName(q runtime.QName) (runtime.ComplexTypeID, error) {
	label := c.rt.formatName(q)
	if err := c.rejectComplexTypeCycle(q, label); err != nil {
		return runtime.NoComplexType, err
	}
	raw, exists := c.complexRaw[q]
	var source *rawNode
	if exists {
		source = raw.node
	}
	if err := c.spendComponentDependency(source); err != nil {
		return runtime.NoComplexType, err
	}
	if id, ok := c.complexDone[q]; ok {
		return id, nil
	}
	if !exists {
		return runtime.NoComplexType, SchemaComponentMissingError(SchemaComponentComplexType, label)
	}
	leave, err := c.enterComponent(raw.node)
	if err != nil {
		return runtime.NoComplexType, err
	}
	defer leave()
	c.compilingComplex[q] = true
	defer delete(c.compilingComplex, q)
	id, err := c.registerGlobalComplexType(q, runtime.ComplexType{Name: q, Content: runtime.NoContentModel, Attrs: runtime.NoAttributeUseSet, TextType: runtime.NoSimpleType, Base: runtime.ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.complexDone[q] = id
	ct, err := c.compileComplexType(raw.node, raw.ctx, q, complexTypeScopeGlobal)
	if err != nil {
		return runtime.NoComplexType, err
	}
	if err := c.completeGlobalComplexType(raw, q, id, &ct); err != nil {
		return runtime.NoComplexType, err
	}
	return id, nil
}

func (c *compiler) rejectComplexTypeCycle(q runtime.QName, label string) error {
	if !c.compilingComplex[q] {
		return nil
	}
	err := SchemaComponentCycleError(SchemaComponentComplexType, label)
	if raw, ok := c.complexRaw[q]; ok {
		return withSchemaCompileLocation(raw.node, err)
	}
	return err
}

func (c *compiler) completeGlobalComplexType(raw rawComponent, q runtime.QName, id runtime.ComplexTypeID, ct *runtime.ComplexType) error {
	block, err := complexBlockMaskWithDefault(raw.node, raw.ctx.blockDefault)
	if err != nil {
		return err
	}
	final, err := derivationMaskWithDefaultChecked(raw.node, raw.ctx.finalDefault, complexTypeFinalDerivation())
	if err != nil {
		return err
	}
	ct.Name = q
	ct.Block = block
	ct.Final = final
	c.completeComplexType(id, *ct)
	return nil
}

func (c *compiler) compileAnonymousComplex(n *rawNode, ctx *schemaContext) (runtime.ComplexTypeID, error) {
	if err := c.spendComponentDependency(n); err != nil {
		return runtime.NoComplexType, err
	}
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
	leave, err := c.enterComponent(n)
	if err != nil {
		return runtime.NoComplexType, err
	}
	defer leave()
	ct, err := c.compileComplexType(n, ctx, q, complexTypeScopeAnonymous)
	if err != nil {
		return runtime.NoComplexType, err
	}
	final, err := derivationMaskWithDefaultChecked(n, ctx.finalDefault, complexTypeFinalDerivation())
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
		return c.shouldDeferComplexContent(cc, ctx)
	}
	if sc := n.firstXS(vocab.XSDElemSimpleContent); sc != nil {
		return c.shouldDeferSimpleContent(sc, ctx)
	}
	return false, nil
}

func (c *compiler) shouldDeferComplexContent(n *rawNode, ctx *schemaContext) (bool, error) {
	source, err := checkComplexContentSyntax(n)
	if err != nil {
		return false, err
	}
	base, err := c.contentDerivationBaseQName(vocab.XSDElemComplexContent, source.kind, source.node, ctx)
	if err != nil {
		return false, err
	}
	return c.compilingComplex[base], nil
}

func (c *compiler) shouldDeferSimpleContent(n *rawNode, ctx *schemaContext) (bool, error) {
	source, err := checkSimpleContentSyntax(n)
	if err != nil {
		return false, err
	}
	base, err := c.contentDerivationBaseQName(vocab.XSDElemSimpleContent, source.kind, source.node, ctx)
	if err != nil {
		return false, err
	}
	return c.compilingComplex[base], nil
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

func schemaComplexContentKind(n *rawNode, defaultKind runtime.ContentKind) (runtime.ContentKind, error) {
	var defaultMixed bool
	switch defaultKind {
	case runtime.ContentElementOnly:
	case runtime.ContentMixed:
		defaultMixed = true
	case runtime.ContentSimple, runtime.ContentSimpleMixed:
		return runtime.ContentElementOnly, xsderrors.InternalInvariant("complex content inherited a simple content kind")
	default:
		return runtime.ContentElementOnly, xsderrors.InternalInvariant("complex content inherited an unknown content kind")
	}
	mixed, err := schemaBoolAttrDefault(n, vocab.XSDAttrMixed, defaultMixed)
	if err != nil {
		return runtime.ContentElementOnly, err
	}
	if mixed {
		return runtime.ContentMixed, nil
	}
	return runtime.ContentElementOnly, nil
}

func (c *compiler) compileComplexType(n *rawNode, ctx *schemaContext, name runtime.QName, scope complexTypeScope) (runtime.ComplexType, error) {
	if err := validateComplexTypeScope(scope); err != nil {
		return runtime.ComplexType{}, err
	}
	if err := checkComplexTypeChildren(n); err != nil {
		return runtime.ComplexType{}, err
	}
	ct, err := c.newComplexType(n, ctx, name)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	if complexContent := n.firstXS(vocab.XSDElemComplexContent); complexContent != nil {
		return c.compileComplexContent(complexContent, ctx, ct, scope)
	}
	if simpleContent := n.firstXS(vocab.XSDElemSimpleContent); simpleContent != nil {
		return c.compileSimpleContent(simpleContent, ctx, ct, scope)
	}
	return c.compileDirectComplexType(n, ctx, ct)
}

func (c *compiler) newComplexType(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.ComplexType, error) {
	contentKind, err := schemaComplexContentKind(n, runtime.ContentElementOnly)
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
		ContentKind: contentKind,
		Abstract:    abstract,
		Base:        runtime.ComplexRef(c.rt.builtinIDs().AnyType),
		Derivation:  runtime.DerivationKindRestriction,
		Block:       block,
	}
	return ct, nil
}

func (c *compiler) compileDirectComplexType(n *rawNode, ctx *schemaContext, ct runtime.ComplexType) (runtime.ComplexType, error) {
	content, err := c.compileDirectComplexModel(n, ctx)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Content = content
	if ct.Content == runtime.NoContentModel {
		ct.Content, err = c.addModel(runtime.ContentModel{Kind: runtime.ModelEmpty, Mixed: ct.Mixed()})
		if err != nil {
			return runtime.ComplexType{}, err
		}
	}
	attrs, err := c.compileAttributeUses(n, ctx, nil, runtime.NoWildcard, AttributeMergeDirect)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Attrs = attrs
	return ct, nil
}

func (c *compiler) compileDirectComplexModel(n *rawNode, ctx *schemaContext) (runtime.ContentModelID, error) {
	content := runtime.NoContentModel
	for _, child := range n.Children {
		if child.Name.Space != vocab.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
			continue
		}
		model, present, err := c.compileDirectComplexModelChild(child, ctx)
		if err != nil {
			return runtime.NoContentModel, err
		}
		if present {
			content = model
		}
	}
	return content, nil
}

func (c *compiler) compileDirectComplexModelChild(child *rawNode, ctx *schemaContext) (runtime.ContentModelID, bool, error) {
	switch child.Name.Local {
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		if err := validateModelOccurrence(child, c.limits); err != nil {
			return runtime.NoContentModel, false, err
		}
		model, err := c.compileModel(child, ctx)
		return model, true, err
	default:
		return runtime.NoContentModel, false, nil
	}
}

func (c *compiler) compileComplexContent(n *rawNode, ctx *schemaContext, ct runtime.ComplexType, scope complexTypeScope) (runtime.ComplexType, error) {
	source, err := checkComplexContentSyntax(n)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	contentKind, err := schemaComplexContentKind(n, ct.ContentKind)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	return c.compileComplexContentDerivation(source.node, source.kind, ctx, ct, contentKind, scope)
}

func (c *compiler) compileComplexContentDerivation(child *rawNode, kind ContentDerivationKind, ctx *schemaContext, ct runtime.ComplexType, contentKind runtime.ContentKind, scope complexTypeScope) (runtime.ComplexType, error) {
	baseID, base, err := c.complexContentBase(child, kind, ctx, scope)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	extension := kind == ContentDerivationExtension
	if err := c.validateComplexContentMixedDerivationBase(child, base, kind, contentKind); err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Base = runtime.ComplexRef(baseID)
	if extension {
		if err := checkComplexContentExtensionChildren(child); err != nil {
			return runtime.ComplexType{}, err
		}
		return c.compileComplexContentExtension(child, ctx, ct, baseID, base, contentKind)
	}
	if err := checkComplexContentRestrictionChildren(child); err != nil {
		return runtime.ComplexType{}, err
	}
	return c.compileComplexContentRestriction(child, ctx, ct, base, contentKind)
}

func (c *compiler) complexContentBase(child *rawNode, kind ContentDerivationKind, ctx *schemaContext, scope complexTypeScope) (runtime.ComplexTypeID, runtime.ComplexType, error) {
	baseQName, err := c.contentDerivationBaseQName(vocab.XSDElemComplexContent, kind, child, ctx)
	if err != nil {
		return runtime.NoComplexType, runtime.ComplexType{}, err
	}
	if c.compilingComplex[baseQName] && scope == complexTypeScopeGlobal {
		cycleErr := SchemaComponentCycleError(SchemaComponentComplexType, c.rt.formatName(baseQName))
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
	base := ContentDerivationBase{Container: container, Derivation: kind.String(), Lexical: baseLex, Present: ok}
	if err := checkContentDerivationBase(child, base); err != nil {
		return runtime.QName{}, err
	}
	return c.resolveQNameChecked(child, ctx, base.Lexical)
}

func (c *compiler) compileComplexContentExtension(child *rawNode, ctx *schemaContext, ct runtime.ComplexType, baseID runtime.ComplexTypeID, base runtime.ComplexType, contentKind runtime.ContentKind) (runtime.ComplexType, error) {
	if err := CheckComplexTypeFinalAllows(base.Final, runtime.DerivationExtension, ComplexTypeFinalBaseExtension); err != nil {
		return runtime.ComplexType{}, withSchemaCompileLocation(child, err)
	}
	if base.SimpleContent() {
		return c.compileSimpleValueComplexExtension(child, ctx, ct, base, contentKind)
	}
	ct.Derivation = runtime.DerivationKindExtension
	ct.ExplicitDerivation = true
	ct.Content = base.Content
	ct.Attrs = base.Attrs
	if modelNode := firstModelChild(child); modelNode != nil {
		content, err := c.compileComplexExtensionModel(modelNode, ctx, baseID, base, contentKind)
		if err != nil {
			return runtime.ComplexType{}, err
		}
		ct.Content = content
	}
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, AttributeMergeExtension)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	ct.Attrs = attrs
	ct.ContentKind = contentKind
	if base.Mixed() {
		ct.ContentKind = runtime.ContentMixed
	}
	return ct, nil
}

func (c *compiler) compileSimpleValueComplexExtension(child *rawNode, ctx *schemaContext, ct, base runtime.ComplexType, contentKind runtime.ContentKind) (runtime.ComplexType, error) {
	if err := ValidateComplexExtensionContentAdmission(ComplexExtensionContentAdmission{
		BaseSimpleContent: true,
		HasModelChild:     firstModelChild(child) != nil,
	}); err != nil {
		return runtime.ComplexType{}, withSchemaCompileLocation(child, err)
	}
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcard(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, AttributeMergeExtension)
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
	ct.ContentKind = runtime.ContentSimple
	if contentKind == runtime.ContentMixed {
		ct.ContentKind = runtime.ContentSimpleMixed
	}
	ct.ExplicitDerivation = true
	return ct, nil
}

func (c *compiler) compileComplexExtensionModel(modelNode *rawNode, ctx *schemaContext, baseID runtime.ComplexTypeID, base runtime.ComplexType, contentKind runtime.ContentKind) (runtime.ContentModelID, error) {
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return runtime.NoContentModel, err
	}
	ext, err := c.compileModel(modelNode, ctx)
	if err != nil {
		return runtime.NoContentModel, err
	}
	if err := c.validateComplexExtensionModelAdmission(baseID, base, ext, contentKind); err != nil {
		return runtime.NoContentModel, withSchemaCompileLocation(modelNode, err)
	}
	addAtModelNode := func(model runtime.ContentModel) (runtime.ContentModelID, error) {
		return c.addModelAt(model, modelNode)
	}
	return ExtendSequenceModel(&c.rt, addAtModelNode, base.Content, ext)
}

func (c *compiler) validateComplexContentMixedDerivationBase(child *rawNode, base runtime.ComplexType, derivation ContentDerivationKind, content runtime.ContentKind) error {
	if err := CheckComplexContentMixedDerivationBase(&c.rt, base, derivation, content); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (c *compiler) compileComplexContentRestriction(child *rawNode, ctx *schemaContext, ct, base runtime.ComplexType, contentKind runtime.ContentKind) (runtime.ComplexType, error) {
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
	ct.ContentKind = contentKind
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

func (c *compiler) compileSimpleContent(n *rawNode, ctx *schemaContext, ct runtime.ComplexType, scope complexTypeScope) (runtime.ComplexType, error) {
	source, err := c.resolveSimpleContentSource(n, ctx)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	var textType runtime.SimpleTypeID
	if c.simpleTypeQNameKnown(source.base) {
		ct, textType, err = c.compileSimpleContentSimpleBase(source.child, source.kind, source.base, ct)
	} else {
		ct, textType, err = c.compileSimpleContentComplexBase(source.child, source.kind, source.base, ct, scope)
	}
	if err != nil {
		return runtime.ComplexType{}, err
	}
	derivation, err := c.compileSimpleContentDerivation(source, ctx, textType)
	if err != nil {
		return runtime.ComplexType{}, err
	}
	textType = derivation.textType
	inheritedUses, inheritedWildcard := c.rt.attributeUsesAndWildcard(ct.Attrs)
	attrs, err := c.compileAttributeUses(source.child, ctx, inheritedUses, inheritedWildcard, derivation.mergeMode)
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
	if ct.Mixed() {
		ct.ContentKind = runtime.ContentSimpleMixed
	} else {
		ct.ContentKind = runtime.ContentSimple
	}
	ct.Derivation = derivation.kind
	ct.ExplicitDerivation = true
	return ct, nil
}

type simpleContentSource struct {
	child *rawNode
	base  runtime.QName
	kind  ContentDerivationKind
}

type simpleContentDerivation struct {
	textType  runtime.SimpleTypeID
	mergeMode AttributeMergeMode
	kind      runtime.DerivationKind
}

func (c *compiler) resolveSimpleContentSource(n *rawNode, ctx *schemaContext) (simpleContentSource, error) {
	syntax, err := checkSimpleContentSyntax(n)
	if err != nil {
		return simpleContentSource{}, err
	}
	baseLexical, ok := syntax.node.attr(vocab.XSDAttrBase)
	baseAttribute := ContentDerivationBase{
		Container:  vocab.XSDElemSimpleContent,
		Derivation: syntax.kind.String(),
		Lexical:    baseLexical,
		Present:    ok,
	}
	if baseErr := checkContentDerivationBase(syntax.node, baseAttribute); baseErr != nil {
		return simpleContentSource{}, baseErr
	}
	base, err := c.resolveQNameChecked(syntax.node, ctx, baseAttribute.Lexical)
	if err != nil {
		return simpleContentSource{}, err
	}
	return simpleContentSource{child: syntax.node, base: base, kind: syntax.kind}, nil
}

func (c *compiler) compileSimpleContentDerivation(source simpleContentSource, ctx *schemaContext, textType runtime.SimpleTypeID) (simpleContentDerivation, error) {
	if source.kind != ContentDerivationRestriction {
		if err := checkSimpleContentExtensionChildren(source.child); err != nil {
			return simpleContentDerivation{}, err
		}
		return simpleContentDerivation{
			textType:  textType,
			mergeMode: AttributeMergeExtension,
			kind:      runtime.DerivationKindExtension,
		}, nil
	}
	if err := checkSimpleContentRestrictionChildren(source.child); err != nil {
		return simpleContentDerivation{}, err
	}
	restricted, err := c.compileSimpleContentRestrictionType(source.child, ctx, textType)
	return simpleContentDerivation{
		textType:  restricted,
		mergeMode: AttributeMergeRestriction,
		kind:      runtime.DerivationKindRestriction,
	}, err
}

func (c *compiler) compileSimpleContentSimpleBase(child *rawNode, kind ContentDerivationKind, baseQName runtime.QName, ct runtime.ComplexType) (runtime.ComplexType, runtime.SimpleTypeID, error) {
	if err := CheckSimpleContentSimpleBase(kind); err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	simpleID, err := c.compileSimpleByQName(baseQName)
	if err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if err := CheckSimpleBaseComplexExtensionFinalAllows(c.rt.simpleTypeFinalMask(simpleID)); err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	ct.Base = runtime.SimpleRef(simpleID)
	return ct, simpleID, nil
}

func (c *compiler) compileSimpleContentComplexBase(child *rawNode, kind ContentDerivationKind, baseQName runtime.QName, ct runtime.ComplexType, scope complexTypeScope) (runtime.ComplexType, runtime.SimpleTypeID, error) {
	if c.compilingComplex[baseQName] && scope == complexTypeScopeGlobal {
		err := SchemaComponentCycleError(SchemaComponentComplexType, c.rt.formatName(baseQName))
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if !c.complexTypeQNameKnown(baseQName) {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, SimpleContentComplexBaseMissingError())
	}
	baseComplex, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return runtime.ComplexType{}, runtime.NoSimpleType, withSchemaCompileLocation(child, err)
	}
	base := c.rt.complexType(baseComplex)
	if err := CheckSimpleContentDerivationBase(c.contentAnalysis, base, kind); err != nil {
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
	if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinalMask(baseID), runtime.DerivationRestriction, SimpleTypeFinalBaseRestriction); err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$simple%d", c.rt.SimpleTypeCount()))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	st := c.rt.derivedSimpleType(baseID, q)
	if c.simpleTypeUnavailable[baseID] {
		err = c.validateUnavailableFacetChildren(facetChildren, &st, baseID, facetChildModeExplicitList)
	} else {
		err = c.compileFacetList(facetChildren, &st, baseID, baseID)
	}
	if err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	if st.Variety == runtime.SimpleVarietyUnion {
		if err := c.chargeSimpleUnionMemberEntries(facetChildren[0], len(st.Union)); err != nil {
			return runtime.NoSimpleType, err
		}
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
