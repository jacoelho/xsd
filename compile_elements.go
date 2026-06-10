package xsd

func (c *compiler) compileElementParticle(n *rawNode, ctx *schemaContext) (particle, error) {
	var (
		id  elementID
		err error
	)
	if ref, ok := n.attr(xsdAttrRef); ok {
		err = validateKnownAttributes(n, "element ref", isElementRefAttribute)
		if err != nil {
			return particle{}, err
		}
		if len(n.xsContentChildren()) != 0 {
			return particle{}, schemaCompileAt(n, ErrSchemaContentModel, "element ref can contain only annotation")
		}
		var q qName
		q, err = c.resolveQNameChecked(n, ctx, ref)
		if err != nil {
			return particle{}, err
		}
		id, err = c.compileElementByQName(q)
		if err != nil {
			return particle{}, withSchemaCompileLocation(n, err)
		}
	} else {
		id, err = c.compileLocalElement(n, ctx)
		if err != nil {
			return particle{}, err
		}
	}
	occurs, err := parseOccurs(n, c.limits)
	if err != nil {
		return particle{}, err
	}
	return particle{Kind: particleElement, Element: id, occurs: occurs}, nil
}

func (c *compiler) compileElementByQName(q qName) (elementID, error) {
	if id, ok := c.elementDone[q]; ok {
		return id, nil
	}
	if c.compilingElement[q] {
		if raw, ok := c.elementRaw[q]; ok {
			return 0, schemaCompileAt(raw.node, ErrSchemaReference, "cyclic element declaration "+c.rt.Names.Format(q))
		}
		return 0, schemaCompile(ErrSchemaReference, "cyclic element declaration "+c.rt.Names.Format(q))
	}
	raw, ok := c.elementRaw[q]
	if !ok {
		return 0, schemaCompile(ErrSchemaReference, "unknown element "+c.rt.Names.Format(q))
	}
	c.compilingElement[q] = true
	defer delete(c.compilingElement, q)
	id, err := nextElementID(len(c.rt.Elements))
	if err != nil {
		return 0, err
	}
	c.rt.Elements = append(c.rt.Elements, elementDecl{Name: q, Type: complexRef(c.rt.Builtin.AnyType)})
	c.elementDone[q] = id
	c.rt.GlobalElements[q] = id
	decl, err := c.compileElementDecl(raw.node, raw.ctx, q)
	if err != nil {
		return 0, err
	}
	c.rt.Elements[id] = decl
	return id, nil
}

func (c *compiler) compileLocalElement(n *rawNode, ctx *schemaContext) (elementID, error) {
	if id, ok := c.localDone[n]; ok {
		return id, nil
	}
	for _, attr := range []string{xsdAttrAbstract, xsdAttrFinal, xsdAttrSubstitutionGroup} {
		if _, ok := n.attr(attr); ok {
			return 0, schemaCompileAt(n, ErrSchemaInvalidAttribute, "local element cannot have "+attr)
		}
	}
	name, ok := n.attr(xsdAttrName)
	if !ok {
		return 0, schemaCompileAt(n, ErrSchemaReference, "local element missing name or ref")
	}
	ns := ""
	form := n.attrDefault(xsdAttrForm, "")
	if form != "" && form != xsdValueQualified && form != xsdValueUnqualified {
		return 0, schemaCompileAt(n, ErrSchemaInvalidAttribute, "invalid element form value "+form)
	}
	if form == xsdValueQualified || (form == "" && ctx.elementQualified) {
		ns = ctx.targetNS
	}
	q, err := c.rt.Names.InternQName(ns, name)
	if err != nil {
		return 0, err
	}
	id, err := nextElementID(len(c.rt.Elements))
	if err != nil {
		return 0, err
	}
	c.rt.Elements = append(c.rt.Elements, elementDecl{Name: q, Type: complexRef(c.rt.Builtin.AnyType)})
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

func validateElementDeclContent(n *rawNode) error {
	seenType := false
	seenIdentity := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case xsdElemAnnotation:
			if seenNonAnnotation {
				return schemaCompileAt(child, ErrSchemaContentModel, "element annotation must be first")
			}
		case xsdElemSimpleType, xsdElemComplexType:
			if seenType {
				return schemaCompileAt(child, ErrSchemaContentModel, "element can contain at most one anonymous type")
			}
			if seenIdentity {
				return schemaCompileAt(child, ErrSchemaContentModel, "element anonymous type must precede identity constraints")
			}
			seenType = true
			seenNonAnnotation = true
		case xsdElemUnique, xsdElemKey, xsdElemKeyref:
			seenIdentity = true
			seenNonAnnotation = true
		default:
			return schemaCompileAt(child, ErrSchemaContentModel, "invalid element child "+child.Name.Local)
		}
	}
	if _, ok := n.attr(xsdAttrType); ok && seenType {
		return schemaCompileAt(n, ErrSchemaInvalidAttribute, "element cannot have both type and anonymous type")
	}
	return nil
}

func isElementRefAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrRef, xsdAttrMinOccurs, xsdAttrMaxOccurs:
		return true
	default:
		return false
	}
}

func isElementAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrName, xsdAttrRef, xsdAttrType, xsdAttrSubstitutionGroup,
		xsdAttrNillable, xsdAttrDefault, xsdAttrFixed, xsdAttrForm,
		xsdAttrBlock, xsdAttrFinal, xsdAttrAbstract, xsdAttrMinOccurs, xsdAttrMaxOccurs:
		return true
	default:
		return false
	}
}

func (c *compiler) compileElementDecl(n *rawNode, ctx *schemaContext, q qName) (elementDecl, error) {
	c.elementDepth++
	defer func() { c.elementDepth-- }()
	if err := validateElementDeclContent(n); err != nil {
		return elementDecl{}, err
	}
	identityNodes := identityConstraintNodes(n)
	identityIDs, err := c.declareIdentityConstraints(identityNodes, ctx)
	if err != nil {
		return elementDecl{}, err
	}
	nillable, err := schemaBoolAttr(n, xsdAttrNillable)
	if err != nil {
		return elementDecl{}, err
	}
	abstract, err := schemaBoolAttr(n, xsdAttrAbstract)
	if err != nil {
		return elementDecl{}, err
	}
	typ := complexRef(c.rt.Builtin.AnyType)
	if typeLex, ok := n.attr(xsdAttrType); ok {
		attrType, typeErr := c.compileElementTypeAttribute(n, ctx, typeLex)
		if typeErr != nil {
			return elementDecl{}, typeErr
		}
		typ = attrType
	} else if st := n.firstXS(xsdElemSimpleType); st != nil {
		id, simpleErr := c.compileAnonymousSimple(st, ctx)
		if simpleErr != nil {
			return elementDecl{}, simpleErr
		}
		typ = simpleRef(id)
	} else if ct := n.firstXS(xsdElemComplexType); ct != nil {
		id, complexErr := c.compileAnonymousComplex(ct, ctx)
		if complexErr != nil {
			return elementDecl{}, complexErr
		}
		typ = complexRef(id)
	} else if headLex, ok := n.attr(xsdAttrSubstitutionGroup); ok {
		headQName, headErr := c.resolveQNameChecked(n, ctx, headLex)
		if headErr != nil {
			return elementDecl{}, headErr
		}
		if _, ok := c.elementRaw[headQName]; ok {
			headID, headErr := c.compileElementByQName(headQName)
			if headErr != nil {
				return elementDecl{}, withSchemaCompileLocation(n, headErr)
			}
			typ = c.rt.Elements[headID].Type
		}
	}
	decl := elementDecl{
		Name:      q,
		Type:      typ,
		Nillable:  nillable,
		Abstract:  abstract,
		SubstHead: noElement,
	}
	block, err := derivationMaskWithDefaultChecked(n, ctx.blockDefault, elementBlockDerivation)
	if err != nil {
		return elementDecl{}, err
	}
	decl.Block = block
	final, err := derivationMaskWithDefaultChecked(n, ctx.finalDefault, elementFinalDerivation)
	if err != nil {
		return elementDecl{}, err
	}
	decl.Final = final
	if v, ok := n.attr(xsdAttrDefault); ok {
		decl.Default.Lexical = v
		decl.Default.Present = true
	}
	if v, ok := n.attr(xsdAttrFixed); ok {
		decl.Fixed.Lexical = v
		decl.Fixed.Present = true
	}
	if decl.Default.Present && decl.Fixed.Present {
		return elementDecl{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "element cannot have both default and fixed")
	}
	if err := c.validateElementValueConstraints(&decl, c.schemaQNameResolver(n)); err != nil {
		return elementDecl{}, withSchemaCompileLocation(n, err)
	}
	if err := c.compileDeclaredIdentityConstraints(identityNodes, identityIDs, ctx); err != nil {
		return elementDecl{}, err
	}
	decl.Identity = identityIDs
	return decl, nil
}

func (c *compiler) compileElementTypeAttribute(n *rawNode, ctx *schemaContext, typeLex string) (typeID, error) {
	typeQName, err := c.resolveQNameChecked(n, ctx, typeLex)
	if err != nil {
		return typeID{}, err
	}
	if c.typeQNameKnown(typeQName) {
		return c.resolveTypeQName(typeQName)
	}
	missing, err := c.missingSimpleType()
	if err != nil {
		return typeID{}, err
	}
	return simpleRef(missing), nil
}

func (c *compiler) validateElementValueConstraints(decl *elementDecl, resolve qnameResolver) error {
	simpleID := noSimpleType
	switch decl.Type.Kind {
	case typeSimple:
		simpleID = simpleTypeID(decl.Type.ID)
	case typeComplex:
		ct := c.rt.ComplexTypes[decl.Type.ID]
		if ct.SimpleValue {
			simpleID = ct.TextType
		} else if (decl.Default.Present || decl.Fixed.Present) && ct.Mixed && c.modelEmptiable(ct.Content) {
			decl.Default.Canonical = decl.Default.Lexical
			decl.Fixed.Canonical = decl.Fixed.Lexical
			if decl.Default.Present {
				decl.Default.Value = simpleValue{Canonical: decl.Default.Lexical, Type: noSimpleType}
			}
			if decl.Fixed.Present {
				decl.Fixed.Value = simpleValue{Canonical: decl.Fixed.Lexical, Type: noSimpleType}
			}
			return nil
		}
	}
	if simpleID == noSimpleType {
		if decl.Default.Present || decl.Fixed.Present {
			return schemaCompile(ErrSchemaInvalidAttribute, "element value constraint requires simple content")
		}
		return nil
	}
	if (decl.Default.Present || decl.Fixed.Present) && c.typeDerivesFrom(simpleRef(simpleID), simpleRef(c.rt.Builtin.ID)) {
		return schemaCompile(ErrSchemaInvalidAttribute, "ID-typed element cannot have default or fixed")
	}
	if (decl.Default.Present || decl.Fixed.Present) && c.simpleTypeUsesBareNotation(simpleID, make(map[simpleTypeID]bool)) {
		return schemaCompile(ErrSchemaFacet, "NOTATION value constraint requires enumeration")
	}
	if decl.Default.Present {
		value, err := c.validateValueConstraint(simpleID, decl.Default.Lexical, resolve, decl.Name, "element default")
		if err != nil {
			return err
		}
		decl.Default.Canonical = value.Canonical
		decl.Default.Value = value
	}
	if decl.Fixed.Present {
		value, err := c.validateValueConstraint(simpleID, decl.Fixed.Lexical, resolve, decl.Name, "element fixed")
		if err != nil {
			return err
		}
		decl.Fixed.Canonical = value.Canonical
		decl.Fixed.Value = value
	}
	return nil
}

func (c *compiler) simpleTypeUsesBareNotation(id simpleTypeID, seen map[simpleTypeID]bool) bool {
	st, ok := c.rt.simpleType(id)
	if !ok || seen[id] {
		return false
	}
	seen[id] = true
	if st.Primitive == primNotation && len(st.Facets.Enumeration) == 0 {
		return true
	}
	if st.Variety == varietyList {
		return c.simpleTypeUsesBareNotation(st.ListItem, seen)
	}
	if st.Variety == varietyUnion {
		for _, member := range st.Union {
			if c.simpleTypeUsesBareNotation(member, seen) {
				return true
			}
		}
	}
	return false
}

func (c *compiler) simpleTypeHasListVariety(id simpleTypeID, seen map[simpleTypeID]bool) bool {
	st, ok := c.rt.simpleType(id)
	if !ok || seen[id] {
		return false
	}
	seen[id] = true
	if st.Variety == varietyList {
		return true
	}
	if st.Variety == varietyUnion {
		for _, member := range st.Union {
			if c.simpleTypeHasListVariety(member, seen) {
				return true
			}
		}
	}
	return false
}
