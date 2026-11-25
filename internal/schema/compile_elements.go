package schema

import (
	"encoding/xml"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// compileElementParticle compiles the local xs:element or element reference
// n. It reports false when n has minOccurs and maxOccurs 0: the item then
// corresponds to no particle and no element declaration (Structures §3.3.2),
// so the declaration's own ref and type are neither resolved nor
// import-authorized and its value constraint is not checked. The definitions
// nested in it keep their own mapping and are compiled (compileAbsentElement).
func (c *compiler) compileElementParticle(ctx *schemaSetDocument, n *schemaNode) (contentParticle, bool, error) {
	occurs, err := particleOccurrence(n)
	if err != nil {
		return contentParticle{}, false, err
	}
	if occurs.IsAbsent() {
		return contentParticle{}, false, c.compileAbsentElement(ctx, n)
	}
	id, err := c.compileElementParticleDeclaration(ctx, n)
	if err != nil {
		return contentParticle{}, false, err
	}
	return elementParticle(id, occurs), true, nil
}

// compileAbsentElement compiles the definitions nested in the zero-count
// xs:element n: its inline type definition, which maps on its own as for a
// prohibited attribute (I-008), and its identity constraints, which map to
// identity-constraint definitions whatever their parent maps to (Structures
// §3.11.2). No element declaration has that type or owns those constraints:
// an inline simple type is checked and dropped (compileUnheldSimpleType), and
// detachUnreachableComponents detaches an inline complex type and the
// identity constraints.
func (c *compiler) compileAbsentElement(ctx *schemaSetDocument, n *schemaNode) error {
	if simple := n.firstXS(vocab.XSDElemSimpleType); simple != nil {
		if err := c.compileUnheldSimpleType(ctx, simple); err != nil {
			return err
		}
	}
	if complexType := n.firstXS(vocab.XSDElemComplexType); complexType != nil {
		name, err := localElementName(ctx, n)
		if err != nil {
			return err
		}
		if _, err := c.reserveAnonymousComplexType(ctx, complexType, absentElementComplexTypeLabel(name)); err != nil {
			return err
		}
	}
	identityNodes := identityConstraintNodes(n)
	identityIDs, err := c.declaredIdentityConstraints(ctx, identityNodes)
	if err != nil {
		return err
	}
	return c.compileDeclaredIdentityConstraints(ctx, identityNodes, identityIDs, NoElement)
}

func (c *compiler) compileElementParticleDeclaration(ctx *schemaSetDocument, n *schemaNode) (ElementID, error) {
	source, err := requiredSemanticSource[*schemaElementSource](n)
	if err != nil {
		return NoElement, err
	}
	if !source.Ref.Lexical.Present {
		return c.compileLocalElement(ctx, n)
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
	scope, err := c.enterComponent(raw.sourceNode())
	if err != nil {
		return 0, err
	}
	defer scope.leave(&err)
	id, err := c.registerGlobalElement(q, elementDecl{Name: q, Type: ComplexRef(c.rt.AnyType)})
	if err != nil {
		return 0, err
	}
	decl, pending, err := c.compileElementDecl(raw.ctx, raw.sourceNode(), id, q)
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
	scope, err := c.enterComponent(n)
	if err != nil {
		return 0, err
	}
	defer scope.leave(&err)
	name, err := localElementName(ctx, n)
	if err != nil {
		return 0, err
	}
	q, err := c.rt.internQName(name.Namespace, name.Local)
	if err != nil {
		return 0, err
	}
	id, err := c.addElement(elementDecl{Name: q, Type: ComplexRef(c.rt.AnyType)})
	if err != nil {
		return 0, err
	}
	c.localDone[nodeKey(ctx, n)] = id
	decl, pending, err := c.compileElementDecl(ctx, n, id, q)
	if err != nil {
		return 0, err
	}
	c.completeElement(id, decl)
	c.addPendingElementConstraint(id, n, pending)
	return id, nil
}

// localElementName returns the expanded name of the local element
// declaration n, qualified as its form says.
func localElementName(ctx *schemaSetDocument, n *schemaNode) (ExpandedName, error) {
	source, err := requiredSemanticSource[*schemaElementSource](n)
	if err != nil {
		return ExpandedName{}, err
	}
	qualified, err := parseElementFormAttr(formAttr{
		Value:            source.Form.Value,
		HasValue:         source.Form.Present,
		DefaultQualified: ctx.doc.defaults.ElementQualified,
	})
	if err != nil {
		return ExpandedName{}, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	name := ExpandedName{Local: source.Global.Name.Value}
	if qualified {
		name.Namespace = ctx.effectiveTargetNS
	}
	return name, nil
}

// pendingElementConstraint is an unvalidated element value constraint that
// waits for the declaration's type to finalize.
type pendingElementConstraint struct {
	node       *schemaNode
	constraint *valueConstraint
	element    ElementID
}

func (c *compiler) compileElementDecl(ctx *schemaSetDocument, n *schemaNode, id ElementID, q QName) (elementDecl, *valueConstraint, error) {
	identityNodes := identityConstraintNodes(n)
	identityIDs, err := c.declaredIdentityConstraints(ctx, identityNodes)
	if err != nil {
		return elementDecl{}, nil, err
	}
	properties, err := c.compileElementProperties(ctx, n, q)
	if err != nil {
		return elementDecl{}, nil, err
	}
	decl := elementDecl{
		Name:                         q,
		Type:                         properties.typ,
		Nillable:                     properties.nillable,
		Abstract:                     properties.abstract,
		SubstitutionGroupAffiliation: NoElement,
	}
	if maskErr := applyElementDerivationMasks(ctx, n, &decl); maskErr != nil {
		return elementDecl{}, nil, maskErr
	}
	constraint, err := compileElementConstraintSource(n)
	if err != nil {
		return elementDecl{}, nil, err
	}
	if err := c.compileDeclaredIdentityConstraints(ctx, identityNodes, identityIDs, id); err != nil {
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

func (c *compiler) compileElementProperties(ctx *schemaSetDocument, n *schemaNode, q QName) (compiledElementProperties, error) {
	source, err := requiredSemanticSource[*schemaElementSource](n)
	if err != nil {
		return compiledElementProperties{}, err
	}
	nillable, err := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrNillable, Value: source.Nillable.Value, HasValue: source.Nillable.Present})
	if err != nil {
		return compiledElementProperties{}, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	abstract, err := parseBooleanAttr(booleanAttr{Name: vocab.XSDAttrAbstract, Value: source.Abstract.Value, HasValue: source.Abstract.Present})
	if err != nil {
		return compiledElementProperties{}, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	typ, err := c.compileElementDeclType(ctx, n, q)
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

func (c *compiler) compileElementDeclType(ctx *schemaSetDocument, n *schemaNode, q QName) (TypeID, error) {
	source, err := requiredSemanticSource[*schemaElementSource](n)
	if err != nil {
		return TypeID{}, err
	}
	if source.Type.Lexical.Present {
		return c.compileElementTypeAttribute(ctx, n, source.Type.Name)
	}
	if simple := n.firstXS(vocab.XSDElemSimpleType); simple != nil {
		id, err := c.compileAnonymousSimple(ctx, simple)
		return SimpleRef(id), err
	}
	if complexType := n.firstXS(vocab.XSDElemComplexType); complexType != nil {
		id, err := c.reserveAnonymousComplexType(ctx, complexType, elementComplexTypeLabel(q))
		return ComplexRef(id), err
	}
	return ComplexRef(c.rt.AnyType), nil
}

func applyElementDerivationMasks(ctx *schemaSetDocument, n *schemaNode, decl *elementDecl) error {
	source, err := requiredSemanticSource[*schemaElementSource](n)
	if err != nil {
		return err
	}
	block, err := derivationMaskWithDefaultChecked(n, source.Block, ctx.doc.defaults.BlockDefault, elementBlockDerivation())
	if err != nil {
		return err
	}
	decl.Block = block
	final, err := derivationMaskWithDefaultChecked(n, source.Final, ctx.doc.defaults.FinalDefault, elementFinalDerivation())
	if err != nil {
		return err
	}
	decl.Final = final
	return nil
}

func compileElementConstraintSource(n *schemaNode) (*valueConstraint, error) {
	source, err := requiredSemanticSource[*schemaElementSource](n)
	if err != nil {
		return nil, err
	}
	return sourceValueConstraint(n, source.Default, source.Fixed)
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
	if typ, found, err := c.resolveTypeQName(typeQName); found || err != nil {
		return typ, err
	}
	missing, err := c.unresolvedTypeReference(n, typeQName, schemaComponentType)
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
		decl.ValueConstraint = mixedContentConstraint(constraint)
		return nil
	}
	if !validSimpleTypeID(simpleID, len(unavailable)) {
		return xsderrors.InternalInvariant("element value constraint references invalid simple type", nil)
	}
	if unavailable[simpleID] {
		decl.ValueConstraint = nil
		return nil
	}
	if err = validateValueConstraintIdentity(&c.rt, simpleID, constraint, valueConstraintOwnerElementDeclaration); err != nil {
		return schemaValueError(c.limits, xsderrors.CodeSchemaInvalidAttribute, "invalid element value constraint", err)
	}
	validated, err := c.validateValueConstraint(simpleID, constraint, schemaQNameResolver(n), decl.Name, "element")
	if err != nil {
		return err
	}
	decl.ValueConstraint = validated
	return nil
}

// mixedContentConstraint builds the constraint for an emptiable mixed-content
// element, whose default or fixed text is used verbatim: the lexical form is
// its own canonical form and the value is untyped.
func mixedContentConstraint(constraint *valueConstraint) *valueConstraint {
	lexical := constraint.Lexical
	return &valueConstraint{
		Lexical:   lexical,
		Canonical: lexical,
		Value:     valuepkg.NewUntypedValue(lexical),
		Fixed:     constraint.Fixed,
	}
}
