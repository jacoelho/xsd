package schema

import (
	"fmt"
	"slices"

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

// compileComplexByQName returns a named complex type with its definition
// complete, as a derivation base requires.
func (c *compiler) compileComplexByQName(q QName) (ComplexTypeID, error) {
	label := c.rt.formatName(q)
	if err := c.rejectComplexTypeCycle(q, label); err != nil {
		return NoComplexType, err
	}
	if i := c.deferredComplexIndex(q); i >= 0 {
		return c.completeRequestedDeferredComplex(i)
	}
	raw, exists := c.complexComponents[q]
	var source *schemaNode
	if exists {
		source = raw.sourceNode()
	}
	if err := c.spendComponentDependency(source); err != nil {
		return NoComplexType, err
	}
	if id, ok := c.complexDone[q]; ok {
		return id, nil
	}
	if !exists {
		return NoComplexType, SchemaComponentMissingError(SchemaComponentComplexType, label)
	}
	leave, err := c.enterComponent(raw.sourceNode())
	if err != nil {
		return NoComplexType, err
	}
	defer leave()
	c.compilingComplex[q] = true
	defer delete(c.compilingComplex, q)
	id, err := c.registerGlobalComplexType(q, ComplexType{Name: q, Content: NoContentModel, Attrs: NoAttributeUseSet, TextType: NoSimpleType, Base: ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return NoComplexType, err
	}
	c.complexDone[q] = id
	ct, err := c.compileComplexType(raw.sourceNode(), raw.ctx, q, complexTypeScopeGlobal)
	if err != nil {
		return NoComplexType, err
	}
	if err := c.completeGlobalComplexType(raw, q, id, &ct); err != nil {
		return NoComplexType, err
	}
	return id, c.drainDeferredComplex()
}

// compileComplexReference returns a named complex type used as the type of an
// element declaration. Such a reference needs only the type's identity, so a
// type whose base is still compiling is registered and deferred.
func (c *compiler) compileComplexReference(q QName) (ComplexTypeID, error) {
	if id, ok := c.complexDone[q]; ok {
		return id, nil
	}
	raw, exists := c.complexComponents[q]
	if !exists {
		return c.compileComplexByQName(q)
	}
	base, deferCompletion, err := c.compilingComplexBase(raw.sourceNode(), raw.ctx)
	if err != nil || !deferCompletion {
		return c.compileComplexByQName(q)
	}
	if spendErr := c.spendComponentDependency(raw.sourceNode()); spendErr != nil {
		return NoComplexType, spendErr
	}
	id, err := c.registerGlobalComplexType(q, ComplexType{Name: q, Content: NoContentModel, Attrs: NoAttributeUseSet, TextType: NoSimpleType, Base: ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return NoComplexType, err
	}
	c.complexDone[q] = id
	c.deferredComplex = append(c.deferredComplex, deferredComplex{
		name: q, id: id, base: base, scope: complexTypeScopeGlobal, raw: raw,
	})
	return id, nil
}

// completeRequestedDeferredComplex completes a deferred named type that is
// needed as a derivation base. Its own base must be complete by now;
// otherwise the two types derive from each other.
func (c *compiler) completeRequestedDeferredComplex(i int) (ComplexTypeID, error) {
	item := c.deferredComplex[i]
	if err := c.rejectComplexTypeCycle(item.base, c.rt.formatName(item.base)); err != nil {
		return NoComplexType, err
	}
	c.deferredComplex = slices.Delete(c.deferredComplex, i, i+1)
	return item.id, c.completeDeferredComplex(item)
}

func (c *compiler) deferredComplexIndex(q QName) int {
	return slices.IndexFunc(c.deferredComplex, func(d deferredComplex) bool {
		return d.scope == complexTypeScopeGlobal && d.name == q
	})
}

// drainDeferredComplex completes every deferred type whose base is no longer
// compiling, including types that become ready while others complete.
func (c *compiler) drainDeferredComplex() error {
	for {
		i := slices.IndexFunc(c.deferredComplex, func(d deferredComplex) bool {
			return !c.compilingComplex[d.base]
		})
		if i < 0 {
			return nil
		}
		item := c.deferredComplex[i]
		c.deferredComplex = slices.Delete(c.deferredComplex, i, i+1)
		if err := c.completeDeferredComplex(item); err != nil {
			return err
		}
	}
}

func (c *compiler) completeDeferredComplex(item deferredComplex) error {
	if item.scope == complexTypeScopeAnonymous {
		_, err := c.completeAnonymousComplex(item.id, item.name, item.node, item.ctx)
		return err
	}
	leave, err := c.enterComponent(item.raw.sourceNode())
	if err != nil {
		return err
	}
	defer leave()
	c.compilingComplex[item.name] = true
	defer delete(c.compilingComplex, item.name)
	ct, err := c.compileComplexType(item.raw.sourceNode(), item.raw.ctx, item.name, complexTypeScopeGlobal)
	if err != nil {
		return err
	}
	return c.completeGlobalComplexType(item.raw, item.name, item.id, &ct)
}

func (c *compiler) rejectComplexTypeCycle(q QName, label string) error {
	if !c.compilingComplex[q] {
		return nil
	}
	err := SchemaComponentCycleError(SchemaComponentComplexType, label)
	if raw, ok := c.complexComponents[q]; ok {
		return withSchemaCompileLocation(raw.sourceNode(), err)
	}
	return err
}

func (c *compiler) completeGlobalComplexType(raw schemaComponent, q QName, id ComplexTypeID, ct *ComplexType) error {
	block, err := complexBlockMaskWithDefault(raw.sourceNode(), raw.ctx.blockDefault)
	if err != nil {
		return err
	}
	final, err := derivationMaskWithDefaultChecked(raw.sourceNode(), raw.ctx.finalDefault, complexTypeFinalDerivation())
	if err != nil {
		return err
	}
	ct.Name = q
	ct.Block = block
	ct.Final = final
	c.completeComplexType(id, *ct)
	return nil
}

func (c *compiler) compileAnonymousComplex(n *schemaNode, ctx *schemaContext) (ComplexTypeID, error) {
	if err := c.spendComponentDependency(n); err != nil {
		return NoComplexType, err
	}
	if err := checkLocalComplexTypeAttributes(n); err != nil {
		return NoComplexType, err
	}
	if err := checkComplexTypeChildren(n); err != nil {
		return NoComplexType, err
	}
	base, deferCompletion, err := c.compilingComplexBase(n, ctx)
	if err != nil {
		return NoComplexType, err
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$complex%d", c.rt.ComplexTypeCount()))
	if err != nil {
		return NoComplexType, err
	}
	id, err := c.addComplexType(ComplexType{Name: q, Content: NoContentModel, Attrs: NoAttributeUseSet, TextType: NoSimpleType, Base: ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return NoComplexType, err
	}
	if deferCompletion {
		c.deferredComplex = append(c.deferredComplex, deferredComplex{
			name: q, id: id, base: base, scope: complexTypeScopeAnonymous, node: n, ctx: ctx,
		})
		return id, nil
	}
	return c.completeAnonymousComplex(id, q, n, ctx)
}

func (c *compiler) completeAnonymousComplex(id ComplexTypeID, q QName, n *schemaNode, ctx *schemaContext) (ComplexTypeID, error) {
	leave, err := c.enterComponent(n)
	if err != nil {
		return NoComplexType, err
	}
	defer leave()
	ct, err := c.compileComplexType(n, ctx, q, complexTypeScopeAnonymous)
	if err != nil {
		return NoComplexType, err
	}
	final, err := derivationMaskWithDefaultChecked(n, ctx.finalDefault, complexTypeFinalDerivation())
	if err != nil {
		return NoComplexType, err
	}
	ct.Name = q
	ct.Final = final
	c.completeComplexType(id, ct)
	return id, nil
}

// compilingComplexBase reports the derivation base of a complex type
// definition when that base is currently compiling.
func (c *compiler) compilingComplexBase(n *schemaNode, ctx *schemaContext) (QName, bool, error) {
	container := vocab.XSDElemComplexContent
	var kind ContentDerivationKind
	var node *schemaNode
	if cc := n.firstXS(vocab.XSDElemComplexContent); cc != nil {
		source, err := checkComplexContentSyntax(cc)
		if err != nil {
			return QName{}, false, err
		}
		kind, node = source.kind, source.node
	} else if sc := n.firstXS(vocab.XSDElemSimpleContent); sc != nil {
		source, err := checkSimpleContentSyntax(sc)
		if err != nil {
			return QName{}, false, err
		}
		container, kind, node = vocab.XSDElemSimpleContent, source.kind, source.node
	} else {
		return QName{}, false, nil
	}
	base, err := c.contentDerivationBaseQName(container, kind, node, ctx)
	if err != nil {
		return QName{}, false, err
	}
	return base, c.compilingComplex[base], nil
}

func schemaBoolAttrDefault(n *schemaNode, name string, def bool) (bool, error) {
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

func schemaComplexContentKind(n *schemaNode, defaultKind ContentKind) (ContentKind, error) {
	var defaultMixed bool
	switch defaultKind {
	case ContentElementOnly, ContentEmpty:
	case ContentMixed:
		defaultMixed = true
	case ContentSimple, ContentSimpleMixed:
		return ContentElementOnly, xsderrors.InternalInvariant("complex content inherited a simple content kind")
	default:
		return ContentElementOnly, xsderrors.InternalInvariant("complex content inherited an unknown content kind")
	}
	mixed, err := schemaBoolAttrDefault(n, vocab.XSDAttrMixed, defaultMixed)
	if err != nil {
		return ContentElementOnly, err
	}
	if mixed {
		return ContentMixed, nil
	}
	// A direct complexType with no model-group child has effective empty
	// content. Derivation containers are deliberately left element-only here;
	// their selected extension/restriction child is classified separately.
	if n != nil && n.local == vocab.XSDElemComplexType &&
		n.firstXS(vocab.XSDElemComplexContent) == nil &&
		n.firstXS(vocab.XSDElemSimpleContent) == nil &&
		firstModelChild(n) == nil {
		return ContentEmpty, nil
	}
	return ContentElementOnly, nil
}

// effectiveComplexContentKind maps the raw model-group syntax to the XSD
// complex-content variety before model lowering erases zero-occurrence and
// nested structure. In particular, model emptiability is not equivalent to
// the empty content variety: a present optional particle remains
// element-only, and a present required empty choice remains element-only
// even though its language is empty.
func (c *compiler) effectiveComplexContentKind(modelNode *schemaNode, contentKind ContentKind) (ContentKind, error) {
	if contentKind.Mixed() {
		// XSD supplies a synthetic empty sequence particle whenever effective
		// content is empty but effective mixed is true.
		return ContentMixed, nil
	}
	if modelNode == nil {
		return ContentEmpty, nil
	}

	occurs, err := parseOccurs(modelNode, c.limits)
	if err != nil {
		return ContentElementOnly, err
	}
	if occurs.Max == 0 && !occurs.Unbounded {
		// A top-level particle whose maximum is zero corresponds to no
		// particle in the effective content mapping.
		return ContentEmpty, nil
	}

	if modelNode.kind == schemaKindGroup {
		// A non-zero group reference is a present particle even when its
		// referenced named group is syntactically empty.
		return ContentElementOnly, nil
	}
	return effectiveModelGroupContentKind(modelNode, occurs)
}

func effectiveModelGroupContentKind(modelNode *schemaNode, occurs Occurrence) (ContentKind, error) {
	kind, ok := schemaModelKind(modelNode)
	if !ok {
		return ContentElementOnly, withSchemaCompileLocation(modelNode, xsderrors.InternalInvariant("model node has no typed model kind"))
	}
	// An annotation is metadata on the model group, not a particle in its
	// effective content. Keep this distinction before model lowering removes
	// the annotations and any zero-occurrence structure.
	for _, child := range schemaModelChildren(modelNode) {
		if child.kind != schemaKindAnnotation {
			return ContentElementOnly, nil
		}
	}
	if kind == ModelChoice && occurs.Min != 0 {
		// An empty required choice is a present, impossible particle, so its
		// variety is element-only rather than empty.
		return ContentElementOnly, nil
	}
	return ContentEmpty, nil
}

// canonicalComplexContentModel makes the runtime model agree with the
// published content variety. Empty content has no executable particles; a
// mixed empty variety retains the synthetic text-permitting model flag.
func (c *compiler) canonicalComplexContentModel(id ContentModelID, kind ContentKind) (ContentModelID, error) {
	if id == NoContentModel {
		return c.addModel(ContentModel{Kind: ModelEmpty, Mixed: kind.Mixed()})
	}
	model, ok := c.rt.ContentModel(id)
	if !ok {
		return NoContentModel, xsderrors.InternalInvariant("complex content references missing content model")
	}
	if kind == ContentEmpty {
		if model.Kind == ModelEmpty &&
			len(model.Particles) == 0 &&
			len(model.ChoiceLimits) == 0 &&
			model.Occurs == (Occurrence{}) &&
			!model.Mixed {
			return id, nil
		}
		return c.addModelAt(ContentModel{Kind: ModelEmpty}, c.modelSources[id])
	}
	if model.Mixed == kind.Mixed() {
		return id, nil
	}
	model.Mixed = kind.Mixed()
	return c.addModelAt(model, c.modelSources[id])
}

func (c *compiler) compileComplexType(n *schemaNode, ctx *schemaContext, name QName, scope complexTypeScope) (ComplexType, error) {
	if err := validateComplexTypeScope(scope); err != nil {
		return ComplexType{}, err
	}
	if err := checkComplexTypeChildren(n); err != nil {
		return ComplexType{}, err
	}
	ct, err := c.newComplexType(n, ctx, name)
	if err != nil {
		return ComplexType{}, err
	}
	if complexContent := n.firstXS(vocab.XSDElemComplexContent); complexContent != nil {
		return c.compileComplexContent(complexContent, ctx, ct, scope)
	}
	if simpleContent := n.firstXS(vocab.XSDElemSimpleContent); simpleContent != nil {
		return c.compileSimpleContent(simpleContent, ctx, ct, scope)
	}
	return c.compileDirectComplexType(n, ctx, ct)
}

func (c *compiler) newComplexType(n *schemaNode, ctx *schemaContext, name QName) (ComplexType, error) {
	contentKind, err := schemaComplexContentKind(n, ContentElementOnly)
	if err != nil {
		return ComplexType{}, err
	}
	source := n.semantic.complexType()
	if source == nil {
		return ComplexType{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("complexType node has no typed complexType source"))
	}
	abstract, err := ParseBooleanAttr(BooleanAttr{Name: vocab.XSDAttrAbstract, Value: source.Abstract.Value, HasValue: source.Abstract.Present})
	if err != nil {
		return ComplexType{}, err
	}
	block, err := complexBlockMaskWithDefault(n, ctx.blockDefault)
	if err != nil {
		return ComplexType{}, err
	}
	ct := ComplexType{
		Name:        name,
		Content:     NoContentModel,
		Attrs:       NoAttributeUseSet,
		TextType:    NoSimpleType,
		ContentKind: contentKind,
		Abstract:    abstract,
		Base:        ComplexRef(c.rt.builtinIDs().AnyType),
		Derivation:  DerivationKindRestriction,
		Block:       block,
	}
	return ct, nil
}

func (c *compiler) compileDirectComplexType(n *schemaNode, ctx *schemaContext, ct ComplexType) (ComplexType, error) {
	modelNode := firstModelChild(n)
	content, err := c.compileDirectComplexModel(n, ctx)
	if err != nil {
		return ComplexType{}, err
	}
	contentKind, err := c.effectiveComplexContentKind(modelNode, ct.ContentKind)
	if err != nil {
		return ComplexType{}, err
	}
	content, err = c.canonicalComplexContentModel(content, contentKind)
	if err != nil {
		return ComplexType{}, err
	}
	ct.Content = content
	ct.ContentKind = contentKind
	attrs, err := c.compileAttributeUses(n, ctx, nil, NoWildcard, AttributeMergeDirect)
	if err != nil {
		return ComplexType{}, err
	}
	ct.Attrs = attrs
	return ct, nil
}

func (c *compiler) compileDirectComplexModel(n *schemaNode, ctx *schemaContext) (ContentModelID, error) {
	content := NoContentModel
	for child := range n.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		model, present, err := c.compileDirectComplexModelChild(child, ctx)
		if err != nil {
			return NoContentModel, err
		}
		if present {
			content = model
		}
	}
	return content, nil
}

func (c *compiler) compileDirectComplexModelChild(child *schemaNode, ctx *schemaContext) (ContentModelID, bool, error) {
	switch child.local {
	case vocab.XSDElemSequence, vocab.XSDElemChoice, vocab.XSDElemAll, vocab.XSDElemGroup:
		if err := validateModelOccurrence(child, c.limits); err != nil {
			return NoContentModel, false, err
		}
		model, err := c.compileModel(child, ctx)
		return model, true, err
	default:
		return NoContentModel, false, nil
	}
}

func (c *compiler) compileComplexContent(n *schemaNode, ctx *schemaContext, ct ComplexType, scope complexTypeScope) (ComplexType, error) {
	source, err := checkComplexContentSyntax(n)
	if err != nil {
		return ComplexType{}, err
	}
	rawKind, err := schemaComplexContentKind(n, ct.ContentKind)
	if err != nil {
		return ComplexType{}, err
	}
	contentKind, err := c.effectiveComplexContentKind(firstModelChild(source.node), rawKind)
	if err != nil {
		return ComplexType{}, err
	}
	return c.compileComplexContentDerivation(source.node, source.kind, ctx, ct, contentKind, scope)
}

func (c *compiler) compileComplexContentDerivation(child *schemaNode, kind ContentDerivationKind, ctx *schemaContext, ct ComplexType, contentKind ContentKind, scope complexTypeScope) (ComplexType, error) {
	baseID, base, err := c.complexContentBase(child, kind, ctx, scope)
	if err != nil {
		return ComplexType{}, err
	}
	extension := kind == ContentDerivationExtension
	if err := validateMixedDerivationAt(child, base, kind, contentKind); err != nil {
		return ComplexType{}, err
	}
	ct.Base = ComplexRef(baseID)
	if extension {
		if err := checkComplexContentExtensionChildren(child); err != nil {
			return ComplexType{}, err
		}
		return c.compileComplexContentExtension(child, ctx, ct, baseID, base, contentKind)
	}
	if err := checkComplexContentRestrictionChildren(child); err != nil {
		return ComplexType{}, err
	}
	return c.compileComplexContentRestriction(child, ctx, ct, base, contentKind)
}

func (c *compiler) complexContentBase(child *schemaNode, kind ContentDerivationKind, ctx *schemaContext, scope complexTypeScope) (ComplexTypeID, ComplexType, error) {
	baseQName, err := c.contentDerivationBaseQName(vocab.XSDElemComplexContent, kind, child, ctx)
	if err != nil {
		return NoComplexType, ComplexType{}, err
	}
	if c.compilingComplex[baseQName] && scope == complexTypeScopeGlobal {
		cycleErr := SchemaComponentCycleError(SchemaComponentComplexType, c.rt.formatName(baseQName))
		return NoComplexType, ComplexType{}, withSchemaCompileLocation(child, cycleErr)
	}
	baseID, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return NoComplexType, ComplexType{}, withSchemaCompileLocation(child, err)
	}
	return baseID, c.rt.complexType(baseID), nil
}

func (c *compiler) contentDerivationBaseQName(container string, kind ContentDerivationKind, child *schemaNode, ctx *schemaContext) (QName, error) {
	baseSource, ok := child.semantic.derivationBase()
	if !ok {
		return QName{}, withSchemaCompileLocation(child, xsderrors.InternalInvariant("derivation node has no typed derivation source"))
	}
	baseLex, present := baseSource.Lexical.Value, baseSource.Lexical.Present
	base := ContentDerivationBase{Container: container, Derivation: kind.String(), Lexical: baseLex, Present: present}
	if err := checkContentDerivationBase(child, base); err != nil {
		return QName{}, err
	}
	return c.resolveQNameChecked(child, ctx, base.Lexical)
}

func (c *compiler) compileComplexContentExtension(child *schemaNode, ctx *schemaContext, ct ComplexType, baseID ComplexTypeID, base ComplexType, contentKind ContentKind) (ComplexType, error) {
	if err := CheckComplexTypeFinalAllows(base.Final, DerivationExtension, ComplexTypeFinalBaseExtension); err != nil {
		return ComplexType{}, withSchemaCompileLocation(child, err)
	}
	ct.Derivation = DerivationKindExtension
	ct.ExplicitDerivation = true
	ct.Content = base.Content
	ct.TextType = base.TextType
	ct.ContentKind = complexExtensionContentKind(base, contentKind)
	if modelNode := firstModelChild(child); modelNode != nil {
		content, err := c.compileComplexExtensionModel(modelNode, ctx, baseID, base, contentKind)
		if err != nil {
			return ComplexType{}, err
		}
		ct.Content = content
	}
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcardForCompilation(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, AttributeMergeExtension)
	if err != nil {
		return ComplexType{}, err
	}
	ct.Attrs = attrs
	if contentKind == ContentEmpty || base.SimpleContent() {
		return ct, nil
	}
	ct.Content, err = c.canonicalComplexContentModel(ct.Content, ct.ContentKind)
	if err != nil {
		return ComplexType{}, err
	}
	return ct, nil
}

func complexExtensionContentKind(base ComplexType, contentKind ContentKind) ContentKind {
	if contentKind == ContentEmpty || base.SimpleContent() {
		return base.ContentKind
	}
	if base.Mixed() {
		return ContentMixed
	}
	return contentKind
}

func (c *compiler) compileComplexExtensionModel(modelNode *schemaNode, ctx *schemaContext, baseID ComplexTypeID, base ComplexType, contentKind ContentKind) (ContentModelID, error) {
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return NoContentModel, err
	}
	content, err := c.compileModel(modelNode, ctx)
	if err != nil {
		return NoContentModel, err
	}
	if contentKind == ContentEmpty {
		// Empty effective content inherits the base, but its source syntax and
		// references must still be checked before discarding the local model.
		return base.Content, nil
	}
	if err := ValidateComplexExtensionContentAdmission(ComplexExtensionContentAdmission{
		BaseSimpleContent: base.SimpleContent(),
		HasModelChild:     true,
	}); err != nil {
		return NoContentModel, withSchemaCompileLocation(modelNode, err)
	}
	if err := c.validateComplexExtensionModelAdmission(baseID, base, content, contentKind); err != nil {
		return NoContentModel, withSchemaCompileLocation(modelNode, err)
	}
	addAtModelNode := func(model ContentModel) (ContentModelID, error) {
		return c.addModelAt(model, modelNode)
	}
	return ExtendSequenceModel(&c.rt, addAtModelNode, base.Content, content)
}

func validateMixedDerivationAt(child *schemaNode, base ComplexType, derivation ContentDerivationKind, content ContentKind) error {
	if err := CheckComplexContentMixedDerivationBase(base, derivation, content); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (c *compiler) compileComplexContentRestriction(child *schemaNode, ctx *schemaContext, ct, base ComplexType, contentKind ContentKind) (ComplexType, error) {
	if err := CheckComplexTypeFinalAllows(base.Final, DerivationRestriction, ComplexTypeFinalBaseRestriction); err != nil {
		return ComplexType{}, withSchemaCompileLocation(child, err)
	}
	if err := CheckComplexContentRestrictionBase(base); err != nil {
		return ComplexType{}, withSchemaCompileLocation(child, err)
	}
	ct.Derivation = DerivationKindRestriction
	ct.ExplicitDerivation = true
	content, err := c.compileComplexRestrictionModel(child, ctx, ct)
	if err != nil {
		return ComplexType{}, err
	}
	ct.ContentKind = contentKind
	content, err = c.canonicalComplexContentModel(content, contentKind)
	if err != nil {
		return ComplexType{}, err
	}
	ct.Content = content
	baseUses, baseWildcard := c.rt.attributeUsesAndWildcardForCompilation(base.Attrs)
	attrs, err := c.compileAttributeUses(child, ctx, baseUses, baseWildcard, AttributeMergeRestriction)
	if err != nil {
		return ComplexType{}, err
	}
	ct.Attrs = attrs
	return ct, nil
}

func (c *compiler) compileComplexRestrictionModel(child *schemaNode, ctx *schemaContext, ct ComplexType) (ContentModelID, error) {
	modelNode := firstModelChild(child)
	if modelNode == nil {
		return c.addModel(ContentModel{Kind: ModelEmpty, Mixed: ct.Mixed()})
	}
	if err := validateModelOccurrence(modelNode, c.limits); err != nil {
		return NoContentModel, err
	}
	return c.compileModel(modelNode, ctx)
}

func (c *compiler) compileSimpleContent(n *schemaNode, ctx *schemaContext, ct ComplexType, scope complexTypeScope) (ComplexType, error) {
	source, err := c.resolveSimpleContentSource(n, ctx)
	if err != nil {
		return ComplexType{}, err
	}
	var textType SimpleTypeID
	if c.simpleTypeQNameKnown(source.base) {
		ct, textType, err = c.compileSimpleContentSimpleBase(source.child, source.kind, source.base, ct)
	} else {
		ct, textType, err = c.compileSimpleContentComplexBase(source.child, source.kind, source.base, ct, scope)
	}
	if err != nil {
		return ComplexType{}, err
	}
	derivation, err := c.compileSimpleContentDerivation(source, ctx, textType)
	if err != nil {
		return ComplexType{}, err
	}
	textType = derivation.textType
	inheritedUses, inheritedWildcard := c.rt.attributeUsesAndWildcardForCompilation(ct.Attrs)
	attrs, err := c.compileAttributeUses(source.child, ctx, inheritedUses, inheritedWildcard, derivation.mergeMode)
	if err != nil {
		return ComplexType{}, err
	}
	ct.Attrs = attrs
	ct.Content, err = c.addModel(ContentModel{Kind: ModelEmpty})
	if err != nil {
		return ComplexType{}, err
	}
	ct.TextType = textType
	// xs:simpleContent has no mixed attribute; ct.Mixed() carries mixed="true"
	// from the enclosing complexType element, which downstream complexContent
	// mixed-derivation checks read.
	if ct.Mixed() {
		ct.ContentKind = ContentSimpleMixed
	} else {
		ct.ContentKind = ContentSimple
	}
	ct.Derivation = derivation.kind
	ct.ExplicitDerivation = true
	return ct, nil
}

type simpleContentSource struct {
	child *schemaNode
	base  QName
	kind  ContentDerivationKind
}

type simpleContentDerivation struct {
	textType  SimpleTypeID
	mergeMode AttributeMergeMode
	kind      DerivationKind
}

func (c *compiler) resolveSimpleContentSource(n *schemaNode, ctx *schemaContext) (simpleContentSource, error) {
	syntax, err := checkSimpleContentSyntax(n)
	if err != nil {
		return simpleContentSource{}, err
	}
	baseSource, ok := syntax.node.semantic.derivationBase()
	if !ok {
		return simpleContentSource{}, withSchemaCompileLocation(syntax.node, xsderrors.InternalInvariant("derivation node has no typed derivation source"))
	}
	baseLexical, present := baseSource.Lexical.Value, baseSource.Lexical.Present
	baseAttribute := ContentDerivationBase{
		Container:  vocab.XSDElemSimpleContent,
		Derivation: syntax.kind.String(),
		Lexical:    baseLexical,
		Present:    present,
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

func (c *compiler) compileSimpleContentDerivation(source simpleContentSource, ctx *schemaContext, textType SimpleTypeID) (simpleContentDerivation, error) {
	if source.kind != ContentDerivationRestriction {
		if err := checkSimpleContentExtensionChildren(source.child); err != nil {
			return simpleContentDerivation{}, err
		}
		return simpleContentDerivation{
			textType:  textType,
			mergeMode: AttributeMergeExtension,
			kind:      DerivationKindExtension,
		}, nil
	}
	if err := checkSimpleContentRestrictionChildren(source.child); err != nil {
		return simpleContentDerivation{}, err
	}
	restricted, err := c.compileSimpleContentRestrictionType(source.child, ctx, textType)
	return simpleContentDerivation{
		textType:  restricted,
		mergeMode: AttributeMergeRestriction,
		kind:      DerivationKindRestriction,
	}, err
}

func (c *compiler) compileSimpleContentSimpleBase(child *schemaNode, kind ContentDerivationKind, baseQName QName, ct ComplexType) (ComplexType, SimpleTypeID, error) {
	if err := CheckSimpleContentSimpleBase(kind); err != nil {
		return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	simpleID, err := c.compileSimpleByQName(baseQName)
	if err != nil {
		return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if err := CheckSimpleBaseComplexExtensionFinalAllows(c.rt.simpleTypeFinalMask(simpleID)); err != nil {
		return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	ct.Base = SimpleRef(simpleID)
	return ct, simpleID, nil
}

func (c *compiler) compileSimpleContentComplexBase(child *schemaNode, kind ContentDerivationKind, baseQName QName, ct ComplexType, scope complexTypeScope) (ComplexType, SimpleTypeID, error) {
	if c.compilingComplex[baseQName] && scope == complexTypeScopeGlobal {
		err := SchemaComponentCycleError(SchemaComponentComplexType, c.rt.formatName(baseQName))
		return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if !c.complexTypeQNameKnown(baseQName) {
		return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, SimpleContentComplexBaseMissingError())
	}
	baseComplex, err := c.compileComplexByQName(baseQName)
	if err != nil {
		return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	base := c.rt.complexType(baseComplex)
	if err := CheckSimpleContentDerivationBase(c.contentAnalysis, base, kind); err != nil {
		return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
	}
	switch kind {
	case ContentDerivationNone:
		return ComplexType{}, NoSimpleType, xsderrors.InternalInvariant("simpleContent complex base derivation missing")
	case ContentDerivationExtension:
		if err := CheckComplexTypeFinalAllows(base.Final, DerivationExtension, ComplexTypeFinalBaseExtension); err != nil {
			return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
		}
	case ContentDerivationRestriction:
		if err := CheckComplexTypeFinalAllows(base.Final, DerivationRestriction, ComplexTypeFinalBaseRestriction); err != nil {
			return ComplexType{}, NoSimpleType, withSchemaCompileLocation(child, err)
		}
	}
	ct.Base = ComplexRef(baseComplex)
	ct.Attrs = base.Attrs
	return ct, base.TextType, nil
}

func (c *compiler) compileSimpleContentRestrictionType(child *schemaNode, ctx *schemaContext, baseTextType SimpleTypeID) (SimpleTypeID, error) {
	textType := baseTextType
	facetChildren := facetChildren(child)
	if stNode := child.firstXS(vocab.XSDElemSimpleType); stNode != nil {
		simpleID, err := c.compileAnonymousSimple(stNode, ctx)
		if err != nil {
			return NoSimpleType, err
		}
		textType = simpleID
	}
	if err := CheckSimpleContentRestrictionTextTypePresent(textType); err != nil {
		return NoSimpleType, withSchemaCompileLocation(child, err)
	}
	if len(facetChildren) != 0 {
		simpleID, err := c.compileSimpleContentFacetRestriction(facetChildren, textType)
		if err != nil {
			return NoSimpleType, err
		}
		textType = simpleID
	}
	if err := CheckSimpleContentRestrictionTextType(&c.rt, textType, baseTextType, c.rt.spendTypeDerivationWork); err != nil {
		return NoSimpleType, withSchemaCompileLocation(child, err)
	}
	return textType, nil
}

func (c *compiler) compileSimpleContentFacetRestriction(facetChildren []*schemaNode, baseID SimpleTypeID) (SimpleTypeID, error) {
	if err := CheckSimpleRestrictionBase(baseID, c.rt.builtinIDs().AnySimpleType); err != nil {
		return NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinalMask(baseID), DerivationRestriction, SimpleTypeFinalBaseRestriction); err != nil {
		return NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$simple%d", c.rt.SimpleTypeCount()))
	if err != nil {
		return NoSimpleType, err
	}
	st := c.rt.derivedSimpleType(baseID, q)
	if err = c.compileSimpleContentFacetStep(facetChildren, baseID, &st); err != nil {
		return NoSimpleType, withSchemaCompileLocation(facetChildren[0], err)
	}
	if st.ValueSpec.Variety == SimpleVarietyUnion {
		if chargeErr := c.chargeSimpleUnionMemberEntries(facetChildren[0], len(st.ValueSpec.Union)); chargeErr != nil {
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

func (c *compiler) compileSimpleContentFacetStep(facetChildren []*schemaNode, baseID SimpleTypeID, st *SimpleType) error {
	if c.rt.simpleTypeUnavailable[baseID] {
		return c.validateUnavailableFacetChildren(facetChildren, st, baseID, facetChildModeExplicitList)
	}
	return c.compileFacetList(facetChildren, st, baseID, baseID)
}

func facetChildren(n *schemaNode) []*schemaNode {
	var out []*schemaNode
	for _, child := range typedXSDChildren(n) {
		if IsFacetLocal(child.local) {
			out = append(out, child)
		}
	}
	return out
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
