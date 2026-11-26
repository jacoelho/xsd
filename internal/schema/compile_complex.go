package schema

import (
	"github.com/jacoelho/xsd/internal/diagnostictext"
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
// label names an anonymous type in diagnostics.
type complexTypeDefinition struct {
	source schemaComponent
	label  complexTypeLabel
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

func (c *compiler) reserveAnonymousComplexType(ctx *schemaSetDocument, n *schemaNode, label complexTypeLabel) (ComplexTypeID, error) {
	if err := c.spendComponentDependency(n); err != nil {
		return noComplexType, err
	}
	id, err := c.addComplexType(c.complexTypePlaceholder(noQName()))
	if err != nil {
		return noComplexType, err
	}
	c.complexDefinitions[id].source = schemaComponent{ctx: ctx, node: n}
	c.complexDefinitions[id].label = label
	return id, nil
}

// complexTypeOwner is the complex type definition whose content is being
// compiled: id, the type every model the content creates belongs to, and node,
// its source.
type complexTypeOwner struct {
	node *schemaNode
	id   ComplexTypeID
}

type complexTypeLabelKind uint8

const (
	complexTypeLabelNamed complexTypeLabelKind = iota + 1
	complexTypeLabelElement
	complexTypeLabelAbsentElement
)

// complexTypeLabel names a complex type in a diagnostic: a named type by its
// name, an anonymous type by the element declaration holding it. The
// anonymous type of a zero-count element particle has no declaration and is
// named by the element's expanded name, which is never interned. The zero
// value is no label.
type complexTypeLabel struct {
	element ExpandedName
	name    QName
	kind    complexTypeLabelKind
}

func namedComplexTypeLabel(name QName) complexTypeLabel {
	return complexTypeLabel{name: name, kind: complexTypeLabelNamed}
}

func elementComplexTypeLabel(element QName) complexTypeLabel {
	return complexTypeLabel{name: element, kind: complexTypeLabelElement}
}

func absentElementComplexTypeLabel(element ExpandedName) complexTypeLabel {
	return complexTypeLabel{element: element, kind: complexTypeLabelAbsentElement}
}

func (l complexTypeLabel) valid() bool {
	return l.kind != 0
}

// label renders a valid label.
func (l complexTypeLabel) label(names *nameTable) string {
	switch l.kind {
	case complexTypeLabelElement:
		return "local complex type of element " + names.Format(l.name)
	case complexTypeLabelAbsentElement:
		return "local complex type of zero-count element " + diagnostictext.ExpandedName(l.element.Namespace, l.element.Local)
	case complexTypeLabelNamed:
	}
	return "complex type " + names.Format(l.name)
}

// complexTypeLabel returns the diagnostic label of the complex type id.
func (c *compiler) complexTypeLabel(id ComplexTypeID) complexTypeLabel {
	typ := c.rt.ComplexTypes[id]
	if typ.Scope == declarationScopeGlobal {
		return namedComplexTypeLabel(typ.Name)
	}
	return c.complexDefinitions[id].label
}

func (c *compiler) complexTypePlaceholder(q QName) complexTypeMetadata {
	return complexTypeMetadata{Name: q, Content: noContentModel, Attrs: noAttributeUseSet, TextType: NoSimpleType, Base: ComplexRef(c.rt.AnyType)}
}

func (c *compiler) complexTypeCycleError(id ComplexTypeID, use *schemaNode) error {
	err := schemaComponentCycleError(c.complexTypeLabel(id).label(&c.rt.Names))
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
		return xsderrors.InternalInvariant("unknown complex type completion state", nil)
	}
	n := definition.source.sourceNode()
	if n == nil {
		return xsderrors.InternalInvariant("complex type definition has no source", nil)
	}
	if err = c.spendComponentDependency(n); err != nil {
		return err
	}
	scope, err := c.enterComponent(n)
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	c.complexDefinitions[id].state = complexTypeCompleting
	ct, err := c.compileComplexType(definition.source.ctx, complexTypeOwner{node: n, id: id})
	if err != nil {
		return err
	}
	source, err := requiredSemanticSource[*schemaComplexTypeSource](n)
	if err != nil {
		return err
	}
	ct.Final, err = derivationMaskWithDefaultChecked(n, source.Final, definition.source.ctx.doc.defaults.FinalDefault, complexTypeFinalDerivation())
	if err != nil {
		return err
	}
	if err := validateComplexExtensionAttributeAncestry(&c.rt, ct); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
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
		return contentElementOnly, xsderrors.InternalInvariant("complex content inherited a simple content kind", nil)
	default:
		return contentElementOnly, xsderrors.InternalInvariant("complex content inherited an unknown content kind", nil)
	}
	mixed, err := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrMixed, Value: attr.Value, HasValue: attr.Present, Default: defaultMixed})
	if err != nil {
		return contentElementOnly, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	if mixed {
		return contentMixed, nil
	}
	return contentElementOnly, nil
}

// explicitContentEmpty reports whether modelNode, the model group child of a
// complex type definition or nil, gives an empty ·effective content·
// (Structures §3.4.2 complex content clauses 2.1.1 to 2.1.3), so the content
// has no particle of its own: an empty effective mixed content holds the
// synthetic empty sequence of clause 2.1.4 instead. A particle with maxOccurs
// 0 corresponds to no component (§3.8.2). It reads the source, because
// lowering drops annotations and zero-count particles: a present optional
// particle is not empty, and neither is a required choice with no members
// (clause 2.2), which matches nothing (I-042 in docs/spec/INTERPRETATIONS.md).
func explicitContentEmpty(modelNode *schemaNode) (bool, error) {
	if modelNode == nil {
		return true, nil
	}
	occurs, err := particleOccurrence(modelNode)
	if err != nil {
		return false, err
	}
	if occurs.IsAbsent() {
		return true, nil
	}
	if modelNode.local == vocab.XSDElemGroup {
		return false, nil
	}
	kind, ok := schemaModelKind(modelNode)
	if !ok {
		return false, withSchemaCompileLocation(modelNode, xsderrors.InternalInvariant("model node has no typed model kind", nil))
	}
	for child := range modelNode.xsdChildren() {
		if child.local != vocab.XSDElemAnnotation {
			return false, nil
		}
	}
	return kind != modelChoice || occurs.Min == 0, nil
}

// compileExplicitContent compiles modelNode, the model group child of a
// complex type definition or nil, and returns its ·effective content· with the
// {content type} variety it gives under the ·effective mixed· of mixedKind
// (Structures §3.4.2 complex content clauses 1 and 2), its model compiled
// for use, and no model when the effective content is empty. It also returns
// the occurrence of the model's particle, which a term's record does not
// carry. It is the one compilation of explicit content that the direct,
// restriction and extension paths call.
func (c *compiler) compileExplicitContent(ctx *schemaSetDocument, owner ComplexTypeID, modelNode *schemaNode, mixedKind complexContentVariety, use modelUse) (effectiveContent, occurrence, error) {
	content := effectiveContent{model: noContentModel, variety: contentElementOnly}
	var occurs occurrence
	if modelNode != nil {
		var err error
		if content.model, occurs, err = c.compileModel(ctx, modelNode, use, owner); err != nil {
			return effectiveContent{}, occurrence{}, err
		}
	}
	empty, err := explicitContentEmpty(modelNode)
	if err != nil {
		return effectiveContent{}, occurrence{}, err
	}
	if empty {
		content.model, content.variety = noContentModel, contentEmpty
	}
	if mixedKind.Mixed() {
		content.variety = contentMixed
	}
	return content, occurs, nil
}

// canonicalComplexContentModel makes the runtime model agree with the
// published content variety: empty content, and the synthetic empty sequence
// of mixed empty content, have an empty model with no executable particles. A
// new model belongs to owner and keeps the source of the model it replaces;
// the owner's node is the source of the model made for noContentModel.
func (c *compiler) canonicalComplexContentModel(id contentModelID, kind complexContentVariety, owner complexTypeOwner) (contentModelID, error) {
	if id == noContentModel {
		return c.addModel(contentModelNode{Kind: modelEmpty}, owner.node, owner.id)
	}
	model, ok := contentModelByID(c.rt.Models, id)
	if !ok {
		return noContentModel, xsderrors.InternalInvariant("complex content references missing content model", nil)
	}
	if kind == contentEmpty {
		if model.Kind == modelEmpty &&
			len(model.Particles) == 0 &&
			model.Occurs == (occurrence{}) {
			return id, nil
		}
		return c.addModel(contentModelNode{Kind: modelEmpty}, c.modelSources[id], owner.id)
	}
	return id, nil
}

func (c *compiler) compileComplexType(ctx *schemaSetDocument, owner complexTypeOwner) (complexTypeMetadata, error) {
	n := owner.node
	ct, err := c.newComplexType(ctx, n, c.rt.ComplexTypes[owner.id].Name)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	if complexContent := n.firstXS(vocab.XSDElemComplexContent); complexContent != nil {
		return c.compileComplexContent(ctx, owner, complexContent, ct)
	}
	if simpleContent := n.firstXS(vocab.XSDElemSimpleContent); simpleContent != nil {
		return c.compileSimpleContent(ctx, owner, simpleContent, ct)
	}
	return c.compileDirectComplexType(ctx, owner, ct)
}

func (c *compiler) newComplexType(ctx *schemaSetDocument, n *schemaNode, name QName) (complexTypeMetadata, error) {
	source, err := requiredSemanticSource[*schemaComplexTypeSource](n)
	if err != nil {
		return complexTypeMetadata{}, err
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

func (c *compiler) compileDirectComplexType(ctx *schemaSetDocument, owner complexTypeOwner, ct complexTypeMetadata) (complexTypeMetadata, error) {
	n := owner.node
	content, _, err := c.compileExplicitContent(ctx, owner.id, firstModelChild(n), ct.ContentKind, modelRoot)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Content, err = c.canonicalComplexContentModel(content.model, content.variety, owner)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.ContentKind = content.variety
	attrs, err := c.compileAttributeUses(ctx, n, nil, NoWildcard, attributeMergeDirect)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	return ct, nil
}

func (c *compiler) compileComplexContent(ctx *schemaSetDocument, owner complexTypeOwner, n *schemaNode, ct complexTypeMetadata) (complexTypeMetadata, error) {
	source, err := contentDerivationOf(n, complexContent)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	mixedSource, ok := n.semantic.kind.(*schemaComplexContentSource)
	if !ok || mixedSource == nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("complexContent node has no typed complexContent source", nil))
	}
	mixedKind, err := schemaComplexContentKind(n, mixedSource.Mixed, ct.ContentKind)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	return c.compileComplexContentDerivation(ctx, owner, source.node, source.kind, ct, mixedKind)
}

func (c *compiler) compileComplexContentDerivation(ctx *schemaSetDocument, owner complexTypeOwner, child *schemaNode, kind contentDerivationKind, ct complexTypeMetadata, mixedKind complexContentVariety) (complexTypeMetadata, error) {
	baseID, base, err := c.complexContentBase(ctx, child)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	extension := kind == contentDerivationExtension
	if err := validateMixedDerivationAt(child, base, kind, mixedKind); err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Base = ComplexRef(baseID)
	if extension {
		return c.compileComplexContentExtension(ctx, owner, child, ct, baseID, mixedKind)
	}
	return c.compileComplexContentRestriction(ctx, owner, child, ct, base, mixedKind)
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
	derivation, err := requiredSemanticSource[*schemaBaseDerivationSource](child)
	if err != nil {
		return QName{}, err
	}
	baseSource := derivation.Base
	return c.referenceQName(ctx, child, baseSource.Name)
}

func (c *compiler) compileComplexContentExtension(ctx *schemaSetDocument, owner complexTypeOwner, child *schemaNode, ct complexTypeMetadata, baseID ComplexTypeID, mixedKind complexContentVariety) (complexTypeMetadata, error) {
	base := c.rt.complexType(baseID)
	if err := checkComplexTypeFinalAllows(base.Final, DerivationExtension, complexTypeFinalBaseExtension); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, err)
	}
	ct.Derivation = derivationKindExtension
	ct.ExplicitDerivation = true
	ct.TextType = base.TextType
	modelNode, at := firstModelChild(child), child
	if modelNode != nil {
		at = modelNode
	}
	use := extensionExplicitUse(base)
	effective, occurs, err := c.compileExplicitContent(ctx, owner.id, modelNode, mixedKind, use)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	content, err := c.extensionContentType(owner.id, base, effective, use, occurs, at)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Content, ct.ContentKind = content.model, content.variety
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcardForCompilation(base.Attrs)
	attrs, err := c.compileAttributeUses(ctx, child, baseUses, baseWildcard, attributeMergeExtension)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	if effective.variety == contentEmpty {
		return ct, nil
	}
	ct.Content, err = c.canonicalComplexContentModel(ct.Content, ct.ContentKind, owner)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	return ct, nil
}

// effectiveContent is a {content type} under construction: its variety and,
// unless the content is empty or the synthetic empty sequence of mixed empty
// content, the model of its particle. For an extension's explicit content it
// is the ·effective content· with its ·effective mixed· (Structures §3.4.2
// complex content clauses 1 and 2), which compileExplicitContent decides
// from the source.
type effectiveContent struct {
	model   contentModelID
	variety complexContentVariety
}

// extensionExplicitUse returns what the explicit content of an extension of
// base is compiled as: the term of the second particle of a content sequence
// (Structures §3.4.2 complex content clause 3.2.3) when base has element
// content, and otherwise the root of the {content type} itself.
func extensionExplicitUse(base complexTypeMetadata) modelUse {
	if base.SimpleContent() || base.ContentKind == contentEmpty {
		return modelRoot
	}
	return modelTerm
}

// extensionContentType returns the {content type} of a complex-content
// extension (Structures §3.4.2 complex content clause 3.2). Empty effective
// content keeps the base content type (3.2.1), an empty base content type
// takes the effective content (3.2.2), and otherwise the content is a 1..1
// sequence of the base particle followed by the effective content (3.2.3).
// use is extensionExplicitUse(base), what effective's model was compiled as,
// and occurs the occurrence of its particle. The models it creates belong to
// owner, and at locates the diagnostics of clause 3.2.3.
func (c *compiler) extensionContentType(owner ComplexTypeID, base complexTypeMetadata, effective effectiveContent, use modelUse, occurs occurrence, at *schemaNode) (effectiveContent, error) {
	switch {
	case effective.variety == contentEmpty:
		return effectiveContent{model: base.Content, variety: base.ContentKind}, nil
	case base.SimpleContent():
		return effectiveContent{}, withSchemaCompileLocation(at, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot add particles to simple content", nil))
	case use == modelRoot:
		return effective, nil
	}
	if base.Mixed() && !effective.variety.Mixed() {
		return effectiveContent{}, withSchemaCompileLocation(at, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "complexContent extension cannot drop mixed base content", nil))
	}
	if err := validateExtensionSequenceTerms(&c.rt, base.Content, effective); err != nil {
		return effectiveContent{}, withSchemaCompileLocation(at, err)
	}
	add := func(model contentModelNode) (contentModelID, error) {
		return c.addModel(model, at, owner)
	}
	baseParticle, err := contentTypeParticle(&c.rt, add, base.Content)
	if err != nil {
		return effectiveContent{}, err
	}
	explicitParticle := modelParticle(effective.model, occurs)
	if effective.model == noContentModel {
		if explicitParticle, err = contentTypeParticle(&c.rt, add, noContentModel); err != nil {
			return effectiveContent{}, err
		}
	}
	sequence := contentModelNode{Kind: modelSequence, Occurs: occurrence{Min: 1, Max: 1}}
	if err = appendModelParticles(&c.dependencyWork, c.rt.Models, &sequence, baseParticle, explicitParticle); err != nil {
		return effectiveContent{}, err
	}
	id, err := add(sequence)
	if err != nil {
		return effectiveContent{}, err
	}
	return effectiveContent{model: id, variety: effective.variety}, nil
}

func validateMixedDerivationAt(child *schemaNode, base complexTypeMetadata, derivation contentDerivationKind, mixedKind complexContentVariety) error {
	if err := checkComplexContentMixedDerivationBase(base, derivation, mixedKind); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (c *compiler) compileComplexContentRestriction(ctx *schemaSetDocument, owner complexTypeOwner, child *schemaNode, ct, base complexTypeMetadata, mixedKind complexContentVariety) (complexTypeMetadata, error) {
	if err := checkComplexTypeFinalAllows(base.Final, DerivationRestriction, complexTypeFinalBaseRestriction); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, err)
	}
	if err := checkComplexContentRestrictionBase(base); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, err)
	}
	ct.Derivation = derivationKindRestriction
	ct.ExplicitDerivation = true
	content, _, err := c.compileExplicitContent(ctx, owner.id, firstModelChild(child), mixedKind, modelRoot)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.ContentKind = content.variety
	ct.Content, err = c.canonicalComplexContentModel(content.model, content.variety, owner)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcardForCompilation(base.Attrs)
	attrs, err := c.compileAttributeUses(ctx, child, baseUses, baseWildcard, attributeMergeRestriction)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	if err := validateComplexRestrictionAttributeWildcard(&c.rt, ct); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(child, invalidAttributeError(err))
	}
	return ct, nil
}

func (c *compiler) compileSimpleContent(ctx *schemaSetDocument, owner complexTypeOwner, n *schemaNode, ct complexTypeMetadata) (complexTypeMetadata, error) {
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
	return c.finishSimpleContent(ctx, owner, source, ct, derivation)
}

func (c *compiler) finishSimpleContent(ctx *schemaSetDocument, owner complexTypeOwner, source simpleContentSource, ct complexTypeMetadata, derivation simpleContentDerivation) (complexTypeMetadata, error) {
	if err := c.admitNotationTypeUse(source.child, derivation.textType); err != nil {
		return complexTypeMetadata{}, err
	}
	inheritedUses, inheritedWildcard := c.rt.attributeUsesAndWildcardForCompilation(ct.Attrs)
	attrs, err := c.compileAttributeUses(ctx, source.child, inheritedUses, inheritedWildcard, derivation.mergeMode)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.Attrs = attrs
	ct.Content, err = c.addModel(contentModelNode{Kind: modelEmpty}, owner.node, owner.id)
	if err != nil {
		return complexTypeMetadata{}, err
	}
	ct.TextType = derivation.textType
	ct.ContentKind = contentSimple
	ct.Derivation = derivation.kind
	ct.ExplicitDerivation = true
	if err := validateComplexRestrictionAttributeWildcard(&c.rt, ct); err != nil {
		return complexTypeMetadata{}, withSchemaCompileLocation(source.child, invalidAttributeError(err))
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
	derivation, err := requiredSemanticSource[*schemaBaseDerivationSource](syntax.node)
	if err != nil {
		return simpleContentSource{}, err
	}
	baseSource := derivation.Base
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
		return complexTypeMetadata{}, NoSimpleType, xsderrors.InternalInvariant("simpleContent complex base derivation missing", nil)
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
	st, err := c.rt.derivedSimpleType(baseID, noQName())
	if err != nil {
		return NoSimpleType, withSchemaCompileLocation(first, err)
	}
	if err = c.compileFacetChildren(parent, &st, baseID); err != nil {
		return NoSimpleType, withSchemaCompileLocation(first, err)
	}
	if st.spec.Variety == valuepkg.Union {
		if chargeErr := c.chargeSimpleUnionMemberEntries(first, len(st.spec.Union)); chargeErr != nil {
			return NoSimpleType, chargeErr
		}
	}
	id, err := c.addSimpleType(st.decl)
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
