package schema

import (
	"errors"
	"fmt"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// complexTypeCompletion is a complex type's reserved -> completing -> complete
// transition. Element type references only reserve; derivation-base
// resolution alone forces completion, so re-entering a completing definition
// is a derivation cycle, never legal recursive element content. Builtin
// anyType starts complete.
type complexTypeCompletion uint8

const (
	complexTypeReserved complexTypeCompletion = iota
	complexTypeCompleting
	complexTypeComplete
)

// Definitions are indexed by complexTypeID, independently of QName registration.
// Element declarations need only an identity; derivation requires a complete base.
type complexTypeDefinition struct {
	source schemaComponent
	state  complexTypeCompletion
}

func (c *compiler) compileComplexByQName(q QName, use *schemaNode) (ComplexTypeID, error) {
	if id, ok := c.rt.GlobalTypes[q].Complex(); ok && c.complexDefinitions[id].state == complexTypeCompleting {
		return noComplexType, c.complexTypeCycleError(id, use)
	}
	id, err := c.reserveNamedComplexType(q)
	if err != nil {
		return noComplexType, err
	}
	if err := c.completeComplexDefinition(id); err != nil {
		return noComplexType, err
	}
	return id, nil
}

func (c *compiler) reserveNamedComplexType(q QName) (ComplexTypeID, error) {
	raw, exists := c.complexComponents[q]
	if err := c.spendComponentDependency(raw.sourceNode()); err != nil {
		return noComplexType, err
	}
	if id, ok := c.rt.GlobalTypes[q].Complex(); ok {
		return id, nil
	}
	if !exists {
		return noComplexType, schemaComponentMissingError(schemaComponentComplexType, c.rt.formatName(q))
	}
	id, err := c.registerGlobalComplexType(q, c.complexTypePlaceholder(q))
	if err != nil {
		return noComplexType, err
	}
	c.complexDefinitions[id].source = raw
	return id, nil
}

func (c *compiler) reserveAnonymousComplexType(ctx *schemaSetDocument, n *schemaNode) (ComplexTypeID, error) {
	if err := c.spendComponentDependency(n); err != nil {
		return noComplexType, err
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$complex%d", len(c.rt.ComplexTypes)))
	if err != nil {
		return noComplexType, err
	}
	id, err := c.addComplexType(c.complexTypePlaceholder(q))
	if err != nil {
		return noComplexType, err
	}
	c.complexDefinitions[id].source = schemaComponent{ctx: ctx, id: n.id}
	return id, nil
}

func (c *compiler) complexTypePlaceholder(q QName) complexTypeMetadata {
	return complexTypeMetadata{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, TextType: NoSimpleType, Base: ComplexRef(c.rt.AnyType)}
}

func (c *compiler) complexTypeCycleError(id ComplexTypeID, use *schemaNode) error {
	err := schemaComponentCycleError(schemaComponentComplexType, c.rt.formatName(c.rt.ComplexTypes[id].Name))
	if use == nil {
		use = c.complexDefinitions[id].source.sourceNode()
	}
	return withSchemaCompileLocation(use, err)
}

func (c *compiler) completeComplexDefinition(id ComplexTypeID) (err error) {
	definition := c.complexDefinitions[id]
	switch definition.state {
	case complexTypeComplete:
		return nil
	case complexTypeCompleting:
		return c.complexTypeCycleError(id, nil)
	case complexTypeReserved:
	default:
		return xsderrors.InternalInvariant("unknown complex type completion state")
	}
	n := definition.source.sourceNode()
	if n == nil {
		return xsderrors.InternalInvariant("complex type definition has no source")
	}
	if err = c.spendComponentDependency(n); err != nil {
		return err
	}
	leave, err := c.enterComponent(n)
	if err != nil {
		return err
	}
	defer leave(&err)
	c.complexDefinitions[id].state = complexTypeCompleting
	ct, err := c.compileComplexType(definition.source.ctx, n, c.rt.ComplexTypes[id].Name)
	if err != nil {
		return err
	}
	ct.Final, err = derivationMaskWithDefaultChecked(n, semanticSource[*schemaComplexTypeSource](&n.semantic).Final, definition.source.ctx.doc.defaults.FinalDefault, complexTypeFinalDerivation())
	if err != nil {
		return err
	}
	if err := validateComplexExtensionAttributeAncestry(&c.rt, ct); err != nil {
		if diagnostic, ok := errors.AsType[*xsderrors.Error](err); ok && diagnostic != nil {
			return withSchemaCompileLocation(n, err)
		}
		return withSchemaCompileLocation(n, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, err.Error()))
	}
	c.completeComplexType(id, ct)
	c.complexDefinitions[id].state = complexTypeComplete
	return nil
}

func schemaComplexContentKind(n *schemaNode, attr lexicalAttribute, defaultKind complexContentVariety) (complexContentVariety, error) {
	var defaultMixed bool
	switch defaultKind {
	case contentElementOnly, contentEmpty:
	case contentMixed:
		defaultMixed = true
	case contentSimple:
		return contentElementOnly, xsderrors.InternalInvariant("complex content inherited a simple content kind")
	default:
		return contentElementOnly, xsderrors.InternalInvariant("complex content inherited an unknown content kind")
	}
	mixed, err := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrMixed, Value: attr.Value, HasValue: attr.Present, Default: defaultMixed})
	if err != nil {
		return contentElementOnly, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	if mixed {
		return contentMixed, nil
	}
	// A direct complexType with no model-group child has effective empty
	// content. Derivation containers are deliberately left element-only here;
	// their selected extension/restriction child is classified separately.
	if n != nil && n.local == vocab.XSDElemComplexType &&
		n.firstXS(vocab.XSDElemComplexContent) == nil &&
		n.firstXS(vocab.XSDElemSimpleContent) == nil &&
		firstModelChild(n) == nil {
		return contentEmpty, nil
	}
	return contentElementOnly, nil
}

// effectiveComplexContentKind maps the raw model-group syntax to the XSD
// complex-content variety before model lowering erases zero-occurrence and
// nested structure. In particular, model emptiability is not equivalent to
// the empty content variety: a present optional particle remains
// element-only, and a present required empty choice remains element-only
// even though its language is empty.
func (c *compiler) effectiveComplexContentKind(modelNode *schemaNode, contentKind complexContentVariety) (complexContentVariety, error) {
	if contentKind.Mixed() {
		// XSD supplies a synthetic empty sequence particle whenever effective
		// content is empty but effective mixed is true.
		return contentMixed, nil
	}
	if modelNode == nil {
		return contentEmpty, nil
	}

	occurs, err := parseOccurs(modelNode, c.limits)
	if err != nil {
		return contentElementOnly, err
	}
	if occurs.Max == 0 && !occurs.Unbounded {
		// A top-level particle whose maximum is zero corresponds to no
		// particle in the effective content mapping.
		return contentEmpty, nil
	}

	if modelNode.local == vocab.XSDElemGroup {
		// A non-zero group reference is a present particle even when its
		// referenced named group is syntactically empty.
		return contentElementOnly, nil
	}
	return effectiveModelGroupContentKind(modelNode, occurs)
}

func effectiveModelGroupContentKind(modelNode *schemaNode, occurs occurrence) (complexContentVariety, error) {
	kind, ok := schemaModelKind(modelNode)
	if !ok {
		return contentElementOnly, withSchemaCompileLocation(modelNode, xsderrors.InternalInvariant("model node has no typed model kind"))
	}
	// An annotation is metadata on the model group, not a particle in its
	// effective content. Keep this distinction before model lowering removes
	// the annotations and any zero-occurrence structure.
	for child := range modelNode.xsdChildren() {
		if child.local != vocab.XSDElemAnnotation {
			return contentElementOnly, nil
		}
	}
	if kind == modelChoice && occurs.Min != 0 {
		// An empty required choice is a present, impossible particle, so its
		// variety is element-only rather than empty.
		return contentElementOnly, nil
	}
	return contentEmpty, nil
}

// canonicalComplexContentModel makes the runtime model agree with the
// published content variety. Empty content has no executable particles; a
// mixed empty variety retains the synthetic text-permitting model flag.
func (c *compiler) canonicalComplexContentModel(id contentModelID, kind complexContentVariety) (contentModelID, error) {
	if id == noContentModel {
		return c.addModel(contentModelNode{Kind: modelEmpty, Mixed: kind.Mixed()})
	}
	model, ok := contentModelByID(c.rt.Models, id)
	if !ok {
		return noContentModel, xsderrors.InternalInvariant("complex content references missing content model")
	}
	if kind == contentEmpty {
		if model.Kind == modelEmpty &&
			len(model.Particles) == 0 &&
			model.Occurs == (occurrence{}) &&
			!model.Mixed {
			return id, nil
		}
		return c.addModelAt(contentModelNode{Kind: modelEmpty}, c.modelSources[id])
	}
	if model.Mixed == kind.Mixed() {
		return id, nil
	}
	model.Mixed = kind.Mixed()
	return c.addModelAt(model, c.modelSources[id])
}

func (c *compiler) compileComplexType(ctx *schemaSetDocument, n *schemaNode, name QName) (complexTypeMetadata, error) {
	ct, err := c.newComplexType(ctx, n, name)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	if complexContent := n.firstXS(vocab.XSDElemComplexContent); complexContent != nil {
		return c.compileComplexContent(ctx, complexContent, ct)
	}
	if simpleContent := n.firstXS(vocab.XSDElemSimpleContent); simpleContent != nil {
		return c.compileSimpleContent(ctx, simpleContent, ct)
	}
	return c.compileDirectComplexType(ctx, n, ct)
}

func (c *compiler) newComplexType(ctx *schemaSetDocument, n *schemaNode, name QName) (complexTypeMetadata, error) {
	source := semanticSource[*schemaComplexTypeSource](&n.semantic)
	if source == nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("complexType node has no typed complexType source"))
	}
	contentKind, err := schemaComplexContentKind(n, source.Mixed, contentElementOnly)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	abstract, err := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrAbstract, Value: source.Abstract.Value, HasValue: source.Abstract.Present})
	if err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	block, err := complexBlockMaskWithDefault(n, ctx.doc.defaults.BlockDefault)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct := complexTypeMetadata{
		Name:        name,
		Content:     noContentModel,
		Attrs:       noAttributeUseSet,
		TextType:    NoSimpleType,
		ContentKind: contentKind,
		Abstract:    abstract,
		Base:        ComplexRef(c.rt.AnyType),
		Derivation:  derivationKindRestriction,
		Block:       block,
	}
	return ct, nil
}

func (c *compiler) compileDirectComplexType(ctx *schemaSetDocument, n *schemaNode, ct complexTypeMetadata) (complexTypeMetadata, error) {
	modelNode := firstModelChild(n)
	content, err := c.compileDirectComplexModel(ctx, n)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	contentKind, err := c.effectiveComplexContentKind(modelNode, ct.ContentKind)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	content, err = c.canonicalComplexContentModel(content, contentKind)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Content = content
	ct.ContentKind = contentKind
	attrs, err := c.compileAttributeUses(ctx, n, nil, NoWildcard, attributeMergeDirect)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	return ct, nil
}

func (c *compiler) compileDirectComplexModel(ctx *schemaSetDocument, n *schemaNode) (contentModelID, error) {
	content := noContentModel
	for child := range n.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		model, present, err := c.compileDirectComplexModelChild(ctx, child)
		if err != nil {
			return noContentModel, err
		}
		if present {
			content = model
		}
	}
	return content, nil
}

func (c *compiler) compileDirectComplexModelChild(ctx *schemaSetDocument, child *schemaNode) (contentModelID, bool, error) {
	switch child.local {
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		if err := validateModelOccurrence(child, c.limits); err != nil {
			return noContentModel, false, err
		}
		model, err := c.compileModel(ctx, child)
		return model, true, err
	default:
		return noContentModel, false, nil
	}
}

func (c *compiler) compileComplexContent(ctx *schemaSetDocument, n *schemaNode, ct complexTypeMetadata) (complexTypeMetadata, error) {
	source, err := contentDerivationOf(n, complexContent)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	mixedSource, ok := n.semantic.kind.(*schemaComplexContentSource)
	if !ok || mixedSource == nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("complexContent node has no typed complexContent source"))
	}
	rawKind, err := schemaComplexContentKind(n, mixedSource.Mixed, ct.ContentKind)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	contentKind, err := c.effectiveComplexContentKind(firstModelChild(source.node), rawKind)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	return c.compileComplexContentDerivation(ctx, source.node, source.kind, ct, contentKind)
}

func (c *compiler) compileComplexContentDerivation(ctx *schemaSetDocument, child *schemaNode, kind contentDerivationKind, ct complexTypeMetadata, contentKind complexContentVariety) (complexTypeMetadata, error) {
	baseID, base, err := c.complexContentBase(ctx, child)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	extension := kind == contentDerivationExtension
	if err := validateMixedDerivationAt(child, base, kind, contentKind); err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Base = ComplexRef(baseID)
	if extension {
		return c.compileComplexContentExtension(ctx, child, ct, baseID, base, contentKind)
	}
	return c.compileComplexContentRestriction(ctx, child, ct, base, contentKind)
}

func (c *compiler) complexContentBase(ctx *schemaSetDocument, child *schemaNode) (ComplexTypeID, complexTypeMetadata, error) {
	baseQName, err := c.contentDerivationBaseQName(ctx, child)
	if err != nil {
		return noComplexType, complexTypeMetadata{}, err
	}
	baseID, err := c.compileComplexByQName(baseQName, child)
	if err != nil {
		return noComplexType, complexTypeMetadata{}, withSchemaCompileLocation(child, err)
	}
	return baseID, c.rt.complexType(baseID), nil
}

func (c *compiler) contentDerivationBaseQName(ctx *schemaSetDocument, child *schemaNode) (QName, error) {
	baseSource, ok := child.semantic.derivationBase()
	if !ok {
		return QName{}, withSchemaCompileLocation(child, xsderrors.InternalInvariant("derivation node has no typed derivation source"))
	}
	return c.referenceQName(ctx, child, baseSource.Name)
}

func (c *compiler) compileComplexContentExtension(ctx *schemaSetDocument, child *schemaNode, ct complexTypeMetadata, baseID ComplexTypeID, base complexTypeMetadata, contentKind complexContentVariety) (complexTypeMetadata, error) {
	if err := checkComplexTypeFinalAllows(base.Final, DerivationExtension, complexTypeFinalBaseExtension); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, err)
	}
	ct.Derivation = derivationKindExtension
	ct.ExplicitDerivation = true
	ct.Content = base.Content
	ct.TextType = base.TextType
	ct.ContentKind = complexExtensionContentKind(base, contentKind)
	if modelNode := firstModelChild(child); modelNode != nil {
		content, err := c.compileComplexExtensionModel(ctx, modelNode, baseID, base, contentKind)
		if err != nil {
			return complexTypeMetadata{}, err
		}
		ct.Content = content
	}
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcardForCompilation(base.Attrs)
	attrs, err := c.compileAttributeUses(ctx, child, baseUses, baseWildcard, attributeMergeExtension)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	if contentKind == contentEmpty || base.SimpleContent() {
		return ct, nil
	}
	ct.Content, err = c.canonicalComplexContentModel(ct.Content, ct.ContentKind)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	return ct, nil
}

func complexExtensionContentKind(base complexTypeMetadata, contentKind complexContentVariety) complexContentVariety {
	if contentKind == contentEmpty || base.SimpleContent() {
		return base.ContentKind
	}
	if base.Mixed() {
		return contentMixed
	}
	return contentKind
}

func (c *compiler) compileComplexExtensionModel(ctx *schemaSetDocument, modelNode *schemaNode, baseID ComplexTypeID, base complexTypeMetadata, contentKind complexContentVariety) (contentModelID, error) {
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return noContentModel, err
	}
	content, err := c.compileModel(ctx, modelNode)
	if err != nil {
		return noContentModel, err
	}
	if content == noContentModel || contentKind == contentEmpty {
		// Absent or empty effective content inherits the base after source
		// admission; an absent particle has no term to resolve.
		return base.Content, nil
	}
	if err := validateComplexExtensionContentAdmission(complexExtensionContentAdmission{
		BaseSimpleContent: base.SimpleContent(),
		HasModelChild:     true,
	}); err != nil {
		return noContentModel, withSchemaCompileLocation(modelNode, err)
	}
	if err := c.validateComplexExtensionModelAdmission(baseID, base, content, contentKind); err != nil {
		return noContentModel, withSchemaCompileLocation(modelNode, err)
	}
	addAtModelNode := func(model contentModelNode) (contentModelID, error) {
		return c.addModelAt(model, modelNode)
	}
	return extendSequenceModel(&c.rt, addAtModelNode, base.Content, content)
}

func validateMixedDerivationAt(child *schemaNode, base complexTypeMetadata, derivation contentDerivationKind, content complexContentVariety) error {
	if err := checkComplexContentMixedDerivationBase(base, derivation, content); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (c *compiler) compileComplexContentRestriction(ctx *schemaSetDocument, child *schemaNode, ct, base complexTypeMetadata, contentKind complexContentVariety) (complexTypeMetadata, error) {
	if err := checkComplexTypeFinalAllows(base.Final, DerivationRestriction, complexTypeFinalBaseRestriction); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, err)
	}
	if err := checkComplexContentRestrictionBase(base); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, err)
	}
	ct.Derivation = derivationKindRestriction
	ct.ExplicitDerivation = true
	content, err := c.compileComplexRestrictionModel(ctx, child, ct)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.ContentKind = contentKind
	content, err = c.canonicalComplexContentModel(content, contentKind)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Content = content
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcardForCompilation(base.Attrs)
	attrs, err := c.compileAttributeUses(ctx, child, baseUses, baseWildcard, attributeMergeRestriction)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	if err := validateComplexRestrictionAttributeWildcard(&c.rt, ct); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, err.Error()))
	}
	return ct, nil
}

func (c *compiler) compileComplexRestrictionModel(ctx *schemaSetDocument, child *schemaNode, ct complexTypeMetadata) (contentModelID, error) {
	modelNode := firstModelChild(child)
	if modelNode == nil {
		return c.addModel(contentModelNode{Kind: modelEmpty, Mixed: ct.Mixed()})
	}
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return noContentModel, err
	}
	return c.compileModel(ctx, modelNode)
}

func (c *compiler) compileSimpleContent(ctx *schemaSetDocument, n *schemaNode, ct complexTypeMetadata) (complexTypeMetadata, error) {
	source, err := c.resolveSimpleContentSource(ctx, n)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	var textType SimpleTypeID
	if c.simpleTypeQNameKnown(source.base) {
		ct, textType, err = c.compileSimpleContentSimpleBase(source.child, source.kind, source.base, ct)
	} else {
		ct, textType, err = c.compileSimpleContentComplexBase(source.child, source.kind, source.base, ct)
	}
	if err != nil {
		return complexTypeMetadata{}, err
	}
	derivation, err := c.compileSimpleContentDerivation(ctx, source, textType)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	return c.finishSimpleContent(ctx, source, ct, derivation)
}

func (c *compiler) finishSimpleContent(ctx *schemaSetDocument, source simpleContentSource, ct complexTypeMetadata, derivation simpleContentDerivation) (complexTypeMetadata, error) {
	if err := c.admitNotationTypeUse(source.child, derivation.textType); err != nil {
		return complexTypeMetadata{}, err
	}
	inheritedUses, inheritedWildcard := c.rt.attributeUsesAndWildcardForCompilation(ct.Attrs)
	attrs, err := c.compileAttributeUses(ctx, source.child, inheritedUses, inheritedWildcard, derivation.mergeMode)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	ct.Content, err = c.addModel(contentModelNode{Kind: modelEmpty})
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.TextType = derivation.textType
	ct.ContentKind = contentSimple
	ct.Derivation = derivation.kind
	ct.ExplicitDerivation = true
	if err := validateComplexRestrictionAttributeWildcard(&c.rt, ct); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(source.child, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, err.Error()))
	}
	return ct, nil
}

type simpleContentSource struct {
	child *schemaNode
	base  QName
	kind  contentDerivationKind
}

type simpleContentDerivation struct {
	textType  SimpleTypeID
	mergeMode attributeMergeMode
	kind      derivationKind
}

func (c *compiler) resolveSimpleContentSource(ctx *schemaSetDocument, n *schemaNode) (simpleContentSource, error) {
	syntax, err := contentDerivationOf(n, simpleContent)
	if err != nil {
		return simpleContentSource{}, err
	}
	baseSource, ok := syntax.node.semantic.derivationBase()
	if !ok {
		return simpleContentSource{}, withSchemaCompileLocation(syntax.node, xsderrors.InternalInvariant("derivation node has no typed derivation source"))
	}
	base, err := c.referenceQName(ctx, syntax.node, baseSource.Name)
	if err != nil {
		return simpleContentSource{}, err
	}
	return simpleContentSource{child: syntax.node, base: base, kind: syntax.kind}, nil
}

func (c *compiler) compileSimpleContentDerivation(ctx *schemaSetDocument, source simpleContentSource, textType SimpleTypeID) (simpleContentDerivation, error) {
	if source.kind != contentDerivationRestriction {
		return simpleContentDerivation{
			textType:  textType,
			mergeMode: attributeMergeExtension,
			kind:      derivationKindExtension,
		}, nil
	}
	restricted, err := c.compileSimpleContentRestrictionType(ctx, source.child, textType)
	return simpleContentDerivation{
		textType:  restricted,
		mergeMode: attributeMergeRestriction,
		kind:      derivationKindRestriction,
	}, err
}

func (c *compiler) compileSimpleContentSimpleBase(child *schemaNode, kind contentDerivationKind, baseQName QName, ct complexTypeMetadata) (complexTypeMetadata, SimpleTypeID, error) {
	if err := checkSimpleContentSimpleBase(kind); err != nil {
		return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	simpleID, err := c.compileSimpleByQName(baseQName)
	if err != nil {
		return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if err := checkSimpleBaseComplexExtensionFinalAllows(c.rt.simpleTypeFinalMask(simpleID)); err != nil {
		return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	ct.Base = SimpleRef(simpleID)
	return ct, simpleID, nil
}

func (c *compiler) compileSimpleContentComplexBase(child *schemaNode, kind contentDerivationKind, baseQName QName, ct complexTypeMetadata) (complexTypeMetadata, SimpleTypeID, error) {
	if !c.complexTypeQNameKnown(baseQName) {
		return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, simpleContentComplexBaseMissingError())
	}
	baseComplex, err := c.compileComplexByQName(baseQName, child)
	if err != nil {
		return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	base := c.rt.complexType(baseComplex)
	if err := checkSimpleContentDerivationBase(c.contentAnalysis, base, kind); err != nil {
		return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	switch kind {
	case contentDerivationNone:
		return complexTypeMetadata{}, NoSimpleType, xsderrors.InternalInvariant("simpleContent complex base derivation missing")
	case contentDerivationExtension:
		if err := checkComplexTypeFinalAllows(base.Final, DerivationExtension, complexTypeFinalBaseExtension); err != nil {
			return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, err)
		}
	case contentDerivationRestriction:
		if err := checkComplexTypeFinalAllows(base.Final, DerivationRestriction, complexTypeFinalBaseRestriction); err != nil {
			return complexTypeMetadata{}, NoSimpleType, withSchemaCompileLocation(child, err)
		}
	}
	ct.Base = ComplexRef(baseComplex)
	ct.Attrs = base.Attrs
	return ct, base.TextType, nil
}

func (c *compiler) compileSimpleContentRestrictionType(ctx *schemaSetDocument, child *schemaNode, baseTextType SimpleTypeID) (SimpleTypeID, error) {
	textType := baseTextType
	if stNode := child.firstXS(vocab.XSDElemSimpleType); stNode != nil {
		simpleID, err := c.compileAnonymousSimple(ctx, stNode)
		if err != nil {
			return NoSimpleType, err
		}
		textType = simpleID
	}
	if err := checkSimpleContentRestrictionTextTypePresent(textType); err != nil {
		return NoSimpleType, withSchemaCompileLocation(child, err)
	}
	simpleID, err := c.compileSimpleContentFacetRestriction(child, textType)
	if err != nil {
		return NoSimpleType, err
	}
	textType = simpleID
	if err := checkSimpleContentRestrictionTextType(&c.rt, textType, baseTextType, c.rt.spendTypeDerivationWork); err != nil {
		return NoSimpleType, withSchemaCompileLocation(child, err)
	}
	return textType, nil
}

func (c *compiler) compileSimpleContentFacetRestriction(parent *schemaNode, baseID SimpleTypeID) (SimpleTypeID, error) {
	first := firstFacetChild(parent)
	if first == nil {
		return baseID, nil
	}
	if err := checkSimpleRestrictionBase(baseID, valuepkg.AnySimpleTypeID); err != nil {
		return NoSimpleType, withSchemaCompileLocation(first, err)
	}
	if err := checkSimpleTypeFinalAllows(c.rt.simpleTypeFinalMask(baseID), DerivationRestriction, simpleTypeFinalBaseRestriction); err != nil {
		return NoSimpleType, withSchemaCompileLocation(first, err)
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$simple%d", len(c.rt.SimpleTypes)))
	if err != nil {
		return NoSimpleType, err
	}
	st := c.rt.derivedSimpleType(baseID, q)
	if err = c.compileFacetChildren(parent, &st, baseID); err != nil {
		return NoSimpleType, withSchemaCompileLocation(first, err)
	}
	if st.ValueSpec.Variety == valuepkg.Union {
		if chargeErr := c.chargeSimpleUnionMemberEntries(first, len(st.ValueSpec.Union)); chargeErr != nil {
			return NoSimpleType, chargeErr
		}
	}
	id, err := c.addSimpleType(st)
	if err != nil {
		return NoSimpleType, err
	}
	if err := c.completeSimpleType(id, st); err != nil {
		return NoSimpleType, err
	}
	return id, nil
}

func firstModelChild(n *schemaNode) *schemaNode {
	for child := range n.xsdChildren() {
		switch child.local {
		case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
			return child
		}
	}
	return nil
}

func firstFacetChild(n *schemaNode) *schemaNode {
	for child := range n.xsdChildren() {
		if isFacetLocal(child.local) {
			return child
		}
	}
	return nil
}
