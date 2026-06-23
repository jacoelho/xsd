package compile

import (
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
)

func (c *compiler) compileAttributeByQName(q runtime.QName) (runtime.AttributeID, error) {
	if id, ok := c.attributeDone[q]; ok {
		return id, nil
	}
	label := c.rt.Names.Format(q)
	if c.compilingAttr[q] {
		err := CheckSchemaComponentCycle(SchemaComponentAttribute, true, label)
		if raw, ok := c.attributeRaw[q]; ok {
			return 0, withSchemaCompileLocation(raw.node, err)
		}
		return 0, err
	}
	raw, ok := c.attributeRaw[q]
	if err := CheckSchemaComponentExists(SchemaComponentAttribute, ok, label); err != nil {
		return 0, err
	}
	c.compilingAttr[q] = true
	defer delete(c.compilingAttr, q)
	decl, err := c.compileAttributeDecl(raw.node, raw.ctx, q)
	if err != nil {
		return 0, err
	}
	id, err := c.registerGlobalAttribute(q, decl)
	if err != nil {
		return 0, err
	}
	c.attributeDone[q] = id
	return id, nil
}

func (c *compiler) compileAttributeDecl(n *rawNode, ctx *schemaContext, q runtime.QName) (runtime.AttributeDecl, error) {
	if err := checkAttributeDeclarationChildren(n); err != nil {
		return runtime.AttributeDecl{}, err
	}
	if err := c.validateAttributeDeclName(n, q); err != nil {
		return runtime.AttributeDecl{}, err
	}
	typ := c.rt.Builtin.AnySimpleType
	if typeLex, ok := n.attr(vocab.XSDAttrType); ok {
		if err := ValidateAttributeTypeSource(true, n.firstXS(vocab.XSDElemSimpleType) != nil); err != nil {
			return runtime.AttributeDecl{}, withSchemaCompileLocation(n, err)
		}
		tq, err := c.resolveQNameChecked(n, ctx, typeLex)
		if err != nil {
			return runtime.AttributeDecl{}, err
		}
		id, err := c.compileSimpleByQName(tq)
		if err != nil {
			return runtime.AttributeDecl{}, err
		}
		typ = id
	} else if st := n.firstXS(vocab.XSDElemSimpleType); st != nil {
		id, err := c.compileAnonymousSimple(st, ctx)
		if err != nil {
			return runtime.AttributeDecl{}, err
		}
		typ = id
	}
	decl := runtime.AttributeDecl{Name: q, Type: typ}
	if v, ok := n.attr(vocab.XSDAttrDefault); ok {
		decl.Default = &runtime.ValueConstraint{Lexical: v}
	}
	if v, ok := n.attr(vocab.XSDAttrFixed); ok {
		decl.Fixed = &runtime.ValueConstraint{Lexical: v}
	}
	if err := validateAttributeDeclValueConstraintAdmission(n, decl.Default != nil, decl.Fixed != nil); err != nil {
		return runtime.AttributeDecl{}, err
	}
	if err := c.validateAttributeValueConstraints(&decl, n); err != nil {
		return runtime.AttributeDecl{}, withSchemaCompileLocation(n, err)
	}
	return decl, nil
}

func (c *compiler) validateAttributeValueConstraints(decl *runtime.AttributeDecl, n *rawNode) error {
	if err := c.validateAttributeDeclValueConstraintIdentity(decl); err != nil {
		return err
	}
	if decl.Default == nil && decl.Fixed == nil {
		return nil
	}
	resolve := c.schemaQNameResolver(n)
	if decl.Default != nil {
		vc, err := c.validateValueConstraint(decl.Type, decl.Default.Lexical, resolve, decl.Name, "attribute default")
		if err != nil {
			return err
		}
		decl.Default = vc
	}
	if decl.Fixed != nil {
		vc, err := c.validateValueConstraint(decl.Type, decl.Fixed.Lexical, resolve, decl.Name, "attribute fixed")
		if err != nil {
			return err
		}
		decl.Fixed = vc
	}
	return nil
}

func (c *compiler) validateValueConstraint(id runtime.SimpleTypeID, lexical string, resolve runtime.ResolveQNameParts, owner runtime.QName, label string) (*runtime.ValueConstraint, error) {
	recorder := valueConstraintResolver{resolve: resolve}
	replayResolve := resolve
	if resolve != nil {
		replayResolve = recorder.resolveQName
	}
	value, err := c.validateSimpleValue(id, lexical, replayResolve, runtime.SimpleNeedCanonical|runtime.SimpleNeedIdentity)
	if err != nil {
		return nil, DeclarationValueConstraintError(label, c.rt.Names.Format(owner), err)
	}
	return &runtime.ValueConstraint{
		ResolvedNames: recorder.names,
		Lexical:       lexical,
		Canonical:     value.Canonical,
		Value:         value,
	}, nil
}

type valueConstraintResolver struct {
	resolve runtime.ResolveQNameParts
	names   []runtime.ResolvedValueName
}

func (r *valueConstraintResolver) resolveQName(lexical string) (string, string, bool) {
	ns, local, ok := r.resolve(lexical)
	if ok {
		r.names = append(r.names, runtime.ResolvedValueName{Lexical: lexical, NS: ns, Local: local})
	}
	return ns, local, ok
}

func (c *compiler) schemaQNameResolver(n *rawNode) runtime.ResolveQNameParts {
	return func(lexical string) (string, string, bool) {
		ns, local, err := n.resolveQName(lexical)
		if err != nil {
			return "", "", false
		}
		return ns, local, true
	}
}

func (c *compiler) compileAttributeUses(parent *rawNode, ctx *schemaContext, inherited []runtime.AttributeUse, inheritedWildcard runtime.WildcardID, mode AttributeMergeMode) (runtime.AttributeUseSetID, error) {
	if parent.Name.Local == vocab.XSDElemAttributeGroup {
		if err := checkAttributeGroupDeclarationChildren(parent); err != nil {
			return runtime.NoAttributeUseSet, err
		}
	}
	uses := slices.Clone(inherited)
	merger := NewAttributeUseMerger(inherited, inheritedWildcard, mode)
	wildcards := NewAttributeWildcardBuilder(inheritedWildcard, mode)
	for _, child := range parent.Children {
		if child.Name.Space != runtime.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
			continue
		}
		switch ClassifyAttributeUseChild(child.Name.Local) {
		case AttributeUseChildAttribute:
			u, err := c.compileAttributeUse(child, ctx)
			if err != nil {
				return runtime.NoAttributeUseSet, err
			}
			uses, err = c.mergeAttributeUse(uses, &merger, u)
			if err != nil {
				return runtime.NoAttributeUseSet, withSchemaCompileLocation(child, err)
			}
		case AttributeUseChildGroup:
			groupUses, groupWildcard, err := c.compileAttributeGroupUse(child, ctx)
			if err != nil {
				return runtime.NoAttributeUseSet, err
			}
			for _, u := range groupUses {
				uses, err = c.mergeAttributeUse(uses, &merger, u)
				if err != nil {
					return runtime.NoAttributeUseSet, withSchemaCompileLocation(child, err)
				}
			}
			if err := wildcards.AddGroup(c, groupWildcard); err != nil {
				return runtime.NoAttributeUseSet, withSchemaCompileLocation(child, err)
			}
		case AttributeUseChildWildcard:
			id, err := c.compileAttributeWildcard(child, ctx)
			if err != nil {
				return runtime.NoAttributeUseSet, err
			}
			if err := wildcards.AddAnyAttribute(c, id); err != nil {
				return runtime.NoAttributeUseSet, withSchemaCompileLocation(child, err)
			}
		case AttributeUseChildIgnored:
		}
	}
	declaredWildcard := wildcards.Declared()
	wildcard, err := wildcards.Finish(c, parent.Name.Local == vocab.XSDElemExtension)
	if err != nil {
		return runtime.NoAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	finalUses := RemoveProhibitedAttributeUses(uses)
	id, err := NextAttributeUseSetID(len(c.rt.AttributeUseSets))
	if err != nil {
		return runtime.NoAttributeUseSet, err
	}
	set, err := newAttributeUseSet(finalUses, wildcard, attributeWildcardProvenance{
		base:     inheritedWildcard,
		declared: declaredWildcard,
		derive:   AttributeWildcardDerivation(parent.Name.Local == vocab.XSDElemExtension, mode),
	})
	if err != nil {
		return runtime.NoAttributeUseSet, err
	}
	if err = c.validateAttributeUseSet(set); err != nil {
		return runtime.NoAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	c.rt.AttributeUseSets = append(c.rt.AttributeUseSets, set)
	return id, nil
}

func (c *compiler) mergeAttributeUse(uses []runtime.AttributeUse, merger *AttributeUseMerger, use runtime.AttributeUse) ([]runtime.AttributeUse, error) {
	result, err := merger.Add(&c.rt, uses, use)
	if err != nil {
		return nil, err
	}
	if result.Appended {
		return append(uses, use), nil
	}
	uses[result.Index] = use
	return uses, nil
}

type attributeWildcardProvenance struct {
	base     runtime.WildcardID
	declared runtime.WildcardID
	derive   runtime.AttributeWildcardDerivation
}

func newAttributeUseSet(uses []runtime.AttributeUse, wildcard runtime.WildcardID, provenance attributeWildcardProvenance) (runtime.AttributeUseSet, error) {
	set := runtime.AttributeUseSet{
		Uses:             uses,
		Wildcard:         wildcard,
		WildcardBase:     provenance.base,
		WildcardDeclared: provenance.declared,
		WildcardDerive:   provenance.derive,
	}
	if len(uses) != 0 {
		set.Index = make(map[runtime.QName]uint32, len(uses))
	}
	for i, use := range uses {
		slot, err := CheckedUint32Index(i, "attribute use limit exceeded")
		if err != nil {
			return runtime.AttributeUseSet{}, err
		}
		set.Index[use.Name] = slot
		if use.Required {
			set.Required = append(set.Required, slot)
		}
		if use.Default != nil || use.Fixed != nil {
			set.ValueConstraints = append(set.ValueConstraints, slot)
		}
	}
	return set, nil
}

func (c *compiler) attrUsesAndWildcard(id runtime.AttributeUseSetID) ([]runtime.AttributeUse, runtime.WildcardID) {
	if id == runtime.NoAttributeUseSet {
		return nil, runtime.NoWildcard
	}
	set := c.rt.AttributeUseSets[id]
	return set.Uses, set.Wildcard
}

type attributeUseBase struct {
	refFixed *runtime.ValueConstraint
	use      runtime.AttributeUse
	ref      bool
}

func (c *compiler) compileAttributeUse(n *rawNode, ctx *schemaContext) (runtime.AttributeUse, error) {
	base, err := c.compileAttributeUseBase(n, ctx)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	use := base.use
	defaultValue, hasDefault := n.attr(vocab.XSDAttrDefault)
	fixedValue, hasFixed := n.attr(vocab.XSDAttrFixed)
	if base.ref && hasFixed {
		use.Fixed = &runtime.ValueConstraint{Lexical: fixedValue}
	}
	modeLexical, hasMode := n.attr(vocab.XSDAttrUse)
	mode, err := parseAttributeUseModeChecked(n, modeLexical, hasMode)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	err = validateAttributeUseValueConstraintAdmission(n, mode, hasDefault, hasFixed, base.refFixed != nil)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	modeState, err := applyAttributeUseMode(n, mode, hasFixed)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	use.Required = modeState.Required
	use.Prohibited = modeState.Prohibited
	if base.ref && hasDefault {
		use.Default = &runtime.ValueConstraint{Lexical: defaultValue}
	}
	if base.ref && (hasDefault || hasFixed) {
		decl := runtime.AttributeDecl{Name: use.Name, Type: use.Type}
		if hasDefault {
			decl.Default = &runtime.ValueConstraint{Lexical: defaultValue}
		}
		if hasFixed {
			decl.Fixed = &runtime.ValueConstraint{Lexical: fixedValue}
		}
		if err := c.validateAttributeValueConstraints(&decl, n); err != nil {
			return runtime.AttributeUse{}, withSchemaCompileLocation(n, err)
		}
		if hasDefault {
			use.Default = decl.Default
		}
		if hasFixed {
			use.Fixed = decl.Fixed
		}
	}
	if err := validateAttributeUseFixedValueAdmission(n, runtime.NewValueConstraintIdentity(use.Fixed), runtime.NewValueConstraintIdentity(base.refFixed)); err != nil {
		return runtime.AttributeUse{}, err
	}
	return use, nil
}

func (c *compiler) compileAttributeUseBase(n *rawNode, ctx *schemaContext) (attributeUseBase, error) {
	if ref, ok := n.attr(vocab.XSDAttrRef); ok {
		return c.compileAttributeRefUse(n, ctx, ref)
	}
	use, err := c.compileLocalAttributeUse(n, ctx)
	return attributeUseBase{use: use}, err
}

func (c *compiler) compileAttributeRefUse(n *rawNode, ctx *schemaContext, ref string) (attributeUseBase, error) {
	if err := checkAttributeRefAttributes(n); err != nil {
		return attributeUseBase{}, err
	}
	if err := checkAttributeRefChildren(n); err != nil {
		return attributeUseBase{}, err
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
	base := attributeUseBase{use: use, ref: true}
	if use.Fixed != nil {
		base.refFixed = use.Fixed
	}
	return base, nil
}

func (c *compiler) compileLocalAttributeUse(n *rawNode, ctx *schemaContext) (runtime.AttributeUse, error) {
	if err := checkAttributeUseSource(n); err != nil {
		return runtime.AttributeUse{}, err
	}
	name, _ := n.attr(vocab.XSDAttrName)
	ns := ""
	form, hasForm := n.attr(vocab.XSDAttrForm)
	qualified, err := ParseAttributeFormAttr(FormAttr{
		Value:            form,
		HasValue:         hasForm,
		DefaultQualified: ctx.attrQualified,
	})
	if err != nil {
		return runtime.AttributeUse{}, withSchemaCompileLocation(n, err)
	}
	if qualified {
		ns = ctx.targetNS
	}
	nameID, err := c.names.InternQName(ns, name)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	decl, err := c.compileAttributeDecl(n, ctx, nameID)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	return attributeUseFromDecl(decl), nil
}

func attributeUseFromDecl(decl runtime.AttributeDecl) runtime.AttributeUse {
	return runtime.AttributeUse{
		Name:    decl.Name,
		Type:    decl.Type,
		Default: decl.Default,
		Fixed:   decl.Fixed,
	}
}

func (c *compiler) compileAttributeGroupUse(n *rawNode, ctx *schemaContext) ([]runtime.AttributeUse, runtime.WildcardID, error) {
	if err := checkAttributeGroupUseChildren(n); err != nil {
		return nil, runtime.NoWildcard, err
	}
	if err := checkAttributeGroupUseSource(n); err != nil {
		return nil, runtime.NoWildcard, err
	}
	ref, _ := n.attr(vocab.XSDAttrRef)
	q, err := c.resolveQNameChecked(n, ctx, ref)
	if err != nil {
		return nil, runtime.NoWildcard, err
	}
	uses, wildcard, err := c.compileAttributeGroupByQName(q)
	return uses, wildcard, withSchemaCompileLocation(n, err)
}

func (c *compiler) compileAttributeGroupByQName(q runtime.QName) ([]runtime.AttributeUse, runtime.WildcardID, error) {
	if id, ok := c.attrGroupDone[q]; ok {
		set := c.rt.AttributeUseSets[id]
		return set.Uses, set.Wildcard, nil
	}
	label := c.rt.Names.Format(q)
	raw, ok := c.attrGroupRaw[q]
	if err := CheckSchemaComponentExists(SchemaComponentAttributeGroup, ok, label); err != nil {
		return nil, runtime.NoWildcard, err
	}
	if c.compilingAttrGrp[q] {
		err := CheckSchemaComponentRecursion(SchemaComponentAttributeGroup, true, label)
		return nil, runtime.NoWildcard, withSchemaCompileLocation(raw.node, err)
	}
	c.compilingAttrGrp[q] = true
	defer delete(c.compilingAttrGrp, q)
	id, err := c.compileAttributeUses(raw.node, raw.ctx, nil, runtime.NoWildcard, AttributeMergeNormal)
	if err != nil {
		return nil, runtime.NoWildcard, err
	}
	c.attrGroupDone[q] = id
	set := c.rt.AttributeUseSets[id]
	return set.Uses, set.Wildcard, nil
}
