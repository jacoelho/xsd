package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) compileAttributeByQName(q runtime.QName) (runtime.AttributeID, error) {
	raw, exists := c.attributeRaw[q]
	var source *rawNode
	if exists {
		source = raw.node
	}
	if err := c.spendComponentDependency(source); err != nil {
		return 0, err
	}
	if id, ok := c.attributeDone[q]; ok {
		return id, nil
	}
	label := c.rt.formatName(q)
	if !exists {
		return 0, SchemaComponentMissingError(SchemaComponentAttribute, label)
	}
	leave, err := c.enterComponent(raw.node)
	if err != nil {
		return 0, err
	}
	defer leave()
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
	typ, err := c.compileAttributeDeclType(n, ctx)
	if err != nil {
		return runtime.AttributeDecl{}, err
	}
	decl := runtime.AttributeDecl{Name: q, Type: typ}
	readAttributeDeclValueConstraints(n, &decl)
	if err := validateAttributeDeclValueConstraintAtNode(n, runtime.DeclarationValueConstraintOf(decl.Default, decl.Fixed)); err != nil {
		return runtime.AttributeDecl{}, err
	}
	if err := c.validateAttributeValueConstraints(&decl, n); err != nil {
		return runtime.AttributeDecl{}, withSchemaCompileLocation(n, err)
	}
	return decl, nil
}

func (c *compiler) compileAttributeDeclType(n *rawNode, ctx *schemaContext) (runtime.SimpleTypeID, error) {
	if typeLex, ok := n.attr(vocab.XSDAttrType); ok {
		source := AttributeTypeSource{
			Type:               LexicalAttribute{Value: typeLex, Present: true},
			HasSimpleTypeChild: n.firstXS(vocab.XSDElemSimpleType) != nil,
		}
		if err := ValidateAttributeTypeSource(source); err != nil {
			return runtime.NoSimpleType, withSchemaCompileLocation(n, err)
		}
		q, err := c.resolveQNameChecked(n, ctx, typeLex)
		if err != nil {
			return runtime.NoSimpleType, err
		}
		return c.compileSimpleTypeReference(n, q)
	}
	if simple := n.firstXS(vocab.XSDElemSimpleType); simple != nil {
		return c.compileAnonymousSimple(simple, ctx)
	}
	return c.rt.builtinIDs().AnySimpleType, nil
}

func readAttributeDeclValueConstraints(n *rawNode, decl *runtime.AttributeDecl) {
	if value, ok := n.attr(vocab.XSDAttrDefault); ok {
		decl.Default = &runtime.ValueConstraint{Lexical: value}
	}
	if value, ok := n.attr(vocab.XSDAttrFixed); ok {
		decl.Fixed = &runtime.ValueConstraint{Lexical: value}
	}
}

func (c *compiler) validateAttributeValueConstraints(decl *runtime.AttributeDecl, n *rawNode) error {
	if !runtime.ValidSimpleTypeID(decl.Type, len(c.simpleTypeUnavailable)) {
		return xsderrors.InternalInvariant("attribute value constraint references invalid simple type")
	}
	if c.simpleTypeUnavailable[decl.Type] {
		decl.Default = nil
		decl.Fixed = nil
		return nil
	}
	if err := c.validateAttributeDeclValueConstraintIdentity(decl); err != nil {
		return err
	}
	if decl.Default == nil && decl.Fixed == nil {
		return nil
	}
	resolve := schemaQNameResolver(n)
	if err := c.validateAttributeConstraint(&decl.Default, decl, resolve, "attribute default"); err != nil {
		return err
	}
	return c.validateAttributeConstraint(&decl.Fixed, decl, resolve, "attribute fixed")
}

func (c *compiler) validateAttributeConstraint(constraint **runtime.ValueConstraint, decl *runtime.AttributeDecl, resolve runtime.ResolveQNameParts, label string) error {
	if *constraint == nil {
		return nil
	}
	validated, err := c.validateValueConstraint(decl.Type, (*constraint).Lexical, resolve, decl.Name, label)
	if err != nil {
		return err
	}
	*constraint = validated
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
		return nil, DeclarationValueConstraintError(label, c.rt.formatName(owner), err)
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

func (r *valueConstraintResolver) resolveQName(lexical string) (namespace, local string, ok bool) {
	ns, local, ok := r.resolve(lexical)
	if ok {
		r.names = append(r.names, runtime.ResolvedValueName{Lexical: lexical, NS: ns, Local: local})
	}
	return ns, local, ok
}

func schemaQNameResolver(n *rawNode) runtime.ResolveQNameParts {
	return func(lexical string) (string, string, bool) {
		name, err := n.resolveQName(lexical)
		if err != nil {
			return "", "", false
		}
		return name.Space, name.Local, true
	}
}

func (c *compiler) compileAttributeUses(parent *rawNode, ctx *schemaContext, inherited []runtime.AttributeUse, inheritedWildcard runtime.WildcardID, mode AttributeMergeMode) (runtime.AttributeUseSetID, error) {
	if parent.Name.Local == vocab.XSDElemAttributeGroup {
		if err := checkAttributeGroupDeclarationChildren(parent); err != nil {
			return runtime.NoAttributeUseSet, err
		}
	}
	compilation := attributeUseCompilation{
		compiler:  c,
		ctx:       ctx,
		uses:      inherited,
		merger:    NewAttributeUseMerger(inherited, inheritedWildcard, mode),
		wildcards: NewAttributeWildcardBuilder(inheritedWildcard, mode),
	}
	for _, child := range parent.Children {
		if child.Name.Space != vocab.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
			continue
		}
		if err := compilation.add(child); err != nil {
			return runtime.NoAttributeUseSet, err
		}
	}
	return compilation.finish(parent, inheritedWildcard, mode)
}

type attributeUseCompilation struct {
	compiler  *compiler
	ctx       *schemaContext
	merger    AttributeUseMerger
	uses      []runtime.AttributeUse
	wildcards AttributeWildcardBuilder
}

func (b *attributeUseCompilation) add(child *rawNode) error {
	switch ClassifyAttributeUseChild(child.Name.Local) {
	case AttributeUseChildAttribute:
		return b.addAttribute(child)
	case AttributeUseChildGroup:
		return b.addGroup(child)
	case AttributeUseChildWildcard:
		return b.addWildcard(child)
	case AttributeUseChildIgnored:
		return nil
	default:
	}
	return nil
}

func (b *attributeUseCompilation) addAttribute(child *rawNode) error {
	use, err := b.compiler.compileAttributeUse(child, b.ctx)
	if err != nil {
		return err
	}
	b.uses, err = b.compiler.mergeAttributeUse(b.uses, &b.merger, use)
	return withSchemaCompileLocation(child, err)
}

func (b *attributeUseCompilation) addGroup(child *rawNode) error {
	uses, wildcard, err := b.compiler.compileAttributeGroupUse(child, b.ctx)
	if err != nil {
		return err
	}
	for _, use := range uses {
		b.uses, err = b.compiler.mergeAttributeUse(b.uses, &b.merger, use)
		if err != nil {
			return withSchemaCompileLocation(child, err)
		}
	}
	if err := b.wildcards.AddGroup(b.compiler, wildcard); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (b *attributeUseCompilation) addWildcard(child *rawNode) error {
	id, err := b.compiler.compileAttributeWildcard(child, b.ctx)
	if err != nil {
		return err
	}
	if err := b.wildcards.AddAnyAttribute(b.compiler, id); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (b *attributeUseCompilation) finish(parent *rawNode, inheritedWildcard runtime.WildcardID, mode AttributeMergeMode) (runtime.AttributeUseSetID, error) {
	declaredWildcard := b.wildcards.Declared()
	wildcard, err := b.wildcards.Finish(b.compiler)
	if err != nil {
		return runtime.NoAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	derivation, err := AttributeWildcardDerivation(mode)
	if err != nil {
		return runtime.NoAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	finalUses := RemoveProhibitedAttributeUses(b.uses)
	set, err := newAttributeUseSet(finalUses, wildcard, attributeWildcardProvenance{
		base:     inheritedWildcard,
		declared: declaredWildcard,
		derive:   derivation,
	})
	if err != nil {
		return runtime.NoAttributeUseSet, err
	}
	if err = b.compiler.validateAttributeUseSet(set); err != nil {
		return runtime.NoAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	return b.compiler.addAttributeUseSet(set)
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
	overrides := readAttributeUseOverrides(n)
	applyAttributeUseFixedOverride(&use, base, overrides)
	mode, err := validateAttributeUseMode(n, overrides, base.refFixed != nil)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	modeState, err := applyAttributeUseModeAtNode(n, mode, overrides.hasFixed)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	use.Required = modeState.Required
	use.Prohibited = modeState.Prohibited
	applyAttributeUseDefaultOverride(&use, base, overrides)
	if err := c.validateAttributeUseOverrides(n, &use, base, overrides); err != nil {
		return runtime.AttributeUse{}, err
	}
	if err := validateAttributeUseFixedValueAtNode(n, runtime.NewValueConstraintIdentity(use.Fixed), runtime.NewValueConstraintIdentity(base.refFixed)); err != nil {
		return runtime.AttributeUse{}, err
	}
	return use, nil
}

type attributeUseOverrides struct {
	defaultValue string
	fixedValue   string
	mode         LexicalAttribute
	hasDefault   bool
	hasFixed     bool
}

func readAttributeUseOverrides(n *rawNode) attributeUseOverrides {
	defaultValue, hasDefault := n.attr(vocab.XSDAttrDefault)
	fixedValue, hasFixed := n.attr(vocab.XSDAttrFixed)
	return attributeUseOverrides{
		defaultValue: defaultValue, fixedValue: fixedValue, mode: rawLexicalAttribute(n, vocab.XSDAttrUse),
		hasDefault: hasDefault, hasFixed: hasFixed,
	}
}

func applyAttributeUseFixedOverride(use *runtime.AttributeUse, base attributeUseBase, overrides attributeUseOverrides) {
	if base.ref && overrides.hasFixed {
		use.Fixed = &runtime.ValueConstraint{Lexical: overrides.fixedValue}
		use.FixedFromDeclaration = false
	}
}

func validateAttributeUseMode(n *rawNode, overrides attributeUseOverrides, inheritedFixed bool) (AttributeUseMode, error) {
	mode, err := parseAttributeUseModeChecked(n, overrides.mode)
	if err != nil {
		return AttributeUseOptional, err
	}
	if err := validateAttributeUseValueConstraintAtNode(n, mode, overrides.hasDefault, overrides.hasFixed, inheritedFixed); err != nil {
		return AttributeUseOptional, err
	}
	return mode, nil
}

func applyAttributeUseDefaultOverride(use *runtime.AttributeUse, base attributeUseBase, overrides attributeUseOverrides) {
	if base.ref && overrides.hasDefault {
		use.Default = &runtime.ValueConstraint{Lexical: overrides.defaultValue}
	}
}

func (c *compiler) validateAttributeUseOverrides(n *rawNode, use *runtime.AttributeUse, base attributeUseBase, overrides attributeUseOverrides) error {
	if !base.ref || !overrides.hasDefault && !overrides.hasFixed {
		return nil
	}
	decl := runtime.AttributeDecl{Name: use.Name, Type: use.Type}
	if overrides.hasDefault {
		decl.Default = &runtime.ValueConstraint{Lexical: overrides.defaultValue}
	}
	if overrides.hasFixed {
		decl.Fixed = &runtime.ValueConstraint{Lexical: overrides.fixedValue}
	}
	if err := c.validateAttributeValueConstraints(&decl, n); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	applyValidatedAttributeUseOverrides(use, decl, overrides)
	return nil
}

func applyValidatedAttributeUseOverrides(use *runtime.AttributeUse, decl runtime.AttributeDecl, overrides attributeUseOverrides) {
	if overrides.hasDefault {
		use.Default = decl.Default
	}
	if overrides.hasFixed {
		use.Fixed = decl.Fixed
		use.FixedFromDeclaration = false
	}
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
	use := c.rt.attributeUse(id)
	base := attributeUseBase{use: use, ref: true}
	if use.Fixed != nil {
		base.refFixed = use.Fixed
	}
	return base, nil
}

func (c *compiler) compileLocalAttributeUse(n *rawNode, ctx *schemaContext) (runtime.AttributeUse, error) {
	if err := c.spendComponentDependency(n); err != nil {
		return runtime.AttributeUse{}, err
	}
	leave, err := c.enterComponent(n)
	if err != nil {
		return runtime.AttributeUse{}, err
	}
	defer leave()
	if err = checkAttributeUseSource(n); err != nil {
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
	nameID, err := c.rt.internQName(ns, name)
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
	label := c.rt.formatName(q)
	raw, exists := c.attrGroupRaw[q]
	if c.compilingAttrGrp[q] {
		err := SchemaComponentRecursionError(SchemaComponentAttributeGroup, label)
		return nil, runtime.NoWildcard, withSchemaCompileLocation(raw.node, err)
	}
	var source *rawNode
	if exists {
		source = raw.node
	}
	if err := c.spendComponentDependency(source); err != nil {
		return nil, runtime.NoWildcard, err
	}
	if id, ok := c.attrGroupDone[q]; ok {
		uses, wildcard := c.rt.attributeUsesAndWildcard(id)
		return uses, wildcard, nil
	}
	if !exists {
		return nil, runtime.NoWildcard, SchemaComponentMissingError(SchemaComponentAttributeGroup, label)
	}
	leave, err := c.enterComponent(raw.node)
	if err != nil {
		return nil, runtime.NoWildcard, err
	}
	defer leave()
	c.compilingAttrGrp[q] = true
	defer delete(c.compilingAttrGrp, q)
	id, err := c.compileAttributeUses(raw.node, raw.ctx, nil, runtime.NoWildcard, AttributeMergeDirect)
	if err != nil {
		return nil, runtime.NoWildcard, err
	}
	c.attrGroupDone[q] = id
	uses, wildcard := c.rt.attributeUsesAndWildcard(id)
	return uses, wildcard, nil
}
