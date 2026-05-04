package xsd

func (c *compiler) compileElementParticle(n *rawNode, ctx *schemaContext) (particle, error) {
	if ref, ok := n.attr("ref"); ok {
		if err := validateKnownAttributes(n, "element ref", map[string]bool{
			"id": true, "ref": true, "minOccurs": true, "maxOccurs": true,
		}); err != nil {
			return particle{}, err
		}
		if len(n.xsContentChildren()) != 0 {
			return particle{}, schemaCompile(ErrSchemaContentModel, "element ref can contain only annotation")
		}
		q, err := c.resolveQNameChecked(n, ctx, ref)
		if err != nil {
			return particle{}, err
		}
		id, err := c.compileElementByQName(q)
		if err != nil {
			return particle{}, err
		}
		occurs, err := parseOccurs(n, c.limits)
		if err != nil {
			return particle{}, err
		}
		return particle{Kind: particleElement, Element: id, occurs: occurs}, nil
	}
	id, err := c.compileLocalElement(n, ctx)
	if err != nil {
		return particle{}, err
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
		return 0, schemaCompile(ErrSchemaReference, "cyclic element declaration "+c.rt.Names.Format(q))
	}
	raw, ok := c.elementRaw[q]
	if !ok {
		return 0, schemaCompile(ErrSchemaReference, "unknown element "+c.rt.Names.Format(q))
	}
	c.compilingElement[q] = true
	defer delete(c.compilingElement, q)
	id := elementID(len(c.rt.Elements))
	c.rt.Elements = append(c.rt.Elements, elementDecl{Name: q, Type: typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)}})
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
	for _, attr := range []string{"abstract", "final", "substitutionGroup"} {
		if _, ok := n.attr(attr); ok {
			return 0, schemaCompile(ErrSchemaInvalidAttribute, "local element cannot have "+attr)
		}
	}
	name, ok := n.attr("name")
	if !ok {
		return 0, schemaCompile(ErrSchemaReference, "local element missing name or ref")
	}
	ns := ""
	form := n.attrDefault("form", "")
	if form != "" && form != "qualified" && form != "unqualified" {
		return 0, schemaCompile(ErrSchemaInvalidAttribute, "invalid element form value "+form)
	}
	if form == "qualified" || (form == "" && ctx.elementQualified) {
		ns = ctx.targetNS
	}
	q := c.rt.Names.InternQName(ns, name)
	id := elementID(len(c.rt.Elements))
	c.rt.Elements = append(c.rt.Elements, elementDecl{Name: q, Type: typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)}})
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
	if err := validateKnownAttributes(n, "element", map[string]bool{
		"id": true, "name": true, "ref": true, "type": true, "substitutionGroup": true,
		"nillable": true, "default": true, "fixed": true, "form": true,
		"block": true, "final": true, "abstract": true, "minOccurs": true, "maxOccurs": true,
	}); err != nil {
		return err
	}
	seenType := false
	seenIdentity := false
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case "annotation":
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, "element annotation must be first")
			}
		case "simpleType", "complexType":
			if seenType {
				return schemaCompile(ErrSchemaContentModel, "element can contain at most one anonymous type")
			}
			if seenIdentity {
				return schemaCompile(ErrSchemaContentModel, "element anonymous type must precede identity constraints")
			}
			seenType = true
			seenNonAnnotation = true
		case "unique", "key", "keyref":
			seenIdentity = true
			seenNonAnnotation = true
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid element child "+child.Name.Local)
		}
	}
	if _, ok := n.attr("type"); ok && seenType {
		return schemaCompile(ErrSchemaInvalidAttribute, "element cannot have both type and anonymous type")
	}
	return nil
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
	nillable, err := schemaBoolAttr(n, "nillable", false)
	if err != nil {
		return elementDecl{}, err
	}
	abstract, err := schemaBoolAttr(n, "abstract", false)
	if err != nil {
		return elementDecl{}, err
	}
	typ := typeID{Kind: typeComplex, ID: uint32(c.rt.Builtin.AnyType)}
	if typeLex, ok := n.attr("type"); ok {
		typeQName, typeErr := c.resolveQNameChecked(n, ctx, typeLex)
		if typeErr != nil {
			return elementDecl{}, typeErr
		}
		if c.typeQNameKnown(typeQName) {
			t, typeErr := c.resolveTypeQName(typeQName)
			if typeErr != nil {
				return elementDecl{}, typeErr
			}
			typ = t
		} else {
			typ = typeID{Kind: typeSimple, ID: uint32(c.missingSimpleType())}
		}
	} else if st := n.firstXS("simpleType"); st != nil {
		id, simpleErr := c.compileAnonymousSimple(st, ctx)
		if simpleErr != nil {
			return elementDecl{}, simpleErr
		}
		typ = typeID{Kind: typeSimple, ID: uint32(id)}
	} else if ct := n.firstXS("complexType"); ct != nil {
		id, complexErr := c.compileAnonymousComplex(ct, ctx)
		if complexErr != nil {
			return elementDecl{}, complexErr
		}
		typ = typeID{Kind: typeComplex, ID: uint32(id)}
	} else if headLex, ok := n.attr("substitutionGroup"); ok {
		headQName, headErr := c.resolveQNameChecked(n, ctx, headLex)
		if headErr != nil {
			return elementDecl{}, headErr
		}
		if _, ok := c.elementRaw[headQName]; ok {
			headID, headErr := c.compileElementByQName(headQName)
			if headErr != nil {
				return elementDecl{}, headErr
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
	block, err := derivationMaskWithDefaultChecked(n, "block", ctx.blockDefault, true, "element block")
	if err != nil {
		return elementDecl{}, err
	}
	decl.Block = block
	final, err := derivationMaskWithDefaultChecked(n, "final", ctx.finalDefault, false, "element final")
	if err != nil {
		return elementDecl{}, err
	}
	decl.Final = final
	if v, ok := n.attr("default"); ok {
		decl.Default = v
		decl.HasDefault = true
	}
	if v, ok := n.attr("fixed"); ok {
		decl.Fixed = v
		decl.HasFixed = true
	}
	if decl.HasDefault && decl.HasFixed {
		return elementDecl{}, schemaCompile(ErrSchemaInvalidAttribute, "element cannot have both default and fixed")
	}
	if err := c.validateElementValueConstraints(decl, c.schemaQNameResolver(n)); err != nil {
		return elementDecl{}, err
	}
	if err := c.compileDeclaredIdentityConstraints(identityNodes, identityIDs, ctx); err != nil {
		return elementDecl{}, err
	}
	decl.Identity = identityIDs
	return decl, nil
}

func (c *compiler) validateElementValueConstraints(decl elementDecl, resolve qnameResolver) error {
	simpleID := noSimpleType
	switch decl.Type.Kind {
	case typeSimple:
		simpleID = simpleTypeID(decl.Type.ID)
	case typeComplex:
		ct := c.rt.ComplexTypes[decl.Type.ID]
		if ct.SimpleValue {
			simpleID = ct.TextType
		} else if (decl.HasDefault || decl.HasFixed) && ct.Mixed && c.modelEmptiable(ct.Content) {
			return nil
		}
	}
	if simpleID == noSimpleType {
		if decl.HasDefault || decl.HasFixed {
			return schemaCompile(ErrSchemaInvalidAttribute, "element value constraint requires simple content")
		}
		return nil
	}
	if (decl.HasDefault || decl.HasFixed) && c.typeDerivesFrom(typeID{Kind: typeSimple, ID: uint32(simpleID)}, typeID{Kind: typeSimple, ID: uint32(c.rt.Builtin.ID)}) {
		return schemaCompile(ErrSchemaInvalidAttribute, "ID-typed element cannot have default or fixed")
	}
	if (decl.HasDefault || decl.HasFixed) && c.simpleTypeUsesBareNotation(simpleID, make(map[simpleTypeID]bool)) {
		return schemaCompile(ErrSchemaFacet, "NOTATION value constraint requires enumeration")
	}
	if decl.HasDefault {
		if _, err := validateSimpleValue(&c.rt, simpleID, decl.Default, resolve); err != nil {
			if IsUnsupported(err) {
				return err
			}
			return schemaCompile(ErrSchemaFacet, "invalid element default value for "+c.rt.Names.Format(decl.Name))
		}
	}
	if decl.HasFixed {
		if _, err := validateSimpleValue(&c.rt, simpleID, decl.Fixed, resolve); err != nil {
			if IsUnsupported(err) {
				return err
			}
			return schemaCompile(ErrSchemaFacet, "invalid element fixed value for "+c.rt.Names.Format(decl.Name))
		}
	}
	return nil
}

func (c *compiler) simpleTypeUsesBareNotation(id simpleTypeID, seen map[simpleTypeID]bool) bool {
	if id == noSimpleType || seen[id] || int(id) >= len(c.rt.SimpleTypes) {
		return false
	}
	seen[id] = true
	st := c.rt.SimpleTypes[id]
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
	if id == noSimpleType || seen[id] || int(id) >= len(c.rt.SimpleTypes) {
		return false
	}
	seen[id] = true
	st := c.rt.SimpleTypes[id]
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
