package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) compileElementParticle(n *rawNode, ctx *schemaContext) (runtime.Particle, error) {
	id, err := c.compileElementParticleDeclaration(n, ctx)
	if err != nil {
		return runtime.Particle{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return runtime.Particle{}, err
	}
	return runtime.ElementParticle(id, occurs), nil
}

func (c *compiler) compileElementParticleDeclaration(n *rawNode, ctx *schemaContext) (runtime.ElementID, error) {
	ref, referenced := n.attr(vocab.XSDAttrRef)
	if !referenced {
		return c.compileLocalElement(n, ctx)
	}
	if err := checkElementRefAttributes(n); err != nil {
		return 0, err
	}
	if err := checkElementRefChildren(n); err != nil {
		return 0, err
	}
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return 0, err
	}
	id, err := c.compileElementByQName(q)
	return id, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileElementByQName(q runtime.QName) (runtime.ElementID, error) {
	raw, exists := c.elementRaw[q]
	var source *rawNode
	if exists {
		source = raw.node
	}
	if err := c.spendComponentDependency(source); err != nil {
		return 0, err
	}
	if id, ok := c.elementDone[q]; ok {
		return id, nil
	}
	label := c.rt.formatName(q)
	if !exists {
		return 0, SchemaComponentMissingError(SchemaComponentElement, label)
	}
	leave, err := c.enterComponent(raw.node)
	if err != nil {
		return 0, err
	}
	defer leave()
	id, err := c.registerGlobalElement(q, runtime.ElementDecl{Name: q, Type: runtime.ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return 0, err
	}
	c.elementDone[q] = id
	decl, pending, err := c.compileElementDecl(raw.node, raw.ctx, q)
	if err != nil {
		return 0, err
	}
	c.completeElement(id, decl)
	c.addPendingElementConstraint(id, raw.node, pending)
	return id, nil
}

func (c *compiler) compileLocalElement(n *rawNode, ctx *schemaContext) (runtime.ElementID, error) {
	if id, ok := c.localDone[n]; ok {
		return id, nil
	}
	if err := c.spendComponentDependency(n); err != nil {
		return 0, err
	}
	leave, err := c.enterComponent(n)
	if err != nil {
		return 0, err
	}
	defer leave()
	if err = checkLocalElementAttributes(n); err != nil {
		return 0, err
	}
	if err = checkLocalElementSource(n); err != nil {
		return 0, err
	}
	name, _ := n.attr(vocab.XSDAttrName)
	ns := ""
	form, hasForm := n.attr(vocab.XSDAttrForm)
	qualified, err := ParseElementFormAttr(FormAttr{
		Value:            form,
		HasValue:         hasForm,
		DefaultQualified: ctx.elementQualified,
	})
	if err != nil {
		return 0, withSchemaCompileLocation(n, err)
	}
	if qualified {
		ns = ctx.targetNS
	}
	q, err := c.rt.internQName(ns, name)
	if err != nil {
		return 0, err
	}
	id, err := c.addElement(runtime.ElementDecl{Name: q, Type: runtime.ComplexRef(c.rt.builtinIDs().AnyType)})
	if err != nil {
		return 0, err
	}
	c.localDone[n] = id
	decl, pending, err := c.compileElementDecl(n, ctx, q)
	if err != nil {
		return 0, err
	}
	c.completeElement(id, decl)
	c.addPendingElementConstraint(id, n, pending)
	return id, nil
}

type elementConstraintDraft struct {
	lexical string
	kind    runtime.DeclarationValueConstraint
}

type pendingElementConstraint struct {
	node    *rawNode
	lexical string
	element runtime.ElementID
	kind    runtime.DeclarationValueConstraint
}

func (c *compiler) compileElementDecl(n *rawNode, ctx *schemaContext, q runtime.QName) (runtime.ElementDecl, elementConstraintDraft, error) {
	c.elementDepth++
	defer func() { c.elementDepth-- }()
	if err := checkElementDeclarationChildren(n); err != nil {
		return runtime.ElementDecl{}, elementConstraintDraft{}, err
	}
	identityNodes := identityConstraintNodes(n)
	identityIDs, err := c.declareIdentityConstraints(identityNodes, ctx)
	if err != nil {
		return runtime.ElementDecl{}, elementConstraintDraft{}, err
	}
	properties, err := c.compileElementProperties(n, ctx)
	if err != nil {
		return runtime.ElementDecl{}, elementConstraintDraft{}, err
	}
	decl := runtime.ElementDecl{
		Name:      q,
		Type:      properties.typ,
		Nillable:  properties.nillable,
		Abstract:  properties.abstract,
		SubstHead: runtime.NoElement,
	}
	if maskErr := applyElementDerivationMasks(n, ctx, &decl); maskErr != nil {
		return runtime.ElementDecl{}, elementConstraintDraft{}, maskErr
	}
	draft, err := compileElementConstraintDraft(n)
	if err != nil {
		return runtime.ElementDecl{}, elementConstraintDraft{}, err
	}
	if err := c.compileDeclaredIdentityConstraints(identityNodes, identityIDs, ctx); err != nil {
		return runtime.ElementDecl{}, elementConstraintDraft{}, err
	}
	decl.Identity = identityIDs
	return decl, draft, nil
}

type compiledElementProperties struct {
	typ      runtime.TypeID
	nillable bool
	abstract bool
}

func (c *compiler) compileElementProperties(n *rawNode, ctx *schemaContext) (compiledElementProperties, error) {
	nillable, err := schemaBoolAttr(n, vocab.XSDAttrNillable)
	if err != nil {
		return compiledElementProperties{}, err
	}
	abstract, err := schemaBoolAttr(n, vocab.XSDAttrAbstract)
	if err != nil {
		return compiledElementProperties{}, err
	}
	typ, err := c.compileElementDeclType(n, ctx)
	return compiledElementProperties{typ: typ, nillable: nillable, abstract: abstract}, err
}

func (c *compiler) compileElementDeclType(n *rawNode, ctx *schemaContext) (runtime.TypeID, error) {
	if typeLexical, ok := n.attr(vocab.XSDAttrType); ok {
		return c.compileElementTypeAttribute(n, ctx, typeLexical)
	}
	if simple := n.firstXS(vocab.XSDElemSimpleType); simple != nil {
		id, err := c.compileAnonymousSimple(simple, ctx)
		return runtime.SimpleRef(id), err
	}
	if complexType := n.firstXS(vocab.XSDElemComplexType); complexType != nil {
		id, err := c.compileAnonymousComplex(complexType, ctx)
		return runtime.ComplexRef(id), err
	}
	return runtime.ComplexRef(c.rt.builtinIDs().AnyType), nil
}

func applyElementDerivationMasks(n *rawNode, ctx *schemaContext, decl *runtime.ElementDecl) error {
	block, err := derivationMaskWithDefaultChecked(n, ctx.blockDefault, elementBlockDerivation())
	if err != nil {
		return err
	}
	decl.Block = block
	final, err := derivationMaskWithDefaultChecked(n, ctx.finalDefault, elementFinalDerivation())
	if err != nil {
		return err
	}
	decl.Final = final
	return nil
}

func compileElementConstraintDraft(n *rawNode) (elementConstraintDraft, error) {
	defaultLexical, hasDefault := n.attr(vocab.XSDAttrDefault)
	fixedLexical, hasFixed := n.attr(vocab.XSDAttrFixed)
	var lexical string
	var kind runtime.DeclarationValueConstraint
	switch {
	case hasDefault && hasFixed:
		kind = runtime.DeclarationValueConstraintConflict
	case hasDefault:
		kind = runtime.DeclarationValueConstraintDefault
		lexical = defaultLexical
	case hasFixed:
		kind = runtime.DeclarationValueConstraintFixed
		lexical = fixedLexical
	}
	if err := validateElementDeclValueConstraintAtNode(n, kind); err != nil {
		return elementConstraintDraft{}, err
	}
	return elementConstraintDraft{lexical: lexical, kind: kind}, nil
}

func (c *compiler) addPendingElementConstraint(id runtime.ElementID, n *rawNode, draft elementConstraintDraft) {
	if draft.kind == runtime.DeclarationValueConstraintNone {
		return
	}
	c.pendingElementConstraints = append(c.pendingElementConstraints, pendingElementConstraint{
		node: n, lexical: draft.lexical, element: id, kind: draft.kind,
	})
}

func (c *compiler) compileElementTypeAttribute(n *rawNode, ctx *schemaContext, typeLex string) (runtime.TypeID, error) {
	typeQName, err := c.resolveQNameChecked(n, ctx, typeLex)
	if err != nil {
		return runtime.TypeID{}, err
	}
	if c.typeQNameKnown(typeQName) {
		return c.resolveTypeQName(typeQName)
	}
	if !c.typeQNameMayBeUnavailable(typeQName) {
		missingErr := SchemaComponentMissingError(SchemaComponentType, c.rt.formatName(typeQName))
		return runtime.TypeID{}, withSchemaCompileLocation(n, missingErr)
	}
	missing, err := c.missingSimpleType()
	if err != nil {
		return runtime.TypeID{}, err
	}
	return runtime.SimpleRef(missing), nil
}

func (c *compiler) validateElementValueConstraints(decl *runtime.ElementDecl, n *rawNode, unavailable []bool) error {
	if decl.Default == nil && decl.Fixed == nil {
		return nil
	}
	simpleID, err := runtime.ElementValueConstraintType(&c.rt, c.contentAnalysis, decl.Type)
	if err != nil {
		return ElementValueConstraintTypeError(err)
	}
	if simpleID == runtime.NoSimpleType {
		applyMixedElementConstraints(decl)
		return nil
	}
	unavailableType, err := prepareElementConstraintType(decl, simpleID, unavailable)
	if err != nil || unavailableType {
		return err
	}
	if err := runtime.ValidateElementDeclValueConstraintRuntime(&c.rt, simpleID, runtime.DeclarationValueConstraintOf(decl.Default, decl.Fixed)); err != nil {
		return ElementValueConstraintRuntimeError(err)
	}
	resolve := schemaQNameResolver(n)
	if err := c.validateElementConstraint(&decl.Default, simpleID, decl, resolve, "element default"); err != nil {
		return err
	}
	return c.validateElementConstraint(&decl.Fixed, simpleID, decl, resolve, "element fixed")
}

func applyMixedElementConstraints(decl *runtime.ElementDecl) {
	if decl.Default != nil {
		decl.Default = mixedContentConstraint(decl.Default.Lexical)
	}
	if decl.Fixed != nil {
		decl.Fixed = mixedContentConstraint(decl.Fixed.Lexical)
	}
}

func prepareElementConstraintType(decl *runtime.ElementDecl, simpleID runtime.SimpleTypeID, unavailable []bool) (bool, error) {
	if !runtime.ValidSimpleTypeID(simpleID, len(unavailable)) {
		return false, xsderrors.InternalInvariant("element value constraint references invalid simple type")
	}
	if !unavailable[simpleID] {
		return false, nil
	}
	decl.Default = nil
	decl.Fixed = nil
	return true, nil
}

func (c *compiler) validateElementConstraint(constraint **runtime.ValueConstraint, simpleID runtime.SimpleTypeID, decl *runtime.ElementDecl, resolve runtime.ResolveQNameParts, label string) error {
	if *constraint == nil {
		return nil
	}
	validated, err := c.validateValueConstraint(simpleID, (*constraint).Lexical, resolve, decl.Name, label)
	if err != nil {
		return err
	}
	*constraint = validated
	return nil
}

// mixedContentConstraint builds the constraint for an emptiable mixed-content
// element, whose default or fixed text is used verbatim: the lexical form is
// its own canonical form and the value is untyped.
func mixedContentConstraint(lexical string) *runtime.ValueConstraint {
	return &runtime.ValueConstraint{
		Lexical:   lexical,
		Canonical: lexical,
		Value:     runtime.SimpleValue{Canonical: lexical, Type: runtime.NoSimpleType},
	}
}
