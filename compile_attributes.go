package xsd

import "slices"

func (c *compiler) compileAttributeByQName(q qName) (attributeID, error) {
	if id, ok := c.attributeDone[q]; ok {
		return id, nil
	}
	if c.compilingAttr[q] {
		return 0, schemaCompile(ErrSchemaReference, "cyclic attribute declaration "+c.rt.Names.Format(q))
	}
	raw, ok := c.attributeRaw[q]
	if !ok {
		return 0, schemaCompile(ErrSchemaReference, "unknown attribute "+c.rt.Names.Format(q))
	}
	c.compilingAttr[q] = true
	defer delete(c.compilingAttr, q)
	decl, err := c.compileAttributeDecl(raw.node, raw.ctx, q)
	if err != nil {
		return 0, err
	}
	id := attributeID(len(c.rt.Attributes))
	c.rt.Attributes = append(c.rt.Attributes, decl)
	c.attributeDone[q] = id
	c.rt.GlobalAttributes[q] = id
	return id, nil
}

func (c *compiler) compileAttributeDecl(n *rawNode, ctx *schemaContext, q qName) (attributeDecl, error) {
	if _, ok := n.attr("value"); ok {
		return attributeDecl{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute cannot have value")
	}
	if err := validateAttributeDeclContent(n); err != nil {
		return attributeDecl{}, err
	}
	local := c.rt.Names.Local(q.Local)
	if local == "xmlns" {
		return attributeDecl{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute cannot be named xmlns")
	}
	if c.rt.Names.Namespace(q.Namespace) == xsiNamespaceURI {
		return attributeDecl{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute target namespace cannot be XMLSchema-instance")
	}
	typeID := c.rt.Builtin.AnySimpleType
	if typeLex, ok := n.attr("type"); ok {
		if n.firstXS("simpleType") != nil {
			return attributeDecl{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute cannot have both type and simpleType")
		}
		tq, err := c.resolveQNameChecked(n, ctx, typeLex)
		if err != nil {
			return attributeDecl{}, err
		}
		id, err := c.compileSimpleByQName(tq)
		if err != nil {
			return attributeDecl{}, err
		}
		typeID = id
	} else if st := n.firstXS("simpleType"); st != nil {
		id, err := c.compileAnonymousSimple(st, ctx)
		if err != nil {
			return attributeDecl{}, err
		}
		typeID = id
	}
	decl := attributeDecl{Name: q, Type: typeID}
	if v, ok := n.attr("default"); ok {
		decl.Default = v
		decl.HasDefault = true
	}
	if v, ok := n.attr("fixed"); ok {
		decl.Fixed = v
		decl.HasFixed = true
	}
	if decl.HasDefault && decl.HasFixed {
		return attributeDecl{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute cannot have both default and fixed")
	}
	if err := c.validateAttributeValueConstraints(&decl, c.schemaQNameResolver(n)); err != nil {
		return attributeDecl{}, err
	}
	return decl, nil
}

func validateAttributeDeclContent(n *rawNode) error {
	seenSimple := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case "annotation":
			if seenSimple {
				return schemaCompile(ErrSchemaContentModel, "attribute annotation must precede simpleType")
			}
		case "simpleType":
			if seenSimple {
				return schemaCompile(ErrSchemaContentModel, "attribute can contain at most one simpleType")
			}
			seenSimple = true
		default:
			return schemaCompile(ErrSchemaContentModel, "invalid attribute child "+child.Name.Local)
		}
	}
	return nil
}

func (c *compiler) validateAttributeValueConstraints(decl *attributeDecl, resolve qnameResolver) error {
	if (decl.HasDefault || decl.HasFixed) && c.typeDerivesFrom(typeID{Kind: typeSimple, ID: uint32(decl.Type)}, typeID{Kind: typeSimple, ID: uint32(c.rt.Builtin.ID)}) {
		return schemaCompile(ErrSchemaInvalidAttribute, "ID-typed attribute cannot have default or fixed")
	}
	if decl.HasDefault {
		value, err := validateSimpleValueInfo(&c.rt, decl.Type, decl.Default, resolve)
		if err != nil {
			if IsUnsupported(err) {
				return err
			}
			return schemaCompile(ErrSchemaFacet, "invalid attribute default value for "+c.rt.Names.Format(decl.Name))
		}
		decl.DefaultCanonical = value.Canonical
		decl.DefaultValue = value
	}
	if decl.HasFixed {
		value, err := validateSimpleValueInfo(&c.rt, decl.Type, decl.Fixed, resolve)
		if err != nil {
			if IsUnsupported(err) {
				return err
			}
			return schemaCompile(ErrSchemaFacet, "invalid attribute fixed value for "+c.rt.Names.Format(decl.Name))
		}
		decl.FixedCanonical = value.Canonical
		decl.FixedValue = value
	}
	return nil
}

func (c *compiler) schemaQNameResolver(n *rawNode) qnameResolver {
	return func(lexical string) (string, bool) {
		ns, local, err := n.resolveQName(lexical)
		if err != nil {
			return "", false
		}
		if ns == "" {
			return local, true
		}
		return "{" + ns + "}" + local, true
	}
}

func (c *compiler) compileAttributeUses(parent *rawNode, ctx *schemaContext, inherited []attributeUse, inheritedWildcard wildcardID, allowOverride bool) (attributeUseSetID, error) {
	uses := slices.Clone(inherited)
	completeWildcard := noWildcard
	for _, child := range parent.xsContentChildren() {
		switch child.Name.Local {
		case "attribute":
			u, err := c.compileAttributeUse(child, ctx)
			if err != nil {
				return noAttributeUseSet, err
			}
			uses, err = c.mergeAttributeUse(uses, u, allowOverride, inheritedWildcard)
			if err != nil {
				return noAttributeUseSet, err
			}
		case "attributeGroup":
			groupUses, groupWildcard, err := c.compileAttributeGroupUse(child, ctx)
			if err != nil {
				return noAttributeUseSet, err
			}
			for _, u := range groupUses {
				uses, err = c.mergeAttributeUse(uses, u, allowOverride, inheritedWildcard)
				if err != nil {
					return noAttributeUseSet, err
				}
			}
			if groupWildcard != noWildcard {
				if completeWildcard == noWildcard {
					completeWildcard = groupWildcard
				} else {
					process := c.rt.Wildcards[completeWildcard].Process
					id, err := c.intersectWildcards(completeWildcard, groupWildcard, process)
					if err != nil {
						return noAttributeUseSet, err
					}
					completeWildcard = id
				}
			}
		case "anyAttribute":
			id, err := c.compileWildcard(child, ctx, true)
			if err != nil {
				return noAttributeUseSet, err
			}
			if completeWildcard == noWildcard {
				completeWildcard = id
			} else {
				process := c.rt.Wildcards[id].Process
				intersectionID, err := c.intersectWildcards(completeWildcard, id, process)
				if err != nil {
					return noAttributeUseSet, err
				}
				completeWildcard = intersectionID
			}
		default:
			if parent.Name.Local == "attributeGroup" && child.Name.Space == xsdNamespaceURI {
				return noAttributeUseSet, schemaCompile(ErrSchemaContentModel, "invalid attribute use child "+child.Name.Local)
			}
		}
	}
	wildcard := completeWildcard
	if allowOverride {
		if completeWildcard != noWildcard {
			if inheritedWildcard == noWildcard {
				return noAttributeUseSet, schemaCompile(ErrSchemaInvalidAttribute, "attribute wildcard restriction requires base wildcard")
			}
			if !c.wildcardSubset(completeWildcard, inheritedWildcard) {
				return noAttributeUseSet, schemaCompile(ErrSchemaInvalidAttribute, "attribute wildcard restriction is not subset of base")
			}
		}
	} else if parent.Name.Local == "extension" && inheritedWildcard != noWildcard {
		if completeWildcard == noWildcard {
			wildcard = inheritedWildcard
		} else {
			process := c.rt.Wildcards[completeWildcard].Process
			id, err := c.unionWildcards(completeWildcard, inheritedWildcard, process)
			if err != nil {
				return noAttributeUseSet, err
			}
			wildcard = id
		}
	}
	finalUses := removeProhibitedAttributeUses(uses)
	if err := c.validateAttributeUseSet(finalUses); err != nil {
		return noAttributeUseSet, err
	}
	id := attributeUseSetID(len(c.rt.AttributeUseSets))
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, newAttributeUseSet(finalUses, wildcard))
	return id, nil
}

func newAttributeUseSet(uses []attributeUse, wildcard wildcardID) attributeUseSet {
	set := attributeUseSet{Uses: uses, wildcard: wildcard}
	if len(uses) != 0 {
		set.Index = make(map[qName]uint32, len(uses))
	}
	for i, use := range uses {
		slot := uint32(i)
		set.Index[use.Name] = slot
		if use.Required {
			set.Required = append(set.Required, slot)
		}
		if use.HasDefault || use.HasFixed {
			set.ValueConstraints = append(set.ValueConstraints, slot)
		}
	}
	return set
}

func (c *compiler) mergeAttributeUse(uses []attributeUse, u attributeUse, allowOverride bool, inheritedWildcard wildcardID) ([]attributeUse, error) {
	for i := range uses {
		if uses[i].Name == u.Name {
			if !allowOverride && !uses[i].Prohibited && !u.Prohibited {
				return nil, schemaCompile(ErrSchemaDuplicate, "duplicate attribute use")
			}
			if allowOverride {
				if err := c.validateAttributeUseRestriction(uses[i], u); err != nil {
					return nil, err
				}
			}
			uses[i] = u
			return uses, nil
		}
	}
	if allowOverride && !u.Prohibited {
		if inheritedWildcard == noWildcard || !c.wildcardAllowsQName(inheritedWildcard, u.Name) {
			return nil, schemaCompile(ErrSchemaInvalidAttribute, "new restricted attribute is not allowed by base wildcard")
		}
	}
	return append(uses, u), nil
}

func (c *compiler) validateAttributeUseSet(uses []attributeUse) error {
	hasID := false
	for _, use := range uses {
		if !c.typeDerivesFrom(typeID{Kind: typeSimple, ID: uint32(use.Type)}, typeID{Kind: typeSimple, ID: uint32(c.rt.Builtin.ID)}) {
			continue
		}
		if hasID {
			return schemaCompile(ErrSchemaInvalidAttribute, "complex type cannot have multiple ID attributes")
		}
		hasID = true
	}
	return nil
}

func (c *compiler) validateAttributeUseRestriction(base, derived attributeUse) error {
	if derived.Prohibited {
		if base.Required {
			return schemaCompile(ErrSchemaInvalidAttribute, "required attribute cannot be prohibited by restriction")
		}
		return nil
	}
	if base.Required && !derived.Required {
		return schemaCompile(ErrSchemaInvalidAttribute, "required attribute cannot become optional by restriction")
	}
	if !c.typeDerivesFrom(typeID{Kind: typeSimple, ID: uint32(derived.Type)}, typeID{Kind: typeSimple, ID: uint32(base.Type)}) {
		return schemaCompile(ErrSchemaInvalidAttribute, "restricted attribute type is not derived from base")
	}
	if base.HasFixed {
		if !derived.HasFixed {
			return schemaCompile(ErrSchemaInvalidAttribute, "fixed attribute constraint must be preserved by restriction")
		}
		if base.FixedCanonical != derived.FixedCanonical {
			return schemaCompile(ErrSchemaInvalidAttribute, "fixed attribute constraint must be preserved by restriction")
		}
	}
	return nil
}

func removeProhibitedAttributeUses(uses []attributeUse) []attributeUse {
	out := uses[:0]
	for _, u := range uses {
		if !u.Prohibited {
			out = append(out, u)
		}
	}
	return out
}

func (c *compiler) attrUsesAndWildcard(id attributeUseSetID) ([]attributeUse, wildcardID) {
	if id == noAttributeUseSet {
		return nil, noWildcard
	}
	set := c.rt.AttributeUseSets[id]
	return set.Uses, set.wildcard
}

func (c *compiler) compileAttributeUse(n *rawNode, ctx *schemaContext) (attributeUse, error) {
	use := attributeUse{Type: c.rt.Builtin.AnySimpleType}
	refHasFixed := false
	refFixedCanonical := ""
	if _, ok := n.attr("value"); ok {
		return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute cannot have value")
	}
	if ref, ok := n.attr("ref"); ok {
		for _, attr := range []string{"name", "type", "form"} {
			if _, ok := n.attr(attr); ok {
				return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute ref cannot have "+attr)
			}
		}
		if children := n.xsContentChildren(); len(children) != 0 {
			return attributeUse{}, schemaCompile(ErrSchemaContentModel, "attribute ref can contain only annotation")
		}
		q, err := c.resolveQNameChecked(n, ctx, ref)
		if err != nil {
			return attributeUse{}, err
		}
		id, err := c.compileAttributeByQName(q)
		if err != nil {
			return attributeUse{}, err
		}
		decl := c.rt.Attributes[id]
		use.Name = decl.Name
		use.Type = decl.Type
		use.Default = decl.Default
		use.Fixed = decl.Fixed
		use.DefaultCanonical = decl.DefaultCanonical
		use.FixedCanonical = decl.FixedCanonical
		use.DefaultValue = decl.DefaultValue
		use.FixedValue = decl.FixedValue
		use.HasDefault = decl.HasDefault
		use.HasFixed = decl.HasFixed
		refHasFixed = decl.HasFixed
		refFixedCanonical = decl.FixedCanonical
	} else {
		name, ok := n.attr("name")
		if !ok {
			return attributeUse{}, schemaCompile(ErrSchemaReference, "attribute missing name or ref")
		}
		ns := ""
		form, hasForm := n.attr("form")
		if hasForm && form != "qualified" && form != "unqualified" {
			return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "invalid attribute form "+form)
		}
		if form == "qualified" || (!hasForm && ctx.attrQualified) {
			ns = ctx.targetNS
		}
		q, err := c.rt.Names.InternQName(ns, name)
		if err != nil {
			return attributeUse{}, err
		}
		use.Name = q
		decl, err := c.compileAttributeDecl(n, ctx, use.Name)
		if err != nil {
			return attributeUse{}, err
		}
		use.Type = decl.Type
		use.Default = decl.Default
		use.Fixed = decl.Fixed
		use.DefaultCanonical = decl.DefaultCanonical
		use.FixedCanonical = decl.FixedCanonical
		use.DefaultValue = decl.DefaultValue
		use.FixedValue = decl.FixedValue
		use.HasDefault = decl.HasDefault
		use.HasFixed = decl.HasFixed
	}
	switch n.attrDefault("use", "optional") {
	case "required":
		if _, ok := n.attr("default"); ok {
			return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "required attribute cannot have default")
		}
		use.Required = true
	case "prohibited":
		if _, ok := n.attr("default"); ok {
			return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "prohibited attribute cannot have default")
		}
		use.Prohibited = true
	case "optional":
	default:
		return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "invalid attribute use "+n.attrDefault("use", ""))
	}
	if v, ok := n.attr("default"); ok {
		if refHasFixed {
			return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute use default conflicts with fixed attribute declaration")
		}
		use.Default = v
		use.DefaultCanonical = ""
		use.DefaultValue = simpleValue{}
		use.HasDefault = true
	}
	if v, ok := n.attr("fixed"); ok {
		use.Fixed = v
		use.FixedCanonical = ""
		use.FixedValue = simpleValue{}
		use.HasFixed = true
	}
	if use.HasDefault && use.HasFixed {
		return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute cannot have both default and fixed")
	}
	if use.Prohibited && use.HasFixed {
		use.Prohibited = false
	}
	decl := attributeDecl{Name: use.Name, Type: use.Type, Default: use.Default, Fixed: use.Fixed, HasDefault: use.HasDefault, HasFixed: use.HasFixed}
	if err := c.validateAttributeValueConstraints(&decl, c.schemaQNameResolver(n)); err != nil {
		return attributeUse{}, err
	}
	if refHasFixed && use.HasFixed && decl.FixedCanonical != refFixedCanonical {
		return attributeUse{}, schemaCompile(ErrSchemaInvalidAttribute, "attribute use fixed value conflicts with fixed attribute declaration")
	}
	use.DefaultCanonical = decl.DefaultCanonical
	use.FixedCanonical = decl.FixedCanonical
	use.DefaultValue = decl.DefaultValue
	use.FixedValue = decl.FixedValue
	return use, nil
}

func (c *compiler) compileAttributeGroupUse(n *rawNode, ctx *schemaContext) ([]attributeUse, wildcardID, error) {
	if children := n.xsContentChildren(); len(children) != 0 {
		return nil, noWildcard, schemaCompile(ErrSchemaContentModel, "attributeGroup use can contain only annotation")
	}
	ref, ok := n.attr("ref")
	if !ok {
		return nil, noWildcard, schemaCompile(ErrSchemaReference, "attributeGroup use missing ref")
	}
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return nil, noWildcard, err
	}
	return c.compileAttributeGroupByQName(q)
}

func (c *compiler) compileAttributeGroupByQName(q qName) ([]attributeUse, wildcardID, error) {
	if id, ok := c.attrGroupDone[q]; ok {
		set := c.rt.AttributeUseSets[id]
		return set.Uses, set.wildcard, nil
	}
	raw, ok := c.attrGroupRaw[q]
	if !ok {
		return nil, noWildcard, schemaCompile(ErrSchemaReference, "unknown attribute group "+c.rt.Names.Format(q))
	}
	if c.compilingAttrGrp[q] {
		return nil, noWildcard, schemaCompile(ErrSchemaReference, "recursive attribute group "+c.rt.Names.Format(q))
	}
	c.compilingAttrGrp[q] = true
	defer delete(c.compilingAttrGrp, q)
	id, err := c.compileAttributeUses(raw.node, raw.ctx, nil, noWildcard, false)
	if err != nil {
		return nil, noWildcard, err
	}
	c.attrGroupDone[q] = id
	set := c.rt.AttributeUseSets[id]
	return set.Uses, set.wildcard, nil
}
