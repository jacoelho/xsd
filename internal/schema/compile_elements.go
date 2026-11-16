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

func (c *compiler) compileElementParticle(ctx *schemaSetDocument, n *schemaNode) (contentParticle, error) {
	id, err := c.compileElementParticleDeclaration(ctx, n)
	if err != nil {
		return contentParticle{}, err
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return contentParticle{}, err
	}
	return elementParticle(id, occurs), nil
}

func (c *compiler) compileElementParticleDeclaration(ctx *schemaSetDocument, n *schemaNode) (ElementID, error) {
	source := semanticSource[*schemaElementSource](&n.semantic)
	if source == nil || !source.Ref.Lexical.Present {
		return c.compileLocalElement(ctx, n)
	}
	if err := checkElementRefAttributes(n); err != nil {
		return 0, err
	}
	if err := checkElementRefChildren(n); err != nil {
		return 0, err
	}
	q, err := c.referenceQName(ctx, n, source.Ref.Name)
	if err != nil {
		return 0, err
	}
	id, err := c.compileElementByQName(q)
	return id, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileElementByQName(q QName) (result ElementID, err error) {
	raw, exists := c.elementComponents[q]
	var source *schemaNode
	if exists {
		source = raw.sourceNode()
	}
	if err = c.spendComponentDependency(source); err != nil {
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
	defer leave(&err)
	id, err := c.registerGlobalElement(q, elementDecl{Name: q, Type: ComplexRef(c.rt.AnyType)})
	if err != nil {
		return 0, err
	}
	decl, pending, err := c.compileElementDecl(raw.ctx, raw.sourceNode(), q)
	if err != nil {
		return 0, err
	}
	c.completeElement(id, decl)
	c.addPendingElementConstraint(id, raw.sourceNode(), pending)
	return id, nil
}

func (c *compiler) compileLocalElement(ctx *schemaSetDocument, n *schemaNode) (result ElementID, err error) {
	if id, ok := c.localDone[nodeKey(ctx, n)]; ok {
		return id, nil
	}
	if err = c.spendComponentDependency(n); err != nil {
		return 0, err
	}
	leave, err := c.enterComponent(n)
	if err != nil {
		return 0, err
	}
	defer leave(&err)
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
	c.localDone[nodeKey(ctx, n)] = id
	decl, pending, err := c.compileElementDecl(ctx, n, q)
	if err != nil {
		return 0, err
	}
	c.completeElement(id, decl)
	c.addPendingElementConstraint(id, n, pending)
	return id, nil
}

// pendingElementConstraint is an unvalidated element value constraint that
// waits for the declaration's type to finalize.
type pendingElementConstraint struct {
	node       *schemaNode
	constraint *valueConstraint
	element    ElementID
}

func (c *compiler) compileElementDecl(ctx *schemaSetDocument, n *schemaNode, q QName) (elementDecl, *valueConstraint, error) {
	if err := checkElementDeclarationChildren(n); err != nil {
		return elementDecl{}, nil, err
	}
	identityNodes := identityConstraintNodes(n)
	identityIDs, err := c.declareIdentityConstraints(ctx, identityNodes)
	if err != nil {
		return elementDecl{}, nil, err
	}
	properties, err := c.compileElementProperties(ctx, n)
	if err != nil {
		return elementDecl{}, nil, err
	}
	decl := elementDecl{
		Name:      q,
		Type:      properties.typ,
		Nillable:  properties.nillable,
		Abstract:  properties.abstract,
		SubstHead: NoElement,
	}
	if maskErr := applyElementDerivationMasks(ctx, n, &decl); maskErr != nil {
		return elementDecl{}, nil, maskErr
	}
	constraint, err := compileElementConstraintSource(n)
	if err != nil {
		return elementDecl{}, nil, err
	}
	if err := c.compileDeclaredIdentityConstraints(ctx, identityNodes, identityIDs); err != nil {
		return elementDecl{}, nil, err
	}
	decl.Identity = identityIDs
	return decl, constraint, nil
}

type compiledElementProperties struct {
	typ      TypeID
	nillable bool
	abstract bool
}

func (c *compiler) compileElementProperties(ctx *schemaSetDocument, n *schemaNode) (compiledElementProperties, error) {
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
	typ, err := c.compileElementDeclType(ctx, n)
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

func (c *compiler) compileElementDeclType(ctx *schemaSetDocument, n *schemaNode) (TypeID, error) {
	if source := semanticSource[*schemaElementSource](&n.semantic); source != nil && source.Type.Lexical.Present {
		return c.compileElementTypeAttribute(ctx, n, source.Type.Name)
	}
	if simple := n.firstXS(vocab.XSDElemSimpleType); simple != nil {
		id, err := c.compileAnonymousSimple(ctx, simple)
		return SimpleRef(id), err
	}
	if complexType := n.firstXS(vocab.XSDElemComplexType); complexType != nil {
		id, err := c.reserveAnonymousComplexType(ctx, complexType)
		return ComplexRef(id), err
	}
	return ComplexRef(c.rt.AnyType), nil
}

func applyElementDerivationMasks(ctx *schemaSetDocument, n *schemaNode, decl *elementDecl) error {
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

func compileElementConstraintSource(n *schemaNode) (*valueConstraint, error) {
	source := semanticSource[*schemaElementSource](&n.semantic)
	if source == nil {
		return nil, withSchemaCompileLocation(n, xsderrors.InternalInvariant("element node has no typed element source"))
	}
	constraint, kind := sourceValueConstraint(source.Default, source.Fixed)
	if err := validateElementDeclValueConstraintAtNode(n, kind); err != nil {
		return nil, err
	}
	return constraint, nil
}

func (c *compiler) addPendingElementConstraint(id ElementID, n *schemaNode, constraint *valueConstraint) {
	if constraint == nil {
		return
	}
	c.pendingElementConstraints = append(c.pendingElementConstraints, pendingElementConstraint{
		node: n, constraint: constraint, element: id,
	})
}

func (c *compiler) compileElementTypeAttribute(ctx *schemaSetDocument, n *schemaNode, name xml.Name) (TypeID, error) {
	typeQName, err := c.referenceQName(ctx, n, name)
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
	constraint := decl.ValueConstraint
	if constraint == nil {
		return nil
	}
	simpleID, err := elementValueConstraintType(&c.rt, c.contentAnalysis, decl.Type)
	if err != nil {
		return schemaValueError(c.limits, xsderrors.CodeSchemaInvalidAttribute, "invalid element value constraint", err)
	}
	if simpleID == NoSimpleType {
		decl.ValueConstraint = mixedContentConstraint(constraint, n.namespace.Lookup)
		return nil
	}
	if !validSimpleTypeID(simpleID, len(unavailable)) {
		return xsderrors.InternalInvariant("element value constraint references invalid simple type")
	}
	if unavailable[simpleID] {
		decl.ValueConstraint = nil
		return nil
	}
	if err = validateElementDeclValueConstraintRuntime(&c.rt, simpleID, constraint); err != nil {
		return schemaValueError(c.limits, xsderrors.CodeSchemaInvalidAttribute, "invalid element value constraint", err)
	}
	label := elementDefaultConstraintLabel
	if constraint.Fixed {
		label = elementFixedConstraintLabel
	}
	validated, err := c.validateValueConstraint(simpleID, constraint, schemaQNameResolver(n), n.namespace.Lookup, decl.Name, label)
	if err != nil {
		return err
	}
	decl.ValueConstraint = validated
	return nil
}

// mixedContentConstraint builds the constraint for an emptiable mixed-content
// element, whose default or fixed text is used verbatim: the lexical form is
// its own canonical form and the value is untyped.
func mixedContentConstraint(constraint *valueConstraint, lookup func(string) (string, bool)) *valueConstraint {
	lexical := constraint.Lexical
	return &valueConstraint{
		Lexical:      lexical,
		Canonical:    lexical,
		Value:        valuepkg.NewUntypedValue(lexical),
		qnameContext: newValueConstraintQNameContext(lexical, lookup),
		Fixed:        constraint.Fixed,
	}
}
