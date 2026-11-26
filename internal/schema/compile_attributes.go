package schema

import (
	"encoding/xml"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

func (c *compiler) compileAttributeByQName(q QName) (result AttributeID, err error) {
	raw, exists := c.attributeComponents[q]
	if err = c.spendComponentDependency(raw.sourceNode()); err != nil {
		return 0, err
	}
	if id, ok := c.rt.GlobalAttributes[q]; ok {
		return id, nil
	}
	label := c.rt.formatName(q)
	if !exists {
		return 0, schemaComponentMissingError(schemaComponentAttribute, label)
	}
	scope, err := c.enterComponent(raw.sourceNode())
	if err != nil {
		return 0, err
	}
	defer scope.leave(&err)
	decl, err := c.compileAttributeDecl(raw.ctx, raw.sourceNode(), q)
	if err != nil {
		return 0, err
	}
	id, err := c.registerGlobalAttribute(q, decl)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (c *compiler) compileAttributeDecl(ctx *schemaSetDocument, n *schemaNode, q QName) (attributeDecl, error) {
	if err := c.validateAttributeDeclName(n, q); err != nil {
		return attributeDecl{}, err
	}
	typ, err := c.compileAttributeDeclType(ctx, n)
	if err != nil {
		return attributeDecl{}, err
	}
	if err = c.admitNotationTypeUse(n, typ); err != nil {
		return attributeDecl{}, err
	}
	source, err := requiredSemanticSource[*schemaAttributeSource](n)
	if err != nil {
		return attributeDecl{}, err
	}
	constraint, err := sourceValueConstraint(n, source.Default, source.Fixed)
	if err != nil {
		return attributeDecl{}, err
	}
	validated, err := c.validateAttributeValueConstraint(constraint, q, typ, n, valueConstraintOwnerAttributeDeclaration)
	if err != nil {
		return attributeDecl{}, withSchemaCompileLocation(n, err)
	}
	return attributeDecl{Name: q, Type: typ, ValueConstraint: validated}, nil
}

func (c *compiler) compileAttributeDeclType(ctx *schemaSetDocument, n *schemaNode) (SimpleTypeID, error) {
	source, err := requiredSemanticSource[*schemaAttributeSource](n)
	if err != nil {
		return NoSimpleType, err
	}
	if typeLex := source.Type.Lexical; typeLex.Present {
		q, err := c.referenceQName(ctx, n, source.Type.Name)
		if err != nil {
			return NoSimpleType, err
		}
		return c.compileSimpleTypeReference(n, q)
	}
	if simple := n.firstXS(vocab.XSDElemSimpleType); simple != nil {
		return c.compileAnonymousSimple(ctx, simple)
	}
	return valuepkg.AnySimpleTypeID, nil
}

// validateAttributeValueConstraint returns constraint, which owner carries
// for the attribute named name, validated against typ, or nil when constraint
// is nil or typ is unavailable.
func (c *compiler) validateAttributeValueConstraint(
	constraint *valueConstraint,
	name QName,
	typ SimpleTypeID,
	n *schemaNode,
	owner valueConstraintOwner,
) (*valueConstraint, error) {
	if !validSimpleTypeID(typ, len(c.rt.simpleTypeUnavailable)) {
		return nil, xsderrors.InternalInvariant("attribute value constraint references invalid simple type", nil)
	}
	if c.rt.simpleTypeUnavailable[typ] {
		return nil, nil //nolint:nilnil // A nil constraint is the absent {value constraint}; see valueConstraint.
	}
	if err := validateValueConstraintIdentity(&c.rt, typ, constraint, owner); err != nil {
		return nil, invalidAttributeError(err)
	}
	if constraint == nil {
		return nil, nil //nolint:nilnil // A nil constraint is the absent {value constraint}; see valueConstraint.
	}
	return c.validateValueConstraint(typ, constraint, schemaQNameResolver(n), name, "attribute")
}

// validateValueConstraint validates constraint, which the component kind
// names for owner, against id.
func (c *compiler) validateValueConstraint(id SimpleTypeID, constraint *valueConstraint, resolve valuepkg.QNameResolver, owner QName, kind string) (*valueConstraint, error) {
	lexical := constraint.Lexical
	validated, resolvedNames, err := c.validateValue(id, lexical, resolve, valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	if err != nil {
		return nil, schemaValueError(c.limits, xsderrors.CodeSchemaFacet, "invalid "+kind+" "+constraint.kind().label()+" value for "+c.rt.formatName(owner), err)
	}
	canonical := validated.CanonicalText()
	return &valueConstraint{
		ResolvedNames: resolvedNames,
		Lexical:       lexical,
		Canonical:     canonical,
		Value:         validated,
		Fixed:         constraint.Fixed,
	}, nil
}

type valueConstraintResolver struct {
	resolve valuepkg.QNameResolver
	names   []resolvedValueName
}

func (r *valueConstraintResolver) resolveQName(lexical string) (valuepkg.ExpandedName, bool) {
	name, ok := r.resolve(lexical)
	if ok {
		r.names = append(r.names, resolvedValueName{Lexical: lexical, NS: name.Namespace, Local: name.Local})
	}
	return name, ok
}

func schemaQNameResolver(n *schemaNode) valuepkg.QNameResolver {
	return func(lexical string) (valuepkg.ExpandedName, bool) {
		name, err := n.resolveQName(lexical)
		if err != nil {
			return valuepkg.ExpandedName{}, false
		}
		return valuepkg.ExpandedName{Namespace: name.Space, Local: name.Local}, true
	}
}

func (c *compiler) compileAttributeUses(ctx *schemaSetDocument, parent *schemaNode, inherited []attributeUse, inheritedWildcard WildcardID, mode attributeMergeMode) (attributeUseSetID, error) {
	if id, ok, err := c.compileDirectAttributeGroupUse(ctx, parent, inherited, inheritedWildcard, mode); ok {
		return id, err
	}
	compilation := newAttributeUseCompilation(ctx, c, inherited, inheritedWildcard, mode)
	if err := compilation.addChildren(parent); err != nil {
		return noAttributeUseSet, err
	}
	return compilation.finish(parent, inheritedWildcard, mode)
}

func (c *compiler) compileDirectAttributeGroupUse(ctx *schemaSetDocument, parent *schemaNode, inherited []attributeUse, inheritedWildcard WildcardID, mode attributeMergeMode) (attributeUseSetID, bool, error) {
	if mode != attributeMergeDirect || len(inherited) != 0 || inheritedWildcard != NoWildcard {
		return noAttributeUseSet, false, nil
	}
	child, ok := soleAttributeGroupChild(parent)
	if !ok {
		return noAttributeUseSet, false, nil
	}
	id, err := c.compileAttributeGroupUseID(ctx, child)
	if err != nil {
		return noAttributeUseSet, true, withSchemaCompileLocation(child, err)
	}
	return id, true, nil
}

func soleAttributeGroupChild(parent *schemaNode) (*schemaNode, bool) {
	var group *schemaNode
	for child := range parent.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		if child.local != vocab.XSDElemAttributeGroup || group != nil {
			return nil, false
		}
		group = child
	}
	return group, group != nil
}

// attributeUseCompilation keeps source directives separate from actual uses;
// inherited uses remain immutable until the completed set is assembled.
type attributeUseCompilation struct {
	compiler          *compiler
	ctx               *schemaSetDocument
	seen              map[QName]uint32
	inheritedIndex    map[QName]int
	prohibited        map[QName]struct{}
	uses              []attributeUse
	inherited         []attributeUse
	wildcards         attributeWildcardBuilder
	inheritedWildcard WildcardID
	mode              attributeMergeMode
}

func newAttributeUseCompilation(ctx *schemaSetDocument, c *compiler, inherited []attributeUse, inheritedWildcard WildcardID, mode attributeMergeMode) attributeUseCompilation {
	var inheritedIndex map[QName]int
	if len(inherited) != 0 {
		inheritedIndex = make(map[QName]int, len(inherited))
	}
	for i := range inherited {
		inheritedIndex[inherited[i].Name] = i
	}
	return attributeUseCompilation{
		compiler:          c,
		ctx:               ctx,
		inheritedIndex:    inheritedIndex,
		inherited:         inherited,
		wildcards:         newAttributeWildcardBuilder(inheritedWildcard, mode),
		inheritedWildcard: inheritedWildcard,
		mode:              mode,
	}
}

func (b *attributeUseCompilation) addChildren(parent *schemaNode) error {
	for child := range parent.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		if err := b.add(child); err != nil {
			return err
		}
	}
	return nil
}

func (b *attributeUseCompilation) add(child *schemaNode) error {
	switch child.local {
	case vocab.XSDElemAttribute:
		return b.addAttribute(child)
	case vocab.XSDElemAttributeGroup:
		return b.addGroup(child)
	case vocab.XSDElemAnyAttribute:
		return b.addWildcard(child)
	}
	return nil
}

func (b *attributeUseCompilation) addAttribute(child *schemaNode) error {
	overrides, err := readAttributeUseOverrides(child)
	if err != nil {
		return err
	}
	if overrides.mode == attributeUseProhibited {
		name, admissionErr := b.compiler.admitAttributeProhibition(b.ctx, child)
		if admissionErr != nil {
			return admissionErr
		}
		b.addProhibition(name)
		return nil
	}
	use, err := b.compiler.compileAttributeUse(b.ctx, child, overrides)
	if err != nil {
		return err
	}
	return withSchemaCompileLocation(child, b.mergeUse(use))
}

func (b *attributeUseCompilation) addGroup(child *schemaNode) error {
	uses, wildcard, err := b.compiler.compileAttributeGroupUse(b.ctx, child)
	if err != nil {
		return err
	}
	for _, use := range uses {
		if err := b.mergeUse(use); err != nil {
			return withSchemaCompileLocation(child, err)
		}
	}
	if err := b.wildcards.addGroup(b.compiler, wildcard); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (b *attributeUseCompilation) addWildcard(child *schemaNode) error {
	id, err := b.compiler.compileAttributeWildcard(b.ctx, child)
	if err != nil {
		return err
	}
	if err := b.wildcards.addAnyAttribute(b.compiler, id); err != nil {
		return withSchemaCompileLocation(child, err)
	}
	return nil
}

func (b *attributeUseCompilation) finish(parent *schemaNode, inheritedWildcard WildcardID, mode attributeMergeMode) (attributeUseSetID, error) {
	declaredWildcard := b.wildcards.declared()
	wildcard, err := b.wildcards.finish(b.compiler)
	if err != nil {
		return noAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	derivation, err := schemaAttributeWildcardDerivation(mode)
	if err != nil {
		return noAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	finalUses := b.finalUses()
	// Finalization transfers the explicit-use index to the completed set.
	for i := len(b.uses); i < len(finalUses); i++ {
		slot, slotErr := checkedUint32Index(i, "attribute use limit exceeded")
		if slotErr != nil {
			return noAttributeUseSet, slotErr
		}
		if b.seen == nil {
			b.seen = make(map[QName]uint32, len(finalUses))
		}
		b.seen[finalUses[i].Name] = slot
	}
	set := attributeUseSet{
		Uses: finalUses, Index: b.seen,
		Wildcard: wildcard, WildcardBase: inheritedWildcard,
		WildcardDeclared: declaredWildcard, WildcardDerive: derivation,
	}
	if err = b.validateRequiredInheritedUses(set); err != nil {
		return noAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	if err = b.compiler.validateAttributeUseSet(set); err != nil {
		return noAttributeUseSet, withSchemaCompileLocation(parent, err)
	}
	return b.compiler.addAttributeUseSet(set)
}

func (b *attributeUseCompilation) validateRequiredInheritedUses(set attributeUseSet) error {
	if b.mode != attributeMergeRestriction {
		return nil
	}
	for _, use := range b.inherited {
		if _, present := set.Index[use.Name]; use.Required && !present {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "complex restriction omits required base attribute", nil)
		}
	}
	return nil
}

func (b *attributeUseCompilation) finalUses() []attributeUse {
	uses := b.uses
	for _, inherited := range b.inherited {
		if _, explicit := b.seen[inherited.Name]; explicit {
			continue
		}
		if _, prohibited := b.prohibited[inherited.Name]; prohibited {
			continue
		}
		uses = append(uses, inherited)
	}
	return uses
}

func (b *attributeUseCompilation) mergeUse(use attributeUse) error {
	if b.mode == attributeMergeInvalid {
		return xsderrors.InternalInvariant("invalid attribute merge mode", nil)
	}
	if b.mode > attributeMergeRestriction {
		return xsderrors.InternalInvariant("unknown attribute merge mode", nil)
	}
	if index, ok := b.seen[use.Name]; ok {
		if sameAttributeUseComponent(use, b.uses[index]) {
			return nil
		}
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate attribute use", nil)
	}
	if err := b.validateInheritedUse(use); err != nil {
		return err
	}
	slot, err := checkedUint32Index(len(b.uses), "attribute use limit exceeded")
	if err != nil {
		return err
	}
	if b.seen == nil {
		b.seen = make(map[QName]uint32)
	}
	b.seen[use.Name] = slot
	b.uses = append(b.uses, use)
	return nil
}

func (b *attributeUseCompilation) addProhibition(name QName) {
	if b.mode != attributeMergeRestriction {
		return
	}
	if b.prohibited == nil {
		b.prohibited = make(map[QName]struct{})
	}
	b.prohibited[name] = struct{}{}
}

func sameAttributeUseComponent(first, second attributeUse) bool {
	return first.componentID != 0 && first.componentID == second.componentID
}

func (b *attributeUseCompilation) validateInheritedUse(use attributeUse) error {
	index, inherited := b.inheritedIndex[use.Name]
	if b.mode != attributeMergeRestriction {
		if inherited && !sameAttributeUseComponent(use, b.inherited[index]) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate attribute use", nil)
		}
		return nil
	}
	if !inherited {
		if !b.inheritedWildcardAllows(use.Name) {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "new restricted attribute is not allowed by base wildcard", nil)
		}
		return nil
	}
	return b.validateRestrictedUse(b.inherited[index], use)
}

func (b *attributeUseCompilation) validateRestrictedUse(base, use attributeUse) error {
	if err := validateAttributeUseRestriction(&b.compiler.rt, base, use); err != nil {
		return invalidAttributeError(err)
	}
	return nil
}

func (b *attributeUseCompilation) inheritedWildcardAllows(name QName) bool {
	if b.inheritedWildcard == NoWildcard {
		return false
	}
	wildcard, ok := wildcardByID(b.compiler.rt.Wildcards, b.inheritedWildcard)
	return ok && wildcardAllowsNamespace(wildcard, name.Namespace)
}

type attributeUseBase struct {
	refFixed *valueConstraint
	use      attributeUse
	ref      bool
}

// Source admission consumes prohibitions before this boundary; a returned use
// owns validated constraints and a component identity.
func (c *compiler) compileAttributeUse(ctx *schemaSetDocument, n *schemaNode, overrides attributeUseOverrides) (attributeUse, error) {
	base, err := c.compileAttributeUseBase(ctx, n)
	if err != nil {
		return attributeUse{}, err
	}
	use := base.use
	if err = validateAttributeUseValueConstraintAtNode(n, overrides.defaultAttr.Present, base.refFixed != nil); err != nil {
		return attributeUse{}, err
	}
	use.Required = overrides.mode == attributeUseRequired
	if err = c.validateAttributeUseOverrides(n, &use, base, overrides); err != nil {
		return attributeUse{}, err
	}
	if err = validateAttributeUseFixedValueAtNode(n, newValueConstraintIdentity(use.ValueConstraint.fixedConstraint()), newValueConstraintIdentity(base.refFixed)); err != nil {
		return attributeUse{}, err
	}
	use.componentID, err = c.allocateAttributeUseID()
	if err != nil {
		return attributeUse{}, err
	}
	return use, nil
}

func (c *compiler) allocateAttributeUseID() (uint32, error) {
	if c.attributeUseCount == ^uint32(0) {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "attribute use identity limit exceeded", nil)
	}
	c.attributeUseCount++
	return c.attributeUseCount, nil
}

type attributeUseOverrides struct {
	defaultAttr lexicalAttribute
	fixedAttr   lexicalAttribute
	mode        attributeUseMode
}

func readAttributeUseOverrides(n *schemaNode) (attributeUseOverrides, error) {
	source, err := requiredSemanticSource[*schemaAttributeSource](n)
	if err != nil {
		return attributeUseOverrides{}, err
	}
	return attributeUseOverrides{defaultAttr: source.Default, fixedAttr: source.Fixed, mode: source.Use}, nil
}

func (c *compiler) validateAttributeUseOverrides(n *schemaNode, use *attributeUse, base attributeUseBase, overrides attributeUseOverrides) error {
	if !base.ref {
		return nil
	}
	local, err := sourceValueConstraint(n, overrides.defaultAttr, overrides.fixedAttr)
	if err != nil || local == nil {
		return err
	}
	validated, err := c.validateAttributeValueConstraint(local, use.Name, use.Type, n, valueConstraintOwnerAttributeUse)
	if err != nil {
		return withSchemaCompileLocation(n, err)
	}
	// The local constraint replaces the declaration's rather than joining it
	// (Structures §3.2.2 attribute use mapping).
	use.ValueConstraint = validated
	return nil
}

func (c *compiler) compileAttributeUseBase(ctx *schemaSetDocument, n *schemaNode) (attributeUseBase, error) {
	source, err := requiredSemanticSource[*schemaAttributeSource](n)
	if err != nil {
		return attributeUseBase{}, err
	}
	if source.Ref.Lexical.Present {
		return c.compileAttributeRefUse(ctx, n, source.Ref.Name)
	}
	use, err := c.compileLocalAttributeUse(ctx, n)
	return attributeUseBase{use: use}, err
}

func (c *compiler) compileAttributeRefUse(ctx *schemaSetDocument, n *schemaNode, ref xml.Name) (attributeUseBase, error) {
	q, err := c.referenceQName(ctx, n, ref)
	if err != nil {
		return attributeUseBase{}, err
	}
	id, err := c.compileAttributeByQName(q)
	if err != nil {
		return attributeUseBase{}, withSchemaCompileLocation(n, err)
	}
	use := c.rt.attributeUse(id)
	return attributeUseBase{use: use, ref: true, refFixed: use.ValueConstraint.fixedConstraint()}, nil
}

func (c *compiler) compileLocalAttributeUse(ctx *schemaSetDocument, n *schemaNode) (result attributeUse, err error) {
	if err = c.spendComponentDependency(n); err != nil {
		return attributeUse{}, err
	}
	scope, err := c.enterComponent(n)
	if err != nil {
		return attributeUse{}, err
	}
	defer scope.leave(&err)
	name, err := c.localAttributeUseName(ctx, n)
	if err != nil {
		return attributeUse{}, err
	}
	decl, err := c.compileAttributeDecl(ctx, n, name)
	if err != nil {
		return attributeUse{}, err
	}
	return attributeUseFromDecl(decl), nil
}

func (c *compiler) localAttributeUseName(ctx *schemaSetDocument, n *schemaNode) (QName, error) {
	source, err := requiredSemanticSource[*schemaAttributeSource](n)
	if err != nil {
		return QName{}, err
	}
	name := source.Global.Name.Value
	ns := ""
	form, hasForm := source.Form.Value, source.Form.Present
	qualified, err := parseAttributeFormAttr(formAttr{
		Value:            form,
		HasValue:         hasForm,
		DefaultQualified: ctx.doc.defaults.AttributeQualified,
	})
	if err != nil {
		return QName{}, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	if qualified {
		ns = ctx.effectiveTargetNS
	}
	return c.rt.internQName(ns, name)
}

// A prohibition admits source syntax and independently mapped children, but
// contributes only a name directive rather than an attribute-use component,
// even when fixed is present. See I-008 in docs/spec/INTERPRETATIONS.md.
func (c *compiler) admitAttributeProhibition(ctx *schemaSetDocument, n *schemaNode) (result QName, err error) {
	source, err := requiredSemanticSource[*schemaAttributeSource](n)
	if err != nil {
		return QName{}, err
	}
	if source.Ref.Lexical.Present {
		if err = c.spendComponentDependency(n); err != nil {
			return QName{}, err
		}
		return c.rt.internQName(effectiveReferenceNamespace(ctx, source.Ref.Name), source.Ref.Name.Local)
	}
	if err = c.spendComponentDependency(n); err != nil {
		return QName{}, err
	}
	scope, err := c.enterComponent(n)
	if err != nil {
		return QName{}, err
	}
	defer scope.leave(&err)
	name, err := c.localAttributeUseName(ctx, n)
	if err != nil {
		return QName{}, err
	}
	if err := c.admitProhibitedAttributeChildren(ctx, n); err != nil {
		return QName{}, err
	}
	return name, nil
}

func (c *compiler) admitProhibitedAttributeChildren(ctx *schemaSetDocument, n *schemaNode) error {
	simple := n.firstXS(vocab.XSDElemSimpleType)
	// The prohibited source item maps to nothing, but its inline type maps
	// independently and retains its own component checks and limits.
	if simple == nil {
		return nil
	}
	return c.compileUnheldSimpleType(ctx, simple)
}

func attributeUseFromDecl(decl attributeDecl) attributeUse {
	return attributeUse{
		Name:            decl.Name,
		Type:            decl.Type,
		ValueConstraint: decl.ValueConstraint,
	}
}

func (c *compiler) compileAttributeGroupUse(ctx *schemaSetDocument, n *schemaNode) ([]attributeUse, WildcardID, error) {
	id, err := c.compileAttributeGroupUseID(ctx, n)
	if err != nil {
		return nil, NoWildcard, withSchemaCompileLocation(n, err)
	}
	set := c.rt.AttributeUseSets[id]
	// Every traversed member costs work, including shared members already in
	// the destination. This charges merge traversal; retention is charged in
	// addAttributeUseSet. The caller borrows these immutable records by value.
	if err := c.dependencyWork.spend(len(set.Uses)); err != nil {
		return nil, NoWildcard, withSchemaCompileLocation(n, err)
	}
	return set.Uses, set.Wildcard, nil
}

func (c *compiler) compileAttributeGroupUseID(ctx *schemaSetDocument, n *schemaNode) (attributeUseSetID, error) {
	source, err := requiredSemanticSource[*schemaAttributeGroupSource](n)
	if err != nil {
		return noAttributeUseSet, err
	}
	q, err := c.referenceQName(ctx, n, source.Ref.Name)
	if err != nil {
		return noAttributeUseSet, err
	}
	return c.compileAttributeGroupByQNameID(q)
}

func (c *compiler) compileAttributeGroupByQName(q QName) ([]attributeUse, WildcardID, error) {
	id, err := c.compileAttributeGroupByQNameID(q)
	if err != nil {
		return nil, NoWildcard, err
	}
	uses, wildcard := c.rt.attributeUsesAndWildcardForCompilation(id)
	return uses, wildcard, nil
}

func (c *compiler) compileAttributeGroupByQNameID(q QName) (result attributeUseSetID, err error) {
	label := c.rt.formatName(q)
	raw, exists := c.attributeGroupComponents[q]
	if c.compilingAttrGrp[q] {
		err = schemaComponentRecursionError(schemaComponentAttributeGroup, label)
		return noAttributeUseSet, withSchemaCompileLocation(raw.sourceNode(), err)
	}
	if err = c.spendComponentDependency(raw.sourceNode()); err != nil {
		return noAttributeUseSet, err
	}
	if id, ok := c.attrGroupDone[q]; ok {
		return id, nil
	}
	if !exists {
		return noAttributeUseSet, schemaComponentMissingError(schemaComponentAttributeGroup, label)
	}
	scope, err := c.enterComponent(raw.sourceNode())
	if err != nil {
		return noAttributeUseSet, err
	}
	defer scope.leave(&err)
	c.compilingAttrGrp[q] = true
	defer delete(c.compilingAttrGrp, q)
	id, err := c.compileAttributeUses(raw.ctx, raw.sourceNode(), nil, NoWildcard, attributeMergeDirect)
	if err != nil {
		return noAttributeUseSet, err
	}
	c.attrGroupDone[q] = id
	return id, nil
}
