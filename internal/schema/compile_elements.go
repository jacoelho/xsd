package schema

import (
	"encoding/xml"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	elementDefaultConstraintLabel = "element default"
	elementFixedConstraintLabel   = "element fixed"
)

func (c *compiler) compileElementParticle(n *schemaNode, ctx *schemaSetDocument) (contentParticle, error) {
	id, err := c.compileElementParticleDeclaration(n, ctx)
	if err != nil {
		return contentParticle{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return contentParticle{}, err
	}
	return elementParticle(id, occurs), nil
}

func (c *compiler) compileElementParticleDeclaration(n *schemaNode, ctx *schemaSetDocument) (ElementID, error) {
	source := semanticSource[*schemaElementSource](&n.semantic)
	if source == nil || !source.Ref.Lexical.Present {
		return c.compileLocalElement(n, ctx)
	}
	if err := checkElementRefAttributes(n); err != nil {
		return 0, err
	}
	if err := checkElementRefChildren(n); err != nil {
		return 0, err
	}
	q, err := c.referenceQName(n, ctx, source.Ref.Name)
	if err != nil {
		return 0, err
	}
	id, err := c.compileElementByQName(q)
	return id, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileElementByQName(q QName) (ElementID, error) {
	raw, exists := c.elementComponents[q]
	var source *schemaNode
	if exists {
		source = raw.sourceNode()
	}
	if err := c.spendComponentDependency(source); err != nil {
		return 0, err
	}
	if id, ok := c.rt.GlobalElements[q]; ok {
		return id, nil
	}
	label := c.rt.formatName(q)
	if !exists {
		return 0, schemaComponentMissingError(schemaComponentElement, label)
	}
	leave, err := c.enterComponent(raw.sourceNode())
	if err != nil {
		return 0, err
	}
	defer leave()
	id, err := c.registerGlobalElement(q, elementDecl{Name: q, Type: ComplexRef(c.rt.AnyType)})
	if err != nil {
		return 0, err
	}
	decl, pending, err := c.compileElementDecl(raw.sourceNode(), raw.ctx, q)
	if err != nil {
		return 0, err
	}
	c.completeElement(id, decl)
	c.addPendingElementConstraint(id, raw.sourceNode(), pending)
	return id, nil
}

func (c *compiler) compileLocalElement(n *schemaNode, ctx *schemaSetDocument) (ElementID, error) {
	if id, ok := c.localDone[nodeKey(n, ctx)]; ok {
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
	name := schemaElementName(n)
	ns := ""
	form, hasForm := schemaElementForm(n)
	qualified, err := parseElementFormAttr(formAttr{
		Value:            form,
		HasValue:         hasForm,
		DefaultQualified: ctx.doc.defaults.ElementQualified,
	})
	if err != nil {
		return 0, withSchemaCompileLocation(n, err)
	}
	if qualified {
		ns = ctx.effectiveTargetNS
	}
	q, err := c.rt.internQName(ns, name)
	if err != nil {
		return 0, err
	}
	id, err := c.addElement(elementDecl{Name: q, Type: ComplexRef(c.rt.AnyType)})
	if err != nil {
		return 0, err
	}
	c.localDone[nodeKey(n, ctx)] = id
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
	kind    declarationValueConstraint
}

type pendingElementConstraint struct {
	node    *schemaNode
	lexical string
	element ElementID
	kind    declarationValueConstraint
}

func (c *compiler) compileElementDecl(n *schemaNode, ctx *schemaSetDocument, q QName) (elementDecl, elementConstraintDraft, error) {
	if err := checkElementDeclarationChildren(n); err != nil {
		return elementDecl{}, elementConstraintDraft{}, err
	}
	identityNodes := identityConstraintNodes(n)
	identityIDs, err := c.declareIdentityConstraints(identityNodes, ctx)
	if err != nil {
		return elementDecl{}, elementConstraintDraft{}, err
	}
	properties, err := c.compileElementProperties(n, ctx)
	if err != nil {
		return elementDecl{}, elementConstraintDraft{}, err
	}
	decl := elementDecl{
		Name:      q,
		Type:      properties.typ,
		Nillable:  properties.nillable,
		Abstract:  properties.abstract,
		SubstHead: NoElement,
	}
	if maskErr := applyElementDerivationMasks(n, ctx, &decl); maskErr != nil {
		return elementDecl{}, elementConstraintDraft{}, maskErr
	}
	draft, err := compileElementConstraintDraft(n)
	if err != nil {
		return elementDecl{}, elementConstraintDraft{}, err
	}
	if err := c.compileDeclaredIdentityConstraints(identityNodes, identityIDs, ctx); err != nil {
		return elementDecl{}, elementConstraintDraft{}, err
	}
	decl.Identity = identityIDs
	return decl, draft, nil
}

type compiledElementProperties struct {
	typ      TypeID
	nillable bool
	abstract bool
}

func (c *compiler) compileElementProperties(n *schemaNode, ctx *schemaSetDocument) (compiledElementProperties, error) {
	source := semanticSource[*schemaElementSource](&n.semantic)
	if source == nil {
		return compiledElementProperties{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("element node has no typed element source"))
	}
	nillable, err := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrNillable, Value: source.Nillable.Value, HasValue: source.Nillable.Present})
	abstract, abstractErr := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrAbstract, Value: source.Abstract.Value, HasValue: source.Abstract.Present})
	if err != nil {
		return compiledElementProperties{}, err
	}
	if abstractErr != nil {
		return compiledElementProperties{}, abstractErr
	}
	typ, err := c.compileElementDeclType(n, ctx)
	if err != nil {
		return compiledElementProperties{}, err
	}
	if simple, ok := typ.Simple(); ok {
		if err := c.admitNotationTypeUse(n, simple); err != nil {
			return compiledElementProperties{}, err
		}
	}
	return compiledElementProperties{typ: typ, nillable: nillable, abstract: abstract}, nil
}

func (c *compiler) compileElementDeclType(n *schemaNode, ctx *schemaSetDocument) (TypeID, error) {
	if source := semanticSource[*schemaElementSource](&n.semantic); source != nil && source.Type.Lexical.Present {
		return c.compileElementTypeAttribute(n, ctx, source.Type.Name)
	}
	if simple := n.firstXS(vocab.XSDElemSimpleType); simple != nil {
		id, err := c.compileAnonymousSimple(simple, ctx)
		return SimpleRef(id), err
	}
	if complexType := n.firstXS(vocab.XSDElemComplexType); complexType != nil {
		id, err := c.reserveAnonymousComplexType(complexType, ctx)
		return ComplexRef(id), err
	}
	return ComplexRef(c.rt.AnyType), nil
}

func applyElementDerivationMasks(n *schemaNode, ctx *schemaSetDocument, decl *elementDecl) error {
	block, err := derivationMaskWithDefaultChecked(n, semanticSource[*schemaElementSource](&n.semantic).Block, ctx.doc.defaults.BlockDefault, elementBlockDerivation())
	if err != nil {
		return err
	}
	decl.Block = block
	final, err := derivationMaskWithDefaultChecked(n, semanticSource[*schemaElementSource](&n.semantic).Final, ctx.doc.defaults.FinalDefault, elementFinalDerivation())
	if err != nil {
		return err
	}
	decl.Final = final
	return nil
}

func compileElementConstraintDraft(n *schemaNode) (elementConstraintDraft, error) {
	source := semanticSource[*schemaElementSource](&n.semantic)
	if source == nil {
		return elementConstraintDraft{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("element node has no typed element source"))
	}
	defaultLexical, hasDefault := source.Default.Value, source.Default.Present
	fixedLexical, hasFixed := source.Fixed.Value, source.Fixed.Present
	var lexical string
	var kind declarationValueConstraint
	switch {
	case hasDefault && hasFixed:
		kind = declarationValueConstraintConflict
	case hasDefault:
		kind = declarationValueConstraintDefault
		lexical = defaultLexical
	case hasFixed:
		kind = declarationValueConstraintFixed
		lexical = fixedLexical
	}
	if err := validateElementDeclValueConstraintAtNode(n, kind); err != nil {
		return elementConstraintDraft{}, err
	}
	return elementConstraintDraft{lexical: lexical, kind: kind}, nil
}

func (c *compiler) addPendingElementConstraint(id ElementID, n *schemaNode, draft elementConstraintDraft) {
	if draft.kind == declarationValueConstraintNone {
		return
	}
	c.pendingElementConstraints = append(c.pendingElementConstraints, pendingElementConstraint{
		node: n, lexical: draft.lexical, element: id, kind: draft.kind,
	})
}

func (c *compiler) compileElementTypeAttribute(n *schemaNode, ctx *schemaSetDocument, name xml.Name) (TypeID, error) {
	typeQName, err := c.referenceQName(n, ctx, name)
	if err != nil {
		return TypeID{}, err
	}
	if c.typeQNameKnown(typeQName) {
		return c.resolveTypeQName(typeQName)
	}
	if !c.typeQNameMayBeUnavailable(typeQName) {
		missingErr := schemaComponentMissingError(schemaComponentType, c.rt.formatName(typeQName))
		return TypeID{}, withSchemaCompileLocation(n, missingErr)
	}
	missing, err := c.missingSimpleType()
	if err != nil {
		return TypeID{}, err
	}
	return SimpleRef(missing), nil
}

func (c *compiler) validateElementValueConstraints(decl *elementDecl, n *schemaNode, unavailable []bool) error {
	if decl.Default == nil && decl.Fixed == nil {
		return nil
	}
	simpleID, err := elementValueConstraintType(&c.rt, c.contentAnalysis, decl.Type)
	if err != nil {
		return schemaValueError(c.limits, xsderrors.CodeSchemaInvalidAttribute, "invalid element value constraint", err)
	}
	if simpleID == NoSimpleType {
		applyMixedElementConstraints(decl, n.namespace.Lookup)
		return nil
	}
	unavailableType, err := prepareElementConstraintType(decl, simpleID, unavailable)
	if err != nil || unavailableType {
		return err
	}
	if err = validateElementDeclValueConstraintRuntime(&c.rt, simpleID, declarationValueConstraintOf(decl.Default, decl.Fixed)); err != nil {
		return schemaValueError(c.limits, xsderrors.CodeSchemaInvalidAttribute, "invalid element value constraint", err)
	}
	constraint := decl.Default
	label := elementDefaultConstraintLabel
	if constraint == nil {
		constraint = decl.Fixed
		label = elementFixedConstraintLabel
	}
	resolve := schemaQNameResolver(n)
	validated, err := c.validateValueConstraint(simpleID, constraint.Lexical, resolve, n.namespace.Lookup, decl.Name, label)
	if err != nil {
		return err
	}
	if decl.Default != nil {
		decl.Default = validated
		return nil
	}
	decl.Fixed = validated
	return nil
}

func applyMixedElementConstraints(decl *elementDecl, lookup func(string) (string, bool)) {
	if decl.Default != nil {
		decl.Default = mixedContentConstraint(decl.Default.Lexical, lookup)
	}
	if decl.Fixed != nil {
		decl.Fixed = mixedContentConstraint(decl.Fixed.Lexical, lookup)
	}
}

func prepareElementConstraintType(decl *elementDecl, simpleID SimpleTypeID, unavailable []bool) (bool, error) {
	if !validSimpleTypeID(simpleID, len(unavailable)) {
		return false, xsderrors.InternalInvariant("element value constraint references invalid simple type")
	}
	if !unavailable[simpleID] {
		return false, nil
	}
	decl.Default = nil
	decl.Fixed = nil
	return true, nil
}

// mixedContentConstraint builds the constraint for an emptiable mixed-content
// element, whose default or fixed text is used verbatim: the lexical form is
// its own canonical form and the value is untyped.
func mixedContentConstraint(lexical string, lookup func(string) (string, bool)) *valueConstraint {
	return &valueConstraint{
		Lexical:      lexical,
		Canonical:    lexical,
		Value:        valuepkg.NewUntypedValue(lexical),
		qnameContext: newValueConstraintQNameContext(lexical, lookup),
	}
}
