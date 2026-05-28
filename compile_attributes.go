package xsd

import "slices"

func (c *compiler) compileAttributeByQName(q qName) (attributeID, error) {
	if id, ok := c.attributeDone[q]; ok {
		return id, nil
	}
	if c.compilingAttr[q] {
		if raw, ok := c.attributeRaw[q]; ok {
			return 0, schemaCompileAt(raw.node, ErrSchemaReference, "cyclic attribute declaration "+c.rt.Names.Format(q))
		}
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
	id, err := nextAttributeID(len(c.rt.Attributes))
	if err != nil {
		return 0, err
	}
	c.rt.Attributes = append(c.rt.Attributes, decl)
	c.attributeDone[q] = id
	c.rt.GlobalAttributes[q] = id
	return id, nil
}

func (c *compiler) compileAttributeDecl(n *rawNode, ctx *schemaContext, q qName) (attributeDecl, error) {
	if err := validateAttributeDeclContent(n); err != nil {
		return attributeDecl{}, err
	}
	local := c.rt.Names.Local(q.Local)
	if local == xmlnsPrefix {
		return attributeDecl{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "attribute cannot be named xmlns")
	}
	if c.rt.Names.Namespace(q.Namespace) == xsiNamespaceURI {
		return attributeDecl{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "attribute target namespace cannot be XMLSchema-instance")
	}
	typeID := c.rt.Builtin.AnySimpleType
	if typeLex, ok := n.attr(xsdAttrType); ok {
		if n.firstXS(xsdElemSimpleType) != nil {
			return attributeDecl{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "attribute cannot have both type and simpleType")
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
	} else if st := n.firstXS(xsdElemSimpleType); st != nil {
		id, err := c.compileAnonymousSimple(st, ctx)
		if err != nil {
			return attributeDecl{}, err
		}
		typeID = id
	}
	decl := attributeDecl{Name: q, Type: typeID}
	if v, ok := n.attr(xsdAttrDefault); ok {
		decl.Default = v
		decl.HasDefault = true
	}
	if v, ok := n.attr(xsdAttrFixed); ok {
		decl.Fixed = v
		decl.HasFixed = true
	}
	if decl.HasDefault && decl.HasFixed {
		return attributeDecl{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "attribute cannot have both default and fixed")
	}
	if err := c.validateAttributeValueConstraints(&decl, c.schemaQNameResolver(n)); err != nil {
		return attributeDecl{}, withSchemaCompileLocation(n, err)
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
		case xsdElemAnnotation:
			if seenSimple {
				return schemaCompileAt(child, ErrSchemaContentModel, "attribute annotation must precede simpleType")
			}
		case xsdElemSimpleType:
			if seenSimple {
				return schemaCompileAt(child, ErrSchemaContentModel, "attribute can contain at most one simpleType")
			}
			seenSimple = true
		default:
			return schemaCompileAt(child, ErrSchemaContentModel, "invalid attribute child "+child.Name.Local)
		}
	}
	return nil
}

func isAttributeAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrName, xsdAttrRef, xsdAttrType, xsdAttrUse, xsdAttrDefault, xsdAttrFixed, xsdAttrForm:
		return true
	default:
		return false
	}
}

func (c *compiler) validateAttributeValueConstraints(decl *attributeDecl, resolve qnameResolver) error {
	if (decl.HasDefault || decl.HasFixed) && c.typeDerivesFrom(typeID{Kind: typeSimple, ID: uint32(decl.Type)}, typeID{Kind: typeSimple, ID: uint32(c.rt.Builtin.ID)}) {
		return schemaCompile(ErrSchemaInvalidAttribute, "ID-typed attribute cannot have default or fixed")
	}
	if decl.HasDefault {
		value, err := c.validateValueConstraint(decl.Type, decl.Default, resolve, decl.Name, "attribute default")
		if err != nil {
			return err
		}
		decl.DefaultCanonical = value.Canonical
		decl.DefaultValue = value
	}
	if decl.HasFixed {
		value, err := c.validateValueConstraint(decl.Type, decl.Fixed, resolve, decl.Name, "attribute fixed")
		if err != nil {
			return err
		}
		decl.FixedCanonical = value.Canonical
		decl.FixedValue = value
	}
	return nil
}

func (c *compiler) validateValueConstraint(id simpleTypeID, lexical string, resolve qnameResolver, owner qName, label string) (simpleValue, error) {
	value, err := validateSimpleValueMode(&c.rt, id, lexical, resolve, simpleNeedCanonical|simpleNeedIdentity)
	if err != nil {
		if IsUnsupported(err) {
			return simpleValue{}, err
		}
		return simpleValue{}, schemaCompile(ErrSchemaFacet, "invalid "+label+" value for "+c.rt.Names.Format(owner))
	}
	return value, nil
}

func (c *compiler) schemaQNameResolver(n *rawNode) qnameResolver {
	return func(lexical string) (string, bool) {
		ns, local, err := n.resolveQName(lexical)
		if err != nil {
			return "", false
		}
		return formatExpandedName(ns, local), true
	}
}

type attributeUseMergeMode uint8

const (
	attributeMergeNormal attributeUseMergeMode = iota
	attributeMergeRestriction
)

func (c *compiler) compileAttributeUses(parent *rawNode, ctx *schemaContext, inherited []attributeUse, inheritedWildcard wildcardID, mode attributeUseMergeMode) (attributeUseSetID, error) {
	uses := slices.Clone(inherited)
	seen := make(map[qName]int, len(uses))
	for i := range uses {
		seen[uses[i].Name] = i
	}
	wildcards := attributeWildcardBuilder{wildcard: noWildcard, inheritedWildcard: inheritedWildcard, mode: mode}
	for _, child := range parent.xsContentChildren() {
		switch child.Name.Local {
		case xsdElemAttribute:
			u, err := c.compileAttributeUse(child, ctx)
			if err != nil {
				return noAttributeUseSet, err
			}
			uses, err = c.mergeAttributeUse(uses, seen, u, mode, inheritedWildcard)
			if err != nil {
				return noAttributeUseSet, withSchemaCompileLocation(child, err)
			}
		case xsdElemAttributeGroup:
			groupUses, groupWildcard, err := c.compileAttributeGroupUse(child, ctx)
			if err != nil {
				return noAttributeUseSet, err
			}
			for _, u := range groupUses {
				uses, err = c.mergeAttributeUse(uses, seen, u, mode, inheritedWildcard)
				if err != nil {
					return noAttributeUseSet, withSchemaCompileLocation(child, err)
				}
			}
			if err := wildcards.addGroup(c, groupWildcard); err != nil {
				return noAttributeUseSet, withSchemaCompileLocation(child, err)
			}
		case xsdElemAnyAttribute:
			id, err := c.compileAttributeWildcard(child, ctx)
			if err != nil {
				return noAttributeUseSet, err
			}
			if err := wildcards.addAnyAttribute(c, id); err != nil {
				return noAttributeUseSet, withSchemaCompileLocation(child, err)
			}
		default:
			if parent.Name.Local == xsdElemAttributeGroup && child.Name.Space == xsdNamespaceURI {
				return noAttributeUseSet, schemaCompileAt(child, ErrSchemaContentModel, "invalid attribute use child "+child.Name.Local)
			}
		}
	}
	wildcard, err := wildcards.finish(c, parent.Name.Local)
	if err != nil {
		return noAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	finalUses := removeProhibitedAttributeUses(uses)
	if err = c.validateAttributeUseSet(finalUses); err != nil {
		return noAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	id, err := nextAttributeUseSetID(len(c.rt.AttributeUseSets))
	if err != nil {
		return noAttributeUseSet, err
	}
	set, err := newAttributeUseSet(finalUses, wildcard)
	if err != nil {
		return noAttributeUseSet, err
	}
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, set)
	return id, nil
}

type attributeWildcardBuilder struct {
	wildcard          wildcardID
	inheritedWildcard wildcardID
	mode              attributeUseMergeMode
}

func (b *attributeWildcardBuilder) addGroup(c *compiler, id wildcardID) error {
	if id == noWildcard {
		return nil
	}
	process := c.rt.Wildcards[id].Process
	if b.wildcard != noWildcard {
		process = c.rt.Wildcards[b.wildcard].Process
	}
	return b.add(c, id, process)
}

func (b *attributeWildcardBuilder) addAnyAttribute(c *compiler, id wildcardID) error {
	if id == noWildcard {
		return nil
	}
	return b.add(c, id, c.rt.Wildcards[id].Process)
}

func (b *attributeWildcardBuilder) add(c *compiler, id wildcardID, process processContents) error {
	if b.wildcard == noWildcard {
		b.wildcard = id
		return nil
	}
	intersectionID, err := c.intersectWildcards(b.wildcard, id, process)
	if err != nil {
		return err
	}
	b.wildcard = intersectionID
	return nil
}

func (b *attributeWildcardBuilder) finish(c *compiler, parentName string) (wildcardID, error) {
	if b.mode == attributeMergeRestriction {
		if b.wildcard == noWildcard {
			return noWildcard, nil
		}
		if b.inheritedWildcard == noWildcard {
			return noWildcard, schemaCompile(ErrSchemaInvalidAttribute, "attribute wildcard restriction requires base wildcard")
		}
		if !c.wildcardSubset(b.wildcard, b.inheritedWildcard) {
			return noWildcard, schemaCompile(ErrSchemaInvalidAttribute, "attribute wildcard restriction is not subset of base")
		}
		return b.wildcard, nil
	}
	if parentName != xsdElemExtension || b.inheritedWildcard == noWildcard {
		return b.wildcard, nil
	}
	if b.wildcard == noWildcard {
		return b.inheritedWildcard, nil
	}
	process := c.rt.Wildcards[b.wildcard].Process
	return c.unionWildcards(b.wildcard, b.inheritedWildcard, process)
}

func newAttributeUseSet(uses []attributeUse, wildcard wildcardID) (attributeUseSet, error) {
	set := attributeUseSet{Uses: uses, wildcard: wildcard}
	if len(uses) != 0 {
		set.Index = make(map[qName]uint32, len(uses))
	}
	for i, use := range uses {
		slot, err := checkedUint32(i, "attribute use limit exceeded")
		if err != nil {
			return attributeUseSet{}, err
		}
		set.Index[use.Name] = slot
		if use.Required {
			set.Required = append(set.Required, slot)
		}
		if use.HasDefault || use.HasFixed {
			set.ValueConstraints = append(set.ValueConstraints, slot)
		}
	}
	return set, nil
}

func (c *compiler) mergeAttributeUse(uses []attributeUse, seen map[qName]int, u attributeUse, mode attributeUseMergeMode, inheritedWildcard wildcardID) ([]attributeUse, error) {
	if i, ok := seen[u.Name]; ok {
		if mode != attributeMergeRestriction && !uses[i].Prohibited && !u.Prohibited {
			return nil, schemaCompile(ErrSchemaDuplicate, "duplicate attribute use")
		}
		if mode == attributeMergeRestriction {
			if err := c.validateAttributeUseRestriction(uses[i], u); err != nil {
				return nil, err
			}
		}
		uses[i] = u
		return uses, nil
	}
	if mode == attributeMergeRestriction && !u.Prohibited {
		if inheritedWildcard == noWildcard || !c.wildcardAllowsQName(inheritedWildcard, u.Name) {
			return nil, schemaCompile(ErrSchemaInvalidAttribute, "new restricted attribute is not allowed by base wildcard")
		}
	}
	seen[u.Name] = len(uses)
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

type attributeUseBase struct {
	refFixedCanonical string
	use               attributeUse
	refHasFixed       bool
}

func (c *compiler) compileAttributeUse(n *rawNode, ctx *schemaContext) (attributeUse, error) {
	base, err := c.compileAttributeUseBase(n, ctx)
	if err != nil {
		return attributeUse{}, err
	}
	use := base.use
	defaultValue, hasDefault := n.attr(xsdAttrDefault)
	fixedValue, hasFixed := n.attr(xsdAttrFixed)
	if hasFixed {
		use.Fixed = fixedValue
		use.FixedCanonical = ""
		use.FixedValue = simpleValue{}
		use.HasFixed = true
	}
	switch mode := n.attrDefault(xsdAttrUse, "optional"); mode {
	case "required":
		if hasDefault {
			return attributeUse{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "required attribute cannot have default")
		}
		use.Required = true
	case "prohibited":
		if hasDefault {
			return attributeUse{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "prohibited attribute cannot have default")
		}
		use.Prohibited = !hasFixed
	case "optional":
	default:
		return attributeUse{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "invalid attribute use "+mode)
	}
	if hasDefault {
		if base.refHasFixed {
			return attributeUse{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "attribute use default conflicts with fixed attribute declaration")
		}
		use.Default = defaultValue
		use.DefaultCanonical = ""
		use.DefaultValue = simpleValue{}
		use.HasDefault = true
	}
	if use.HasDefault && use.HasFixed {
		return attributeUse{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "attribute cannot have both default and fixed")
	}
	decl := attributeDecl{Name: use.Name, Type: use.Type, Default: use.Default, Fixed: use.Fixed, HasDefault: use.HasDefault, HasFixed: use.HasFixed}
	if err := c.validateAttributeValueConstraints(&decl, c.schemaQNameResolver(n)); err != nil {
		return attributeUse{}, withSchemaCompileLocation(n, err)
	}
	if base.refHasFixed && use.HasFixed && decl.FixedCanonical != base.refFixedCanonical {
		return attributeUse{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "attribute use fixed value conflicts with fixed attribute declaration")
	}
	use.DefaultCanonical = decl.DefaultCanonical
	use.FixedCanonical = decl.FixedCanonical
	use.DefaultValue = decl.DefaultValue
	use.FixedValue = decl.FixedValue
	return use, nil
}

func (c *compiler) compileAttributeUseBase(n *rawNode, ctx *schemaContext) (attributeUseBase, error) {
	if ref, ok := n.attr(xsdAttrRef); ok {
		return c.compileAttributeRefUse(n, ctx, ref)
	}
	use, err := c.compileLocalAttributeUse(n, ctx)
	return attributeUseBase{use: use}, err
}

func (c *compiler) compileAttributeRefUse(n *rawNode, ctx *schemaContext, ref string) (attributeUseBase, error) {
	if err := validateKnownAttributes(n, "attribute ref", isAttributeRefAttribute); err != nil {
		return attributeUseBase{}, err
	}
	if children := n.xsContentChildren(); len(children) != 0 {
		return attributeUseBase{}, schemaCompileAt(n, ErrSchemaContentModel, "attribute ref can contain only annotation")
	}
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return attributeUseBase{}, err
	}
	id, err := c.compileAttributeByQName(q)
	if err != nil {
		return attributeUseBase{}, withSchemaCompileLocation(n, err)
	}
	use := attributeUseFromDecl(c.rt.Attributes[id])
	return attributeUseBase{use: use, refHasFixed: use.HasFixed, refFixedCanonical: use.FixedCanonical}, nil
}

func (c *compiler) compileLocalAttributeUse(n *rawNode, ctx *schemaContext) (attributeUse, error) {
	name, ok := n.attr(xsdAttrName)
	if !ok {
		return attributeUse{}, schemaCompileAt(n, ErrSchemaReference, "attribute missing name or ref")
	}
	ns := ""
	form, hasForm := n.attr(xsdAttrForm)
	if hasForm && form != xsdValueQualified && form != xsdValueUnqualified {
		return attributeUse{}, schemaCompileAt(n, ErrSchemaInvalidAttribute, "invalid attribute form "+form)
	}
	if form == xsdValueQualified || (!hasForm && ctx.attrQualified) {
		ns = ctx.targetNS
	}
	nameID, err := c.rt.Names.InternQName(ns, name)
	if err != nil {
		return attributeUse{}, err
	}
	decl, err := c.compileAttributeDecl(n, ctx, nameID)
	if err != nil {
		return attributeUse{}, err
	}
	return attributeUseFromDecl(decl), nil
}

func isAttributeRefAttribute(name string) bool {
	switch name {
	case xsdAttrID, xsdAttrRef, xsdAttrUse, xsdAttrDefault, xsdAttrFixed:
		return true
	default:
		return false
	}
}

func attributeUseFromDecl(decl attributeDecl) attributeUse {
	return attributeUse{
		Name:             decl.Name,
		Type:             decl.Type,
		Default:          decl.Default,
		Fixed:            decl.Fixed,
		DefaultCanonical: decl.DefaultCanonical,
		FixedCanonical:   decl.FixedCanonical,
		DefaultValue:     decl.DefaultValue,
		FixedValue:       decl.FixedValue,
		HasDefault:       decl.HasDefault,
		HasFixed:         decl.HasFixed,
	}
}

func (c *compiler) compileAttributeGroupUse(n *rawNode, ctx *schemaContext) ([]attributeUse, wildcardID, error) {
	if children := n.xsContentChildren(); len(children) != 0 {
		return nil, noWildcard, schemaCompileAt(n, ErrSchemaContentModel, "attributeGroup use can contain only annotation")
	}
	ref, ok := n.attr(xsdAttrRef)
	if !ok {
		return nil, noWildcard, schemaCompileAt(n, ErrSchemaReference, "attributeGroup use missing ref")
	}
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return nil, noWildcard, err
	}
	uses, wildcard, err := c.compileAttributeGroupByQName(q)
	return uses, wildcard, withSchemaCompileLocation(n, err)
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
		return nil, noWildcard, schemaCompileAt(raw.node, ErrSchemaReference, "recursive attribute group "+c.rt.Names.Format(q))
	}
	c.compilingAttrGrp[q] = true
	defer delete(c.compilingAttrGrp, q)
	id, err := c.compileAttributeUses(raw.node, raw.ctx, nil, noWildcard, attributeMergeNormal)
	if err != nil {
		return nil, noWildcard, err
	}
	c.attrGroupDone[q] = id
	set := c.rt.AttributeUseSets[id]
	return set.Uses, set.wildcard, nil
}
