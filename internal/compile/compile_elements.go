package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
)

func (c *compiler) compileElementParticle(n *rawNode, ctx *schemaContext) (runtime.Particle, error) {
	var (
		id  runtime.ElementID
		err error
	)
	if ref, ok := n.attr(vocab.XSDAttrRef); ok {
		err = checkElementRefAttributes(n)
		if err != nil {
			return runtime.Particle{}, err
		}
		err = checkElementRefChildren(n)
		if err != nil {
			return runtime.Particle{}, err
		}
		var q runtime.QName
		q, err = c.resolveQNameChecked(n, ctx, ref)
		if err != nil {
			return runtime.Particle{}, err
		}
		id, err = c.compileElementByQName(q)
		if err != nil {
			return runtime.Particle{}, withSchemaCompileLocation(n, err)
		}
	} else {
		id, err = c.compileLocalElement(n, ctx)
		if err != nil {
			return runtime.Particle{}, err
		}
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return runtime.Particle{}, err
	}
	return runtime.ElementParticle(id, occurs), nil
}

func (c *compiler) compileElementByQName(q runtime.QName) (runtime.ElementID, error) {
	if id, ok := c.elementDone[q]; ok {
		return id, nil
	}
	label := c.rt.Names.Format(q)
	if c.compilingElement[q] {
		err := CheckSchemaComponentCycle(SchemaComponentElement, true, label)
		if raw, ok := c.elementRaw[q]; ok {
			return 0, withSchemaCompileLocation(raw.node, err)
		}
		return 0, err
	}
	raw, ok := c.elementRaw[q]
	if err := CheckSchemaComponentExists(SchemaComponentElement, ok, label); err != nil {
		return 0, err
	}
	c.compilingElement[q] = true
	defer delete(c.compilingElement, q)
	id, err := c.registerGlobalElement(q, runtime.ElementDecl{Name: q, Type: runtime.ComplexRef(c.rt.Builtin.AnyType)})
	if err != nil {
		return 0, err
	}
	c.elementDone[q] = id
	decl, err := c.compileElementDecl(raw.node, raw.ctx, q)
	if err != nil {
		return 0, err
	}
	c.rt.Elements[id] = decl
	return id, nil
}

func (c *compiler) compileLocalElement(n *rawNode, ctx *schemaContext) (runtime.ElementID, error) {
	if id, ok := c.localDone[n]; ok {
		return id, nil
	}
	if err := checkLocalElementAttributes(n); err != nil {
		return 0, err
	}
	if err := checkLocalElementSource(n); err != nil {
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
	q, err := c.names.InternQName(ns, name)
	if err != nil {
		return 0, err
	}
	id, err := NextElementID(len(c.rt.Elements))
	if err != nil {
		return 0, err
	}
	c.rt.Elements = append(c.rt.Elements, runtime.ElementDecl{Name: q, Type: runtime.ComplexRef(c.rt.Builtin.AnyType)})
	c.localDone[n] = id
	c.compilingLocal[n] = true
	defer delete(c.compilingLocal, n)
	decl, err := c.compileElementDecl(n, ctx, q)
	if err != nil {
		return 0, err
	}
	c.rt.Elements[id] = decl
	return id, nil
}

func (c *compiler) compileElementDecl(n *rawNode, ctx *schemaContext, q runtime.QName) (runtime.ElementDecl, error) {
	c.elementDepth++
	defer func() { c.elementDepth-- }()
	if err := checkElementDeclarationChildren(n); err != nil {
		return runtime.ElementDecl{}, err
	}
	identityNodes := identityConstraintNodes(n)
	identityIDs, err := c.declareIdentityConstraints(identityNodes, ctx)
	if err != nil {
		return runtime.ElementDecl{}, err
	}
	nillable, err := schemaBoolAttr(n, vocab.XSDAttrNillable)
	if err != nil {
		return runtime.ElementDecl{}, err
	}
	abstract, err := schemaBoolAttr(n, vocab.XSDAttrAbstract)
	if err != nil {
		return runtime.ElementDecl{}, err
	}
	typ := runtime.ComplexRef(c.rt.Builtin.AnyType)
	if typeLex, ok := n.attr(vocab.XSDAttrType); ok {
		attrType, typeErr := c.compileElementTypeAttribute(n, ctx, typeLex)
		if typeErr != nil {
			return runtime.ElementDecl{}, typeErr
		}
		typ = attrType
	} else if st := n.firstXS(vocab.XSDElemSimpleType); st != nil {
		id, simpleErr := c.compileAnonymousSimple(st, ctx)
		if simpleErr != nil {
			return runtime.ElementDecl{}, simpleErr
		}
		typ = runtime.SimpleRef(id)
	} else if ct := n.firstXS(vocab.XSDElemComplexType); ct != nil {
		id, complexErr := c.compileAnonymousComplex(ct, ctx)
		if complexErr != nil {
			return runtime.ElementDecl{}, complexErr
		}
		typ = runtime.ComplexRef(id)
	} else if headLex, ok := n.attr(vocab.XSDAttrSubstitutionGroup); ok {
		headQName, headErr := c.resolveQNameChecked(n, ctx, headLex)
		if headErr != nil {
			return runtime.ElementDecl{}, headErr
		}
		if _, ok := c.elementRaw[headQName]; ok {
			headID, headErr := c.compileElementByQName(headQName)
			if headErr != nil {
				return runtime.ElementDecl{}, withSchemaCompileLocation(n, headErr)
			}
			typ = c.rt.Elements[headID].Type
		}
	}
	decl := runtime.ElementDecl{
		Name:      q,
		Type:      typ,
		Nillable:  nillable,
		Abstract:  abstract,
		SubstHead: runtime.NoElement,
	}
	block, err := derivationMaskWithDefaultChecked(n, ctx.blockDefault, ElementBlockDerivation)
	if err != nil {
		return runtime.ElementDecl{}, err
	}
	decl.Block = block
	final, err := derivationMaskWithDefaultChecked(n, ctx.finalDefault, ElementFinalDerivation)
	if err != nil {
		return runtime.ElementDecl{}, err
	}
	decl.Final = final
	if v, ok := n.attr(vocab.XSDAttrDefault); ok {
		decl.Default = &runtime.ValueConstraint{Lexical: v}
	}
	if v, ok := n.attr(vocab.XSDAttrFixed); ok {
		decl.Fixed = &runtime.ValueConstraint{Lexical: v}
	}
	if err := validateElementDeclValueConstraintAdmission(n, decl.Default != nil, decl.Fixed != nil); err != nil {
		return runtime.ElementDecl{}, err
	}
	if err := c.validateElementValueConstraints(&decl, n); err != nil {
		return runtime.ElementDecl{}, withSchemaCompileLocation(n, err)
	}
	if err := c.compileDeclaredIdentityConstraints(identityNodes, identityIDs, ctx); err != nil {
		return runtime.ElementDecl{}, err
	}
	decl.Identity = identityIDs
	return decl, nil
}

func (c *compiler) compileElementTypeAttribute(n *rawNode, ctx *schemaContext, typeLex string) (runtime.TypeID, error) {
	typeQName, err := c.resolveQNameChecked(n, ctx, typeLex)
	if err != nil {
		return runtime.TypeID{}, err
	}
	if c.typeQNameKnown(typeQName) {
		return c.resolveTypeQName(typeQName)
	}
	missing, err := c.missingSimpleType()
	if err != nil {
		return runtime.TypeID{}, err
	}
	return runtime.SimpleRef(missing), nil
}

func (c *compiler) validateElementValueConstraints(decl *runtime.ElementDecl, n *rawNode) error {
	if decl.Default == nil && decl.Fixed == nil {
		return nil
	}
	simpleID, err := runtime.ElementValueConstraintType(&c.rt, decl.Type)
	if err != nil {
		return ElementValueConstraintTypeError(err)
	}
	if simpleID == runtime.NoSimpleType {
		if decl.Default != nil {
			decl.Default = mixedContentConstraint(decl.Default.Lexical)
		}
		if decl.Fixed != nil {
			decl.Fixed = mixedContentConstraint(decl.Fixed.Lexical)
		}
		return nil
	}
	if err := runtime.ValidateElementDeclValueConstraintRuntime(&c.rt, simpleID, decl.Default != nil, decl.Fixed != nil); err != nil {
		return ElementValueConstraintRuntimeError(err)
	}
	resolve := c.schemaQNameResolver(n)
	if decl.Default != nil {
		vc, err := c.validateValueConstraint(simpleID, decl.Default.Lexical, resolve, decl.Name, "element default")
		if err != nil {
			return err
		}
		decl.Default = vc
	}
	if decl.Fixed != nil {
		vc, err := c.validateValueConstraint(simpleID, decl.Fixed.Lexical, resolve, decl.Name, "element fixed")
		if err != nil {
			return err
		}
		decl.Fixed = vc
	}
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
