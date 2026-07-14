package compile

import (
	"context"
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// Compile compiles internal schema sources into a published validation runtime.
func Compile(ctx context.Context, opts Options, sources []source.Source) (*runtime.Schema, error) {
	return CompileMappedSources(ctx, opts, sources, func(src source.Source) source.Source { return src })
}

// CompileMappedSources compiles a caller-owned source slice without converting
// it until the normalized explicit-source bound has been enforced.
func CompileMappedSources[T any](ctx context.Context, opts Options, sources []T, sourceOf func(T) source.Source) (*runtime.Schema, error) {
	if err := compileContextError(ctx); err != nil {
		return nil, err
	}
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaNoSources, "at least one schema source is required")
	}
	if len(sources) > limits.MaxSchemaSources {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema source count exceeds MaxSchemaSources")
	}
	if sourceOf == nil {
		return nil, xsderrors.InternalInvariant("schema source mapper is nil")
	}
	c, err := newCompiler(ctx, limits)
	if err != nil {
		return nil, err
	}
	owned := make([]source.Source, len(sources))
	for i, input := range sources {
		if contextErr := compileContextError(ctx); contextErr != nil {
			return nil, contextErr
		}
		owned[i] = sourceOf(input)
		if contextErr := compileContextError(ctx); contextErr != nil {
			return nil, contextErr
		}
		if owned[i].Name() == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source name is required")
		}
	}
	if err = c.loadOwned(owned); err != nil {
		return nil, err
	}
	if err = c.index(); err != nil {
		return nil, err
	}
	if err = c.compileGlobals(); err != nil {
		return nil, err
	}
	rt, err := c.publishSchema()
	if err != nil {
		return nil, err
	}
	return rt, nil
}

type schemaContext struct {
	doc              *rawDoc
	imports          map[string]bool
	targetNS         string
	adoptedTarget    bool
	elementQualified bool
	attrQualified    bool
	blockDefault     runtime.DerivationMask
	finalDefault     runtime.DerivationMask
}

type rawComponent struct {
	node *rawNode
	ctx  *schemaContext
}

type compilerIndexState struct {
	simpleRaw    map[runtime.QName]rawComponent
	complexRaw   map[runtime.QName]rawComponent
	elementRaw   map[runtime.QName]rawComponent
	attributeRaw map[runtime.QName]rawComponent
	groupRaw     map[runtime.QName]rawComponent
	attrGroupRaw map[runtime.QName]rawComponent
	contexts     map[*rawDoc]*schemaContext
}

type compilerBuildState struct {
	simpleDone                map[runtime.QName]runtime.SimpleTypeID
	complexDone               map[runtime.QName]runtime.ComplexTypeID
	attributeDone             map[runtime.QName]runtime.AttributeID
	attrGroupDone             map[runtime.QName]runtime.AttributeUseSetID
	elementDone               map[runtime.QName]runtime.ElementID
	localDone                 map[*rawNode]runtime.ElementID
	identityDeclared          map[*rawNode]runtime.IdentityConstraintID
	regexCategories           RegexCategoryCache
	simpleListReach           simpleTypeListReachability
	simpleFacetCache          simpleValueFacetCache
	simpleTypeUnavailable     []bool
	deferredAnonymousComplex  []deferredAnonymousComplex
	pendingElementConstraints []pendingElementConstraint
	unionMemberEntries        int
}

type deferredAnonymousComplex struct {
	node *rawNode
	ctx  *schemaContext
	name runtime.QName
	id   runtime.ComplexTypeID
}

type compilerCycleState struct {
	compilingSimple  map[runtime.QName]bool
	compilingComplex map[runtime.QName]bool
	compilingAttrGrp map[runtime.QName]bool
	compilingModel   map[*rawNode]bool
}

type compilerModelState struct {
	modelDone    map[*rawNode]runtime.ContentModelID
	modelDepth   map[*rawNode]int
	modelSources []*rawNode
	elementDepth int
}

//nolint:govet // Field order groups the compiler's lifecycle-owned state.
type compiler struct {
	compilerBuildState
	compilerCycleState
	simpleValues  runtime.SimpleValueCallbacks
	builtinFacets runtime.BuiltinSimpleFacetStorage
	compilerIndexState
	compilerModelState
	schemas       schemaSet
	rt            compilerSchemaBuild
	limits        Limits
	ctx           context.Context
	missingSimple runtime.SimpleTypeID
}

func newCompiler(ctx context.Context, limits Limits) (*compiler, error) {
	names, err := NewNameTable(limits.MaxSchemaNames)
	if err != nil {
		return nil, err
	}
	builtinSimpleTypeCount := runtime.BuiltinSimpleTypeCount()
	builtinAttributeCount := runtime.BuiltinAttributeCount()
	builtinComplexTypeCount := runtime.BuiltinComplexTypeCount()
	rt := newCompilerSchemaBuild(names)
	c := &compiler{
		ctx:           ctx,
		builtinFacets: runtime.NewBuiltinSimpleFacetStorage(),
		compilerIndexState: compilerIndexState{
			simpleRaw:    make(map[runtime.QName]rawComponent),
			complexRaw:   make(map[runtime.QName]rawComponent),
			elementRaw:   make(map[runtime.QName]rawComponent),
			attributeRaw: make(map[runtime.QName]rawComponent),
			groupRaw:     make(map[runtime.QName]rawComponent),
			attrGroupRaw: make(map[runtime.QName]rawComponent),
			contexts:     make(map[*rawDoc]*schemaContext),
		},
		compilerBuildState: compilerBuildState{
			simpleDone:       make(map[runtime.QName]runtime.SimpleTypeID, builtinSimpleTypeCount),
			complexDone:      make(map[runtime.QName]runtime.ComplexTypeID, builtinComplexTypeCount),
			attributeDone:    make(map[runtime.QName]runtime.AttributeID, builtinAttributeCount),
			attrGroupDone:    make(map[runtime.QName]runtime.AttributeUseSetID),
			elementDone:      make(map[runtime.QName]runtime.ElementID),
			localDone:        make(map[*rawNode]runtime.ElementID),
			identityDeclared: make(map[*rawNode]runtime.IdentityConstraintID),
		},
		compilerCycleState: compilerCycleState{
			compilingSimple:  make(map[runtime.QName]bool),
			compilingComplex: make(map[runtime.QName]bool),
			compilingAttrGrp: make(map[runtime.QName]bool),
			compilingModel:   make(map[*rawNode]bool),
		},
		compilerModelState: compilerModelState{
			modelDone:  make(map[*rawNode]runtime.ContentModelID),
			modelDepth: make(map[*rawNode]int),
		},
		rt:            rt,
		missingSimple: runtime.NoSimpleType,
		limits:        limits,
	}
	if err := c.addBuiltins(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *compiler) compileGlobals() error {
	for _, q := range sortedBuildQNames(&c.rt, c.simpleRaw) {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if _, err := c.compileSimpleByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.complexRaw) {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if _, err := c.compileComplexByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.attributeRaw) {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if _, err := c.compileAttributeByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.attrGroupRaw) {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if _, _, err := c.compileAttributeGroupByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.groupRaw) {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if err := c.compileModelGroupByQName(q); err != nil {
			return err
		}
	}
	if err := c.declareAllIdentityConstraints(); err != nil {
		return err
	}
	for _, q := range sortedBuildQNames(&c.rt, c.elementRaw) {
		if err := compileContextError(c.ctx); err != nil {
			return err
		}
		if _, err := c.compileElementByQName(q); err != nil {
			return err
		}
	}
	if err := c.drainDeferredAnonymousComplex(); err != nil {
		return err
	}
	if err := compileContextError(c.ctx); err != nil {
		return err
	}
	if err := c.compileSubstitutions(); err != nil {
		return err
	}
	if err := compileContextError(c.ctx); err != nil {
		return err
	}
	if err := c.validateCompiledComplexRestrictions(); err != nil {
		return err
	}
	if err := c.checkCompiledElementDeclarationsConsistent(); err != nil {
		return err
	}
	if err := compileContextError(c.ctx); err != nil {
		return err
	}
	if err := c.validateIdentityReferences(); err != nil {
		return err
	}
	if err := c.checkCompiledModelsUPA(); err != nil {
		return err
	}
	if err := compileContextError(c.ctx); err != nil {
		return err
	}
	return c.compileContentModels()
}

func (c *compiler) compileModelGroupByQName(q runtime.QName) error {
	label := c.rt.formatName(q)
	raw, ok := c.groupRaw[q]
	if err := CheckSchemaComponentExists(SchemaComponentModelGroup, ok, label); err != nil {
		return err
	}
	modelNode, err := checkTopLevelGroupChildren(raw.node)
	if err != nil {
		return err
	}
	_, err = c.compileModel(modelNode, raw.ctx)
	return err
}

func (c *compiler) validateCompiledComplexRestrictions() error {
	for id := range c.rt.ComplexTypeCount() {
		ct := c.rt.complexType(runtime.ComplexTypeID(id))
		if id == int(c.rt.builtinIDs().AnyType) || ct.Derivation != runtime.DerivationKindRestriction {
			continue
		}
		baseID, ok := ct.Base.Complex()
		if !ok || baseID == c.rt.builtinIDs().AnyType {
			continue
		}
		base := c.rt.complexType(baseID)
		if err := runtime.ValidateContentRestriction(&c.rt, base.Content, ct.Content); err != nil {
			return err
		}
	}
	updates, err := c.restrictionChoiceLimitUpdates()
	if err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	for _, update := range updates {
		id, err := c.addModel(update.Model)
		if err != nil {
			return err
		}
		ct := c.rt.complexType(update.ComplexType)
		ct.Content = id
		c.completeComplexType(update.ComplexType, ct)
	}
	return nil
}

func complexBlockMaskWithDefault(n *rawNode, def runtime.DerivationMask) (runtime.DerivationMask, error) {
	return derivationMaskWithDefaultChecked(n, def, complexTypeBlockDerivation())
}

func simpleFinalMaskWithDefaultChecked(n *rawNode, def runtime.DerivationMask) (runtime.DerivationMask, error) {
	return derivationMaskWithDefaultChecked(n, def, simpleTypeFinalDerivation())
}

func derivationMaskWithDefaultChecked(n *rawNode, def runtime.DerivationMask, rule DerivationAttrRule) (runtime.DerivationMask, error) {
	v, ok := n.attr(rule.Name)
	mask, err := ParseDerivationAttrWithDefault(v, ok, def, rule)
	return mask, withSchemaCompileLocation(n, err)
}

func (c *compiler) resolveQNameChecked(n *rawNode, ctx *schemaContext, lexical string) (runtime.QName, error) {
	ns, local, err := n.resolveQName(lexical)
	if err != nil {
		return runtime.QName{}, err
	}
	ns, err = c.checkReferenceNamespace(n, ctx, ns)
	if err != nil {
		return runtime.QName{}, err
	}
	return c.rt.internQName(ns, local)
}

func (c *compiler) validateAttributeDeclValueConstraintIdentity(decl *runtime.AttributeDecl) error {
	if err := runtime.ValidateAttributeDeclValueConstraintRuntime(&c.rt, decl.Type, decl.Default != nil, decl.Fixed != nil); err != nil {
		return invalidAttributeError(err)
	}
	return nil
}

func (c *compiler) validateAttributeDeclName(n *rawNode, q runtime.QName) error {
	if err := c.validateAttributeDeclNameBuild(q); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func validateElementDeclValueConstraintAdmission(n *rawNode, hasDefault, hasFixed bool) error {
	if err := ValidateElementDeclValueConstraintAdmission(hasDefault, hasFixed); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func validateAttributeDeclValueConstraintAdmission(n *rawNode, hasDefault, hasFixed bool) error {
	if err := ValidateAttributeDeclValueConstraintAdmission(hasDefault, hasFixed); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func parseAttributeUseModeChecked(n *rawNode, mode string, ok bool) (AttributeUseMode, error) {
	if !ok {
		return AttributeUseOptional, nil
	}
	parsed, err := ParseAttributeUseMode(mode)
	if err != nil {
		return AttributeUseOptional, withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return parsed, nil
}

func validateAttributeUseValueConstraintAdmission(n *rawNode, mode AttributeUseMode, hasDefault, hasFixed, refHasFixed bool) error {
	if err := ValidateAttributeUseValueConstraintAdmission(AttributeUseValueConstraintAdmission{
		Mode:                   mode,
		HasDefault:             hasDefault,
		HasFixed:               hasFixed,
		ReferencedDeclHasFixed: refHasFixed,
	}); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func applyAttributeUseMode(n *rawNode, mode AttributeUseMode, hasFixed bool) (AttributeUseModeState, error) {
	state, err := ApplyAttributeUseMode(AttributeUseModeApplication{
		Mode:     mode,
		HasFixed: hasFixed,
	})
	if err != nil {
		return AttributeUseModeState{}, withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return state, nil
}

func validateAttributeUseFixedValueAdmission(
	n *rawNode,
	fixed, refFixed runtime.ValueConstraintIdentity,
) error {
	if err := ValidateAttributeUseFixedValueAdmission(AttributeUseFixedValueAdmission{
		Fixed:               fixed,
		ReferencedDeclFixed: refFixed,
	}); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func (c *compiler) validateAttributeUseSet(set runtime.AttributeUseSet) error {
	if err := c.validateAttributeUseSetBuild(set); err != nil {
		return invalidAttributeError(err)
	}
	return nil
}

func (c *compiler) compileSimpleByQName(q runtime.QName) (runtime.SimpleTypeID, error) {
	label := c.rt.formatName(q)
	if c.compilingSimple[q] {
		err := CheckSchemaComponentCycle(SchemaComponentSimpleType, true, label)
		if raw, ok := c.simpleRaw[q]; ok {
			return runtime.NoSimpleType, withSchemaCompileLocation(raw.node, err)
		}
		return runtime.NoSimpleType, err
	}
	if id, ok := c.simpleDone[q]; ok {
		return id, nil
	}
	raw, ok := c.simpleRaw[q]
	if err := CheckSchemaComponentExists(SchemaComponentSimpleType, ok, label); err != nil {
		return runtime.NoSimpleType, err
	}
	c.compilingSimple[q] = true
	defer delete(c.compilingSimple, q)
	id, err := c.registerGlobalSimpleType(q, runtime.SimpleType{Name: q, Variety: runtime.SimpleVarietyAtomic, Primitive: runtime.PrimitiveString, Base: c.rt.builtinIDs().AnySimpleType, ListItem: runtime.NoSimpleType, Whitespace: runtime.WhitespacePreserve})
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.simpleDone[q] = id
	st, err := c.compileSimpleType(raw.node, raw.ctx, q)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	st.Name = q
	final, err := simpleFinalMaskWithDefaultChecked(raw.node, raw.ctx.finalDefault)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	st.Final = final
	st.Identity = c.rt.DerivedSimpleIdentity(st)
	st.Fast = runtime.DeriveSimpleFastPathForSimpleType(st)
	c.completeSimpleType(id, st)
	return id, nil
}

func (c *compiler) compileAnonymousSimple(n *rawNode, ctx *schemaContext) (runtime.SimpleTypeID, error) {
	if err := checkLocalSimpleTypeAttributes(n); err != nil {
		return runtime.NoSimpleType, err
	}
	q, err := c.rt.internQName("", fmt.Sprintf("$simple%d", c.rt.SimpleTypeCount()))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	id, err := c.addSimpleType(runtime.SimpleType{Name: q, Variety: runtime.SimpleVarietyAtomic, Primitive: runtime.PrimitiveString, Base: c.rt.builtinIDs().AnySimpleType, ListItem: runtime.NoSimpleType, Whitespace: runtime.WhitespacePreserve})
	if err != nil {
		return runtime.NoSimpleType, err
	}
	st, err := c.compileSimpleType(n, ctx, q)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	st.Name = q
	final, err := simpleFinalMaskWithDefaultChecked(n, ctx.finalDefault)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	st.Final = final
	st.Identity = c.rt.DerivedSimpleIdentity(st)
	st.Fast = runtime.DeriveSimpleFastPathForSimpleType(st)
	c.completeSimpleType(id, st)
	return id, nil
}

func (c *compiler) compileSimpleType(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.SimpleType, error) {
	if err := validateSimpleTypeChildren(n); err != nil {
		return runtime.SimpleType{}, err
	}
	child := simpleTypeDerivationChild(n)
	if child == nil {
		return runtime.SimpleType{}, xsderrors.InternalInvariant("simpleType child validator admitted invalid derivation count")
	}
	switch child.Name.Local {
	case vocab.XSDElemRestriction:
		return c.compileRestriction(child, ctx, name)
	case vocab.XSDElemList:
		return c.compileList(child, ctx, name)
	case vocab.XSDElemUnion:
		return c.compileUnion(child, ctx, name)
	default:
		return runtime.SimpleType{}, xsderrors.InternalInvariant("simpleType child validator admitted " + child.Name.Local)
	}
}

func simpleTypeDerivationChild(n *rawNode) *rawNode {
	for _, child := range n.Children {
		if child.Name.Space != runtime.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
			continue
		}
		return child
	}
	return nil
}

func validateSimpleTypeChildren(n *rawNode) error {
	if err := checkChildOrderRules(n, simpleTypeChildOrder); err != nil {
		return err
	}
	for child := range n.xsdChildren() {
		switch child.Name.Local {
		case restrictionChild, listChild, unionChild:
			return nil
		}
	}
	return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "simpleType must contain one restriction, list, or union")
}

func (c *compiler) compileRestriction(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.SimpleType, error) {
	if err := checkChildOrderRules(n, simpleRestrictionChildOrder); err != nil {
		return runtime.SimpleType{}, err
	}
	var baseID runtime.SimpleTypeID
	simpleTypeChildren := n.xsSimpleTypeChildren()
	if len(simpleTypeChildren) > 1 {
		return runtime.SimpleType{}, xsderrors.InternalInvariant("restriction child validator admitted multiple simpleType children")
	}
	baseAttr, hasBase := n.attr(vocab.XSDAttrBase)
	if err := ValidateSimpleRestrictionTypeSource(hasBase, len(simpleTypeChildren) != 0); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	if hasBase {
		q, err := c.resolveQNameChecked(n, ctx, baseAttr)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		id, err := c.compileSimpleByQName(q)
		if err != nil {
			return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
		}
		baseID = id
	} else if len(simpleTypeChildren) == 1 {
		id, err := c.compileAnonymousSimple(simpleTypeChildren[0], ctx)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		baseID = id
	}
	if err := CheckSimpleRestrictionBase(baseID, c.rt.builtinIDs().AnySimpleType); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinal(baseID), runtime.DerivationRestriction, SimpleTypeFinalBaseRestriction); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	st := c.rt.derivedSimpleType(baseID, name)
	if c.simpleTypeUnavailable[baseID] {
		if err := c.validateUnavailableFacetChildren(n.Children, &st, baseID, true); err != nil {
			return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
		}
	} else if err := c.compileFacets(n, &st, baseID, baseID); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	if st.Variety == runtime.SimpleVarietyUnion {
		if err := c.chargeSimpleUnionMemberEntries(n, len(st.Union)); err != nil {
			return runtime.SimpleType{}, err
		}
	}
	return st, nil
}

func (c *compiler) compileList(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.SimpleType, error) {
	if err := checkChildOrderRules(n, simpleListChildOrder); err != nil {
		return runtime.SimpleType{}, err
	}
	item := runtime.NoSimpleType
	simpleTypeChildren := n.xsSimpleTypeChildren()
	itemType, hasItemType := n.attr(vocab.XSDAttrItemType)
	if err := ValidateSimpleListItemTypeSource(hasItemType, len(simpleTypeChildren) != 0); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	switch {
	case hasItemType:
		id, err := c.compileListItemType(n, ctx, itemType)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		item = id
	case len(simpleTypeChildren) == 1:
		id, err := c.compileAnonymousSimple(simpleTypeChildren[0], ctx)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		item = id
	case len(simpleTypeChildren) > 1:
		return runtime.SimpleType{}, xsderrors.InternalInvariant("list child validator admitted multiple simpleType children")
	}
	if item == runtime.NoSimpleType {
		return runtime.SimpleType{}, xsderrors.InternalInvariant("list source validator admitted missing item type")
	}
	if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinal(item), runtime.DerivationList, SimpleTypeFinalListItem); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	if err := checkSimpleListItemType(c.simpleListItemReachesList(item)); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	return runtime.SimpleType{Name: name, Variety: runtime.SimpleVarietyList, Primitive: runtime.PrimitiveString, Base: c.rt.builtinIDs().AnySimpleType, Whitespace: runtime.WhitespaceCollapse, ListItem: item}, nil
}

func (c *compiler) compileListItemType(n *rawNode, ctx *schemaContext, itemType string) (runtime.SimpleTypeID, error) {
	q, err := c.resolveQNameChecked(n, ctx, itemType)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	return c.compileSimpleTypeReference(n, q)
}

func (c *compiler) compileSimpleTypeReference(n *rawNode, q runtime.QName) (runtime.SimpleTypeID, error) {
	if c.simpleTypeQNameKnown(q) {
		id, compileErr := c.compileSimpleByQName(q)
		return id, withSchemaCompileLocation(n, compileErr)
	}
	if c.typeQNameKnown(q) {
		missingErr := CheckSchemaComponentExists(SchemaComponentSimpleType, false, c.rt.formatName(q))
		return runtime.NoSimpleType, withSchemaCompileLocation(n, missingErr)
	}
	if !c.typeQNameMayBeUnavailable(q) {
		missingErr := CheckSchemaComponentExists(SchemaComponentSimpleType, false, c.rt.formatName(q))
		return runtime.NoSimpleType, withSchemaCompileLocation(n, missingErr)
	}
	return c.missingSimpleType()
}

func (c *compiler) compileUnion(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.SimpleType, error) {
	if err := checkChildOrderRules(n, simpleUnionChildOrder); err != nil {
		return runtime.SimpleType{}, err
	}
	st := runtime.SimpleType{Name: name, Variety: runtime.SimpleVarietyUnion, Primitive: runtime.PrimitiveString, Base: c.rt.builtinIDs().AnySimpleType, ListItem: runtime.NoSimpleType, Whitespace: runtime.WhitespaceCollapse}
	simpleTypeChildren := n.xsSimpleTypeChildren()
	mt, hasMemberTypes := n.attr(vocab.XSDAttrMemberTypes)
	memberTypes, err := parseUnionMemberTypes(n, mt, hasMemberTypes, len(simpleTypeChildren) != 0)
	if err != nil {
		return runtime.SimpleType{}, err
	}
	seen := make(map[runtime.SimpleTypeID]struct{})
	appendMember := func(node *rawNode, id runtime.SimpleTypeID) error {
		st.UnionSources = append(st.UnionSources, id)
		remaining := c.limits.MaxSimpleUnionMemberEntries - c.unionMemberEntries
		added, ok := c.rt.appendFlattenedUnionMember(&st.Union, id, seen, remaining)
		c.unionMemberEntries += added
		if !ok {
			return withSchemaCompileLocation(node, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple union members exceed MaxSimpleUnionMemberEntries"))
		}
		return nil
	}
	for _, part := range memberTypes {
		q, err := c.resolveQNameChecked(n, ctx, part)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		id, err := c.compileSimpleTypeReference(n, q)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinal(id), runtime.DerivationUnion, SimpleTypeFinalUnionMember); err != nil {
			return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
		}
		if err := appendMember(n, id); err != nil {
			return runtime.SimpleType{}, err
		}
	}
	for _, child := range simpleTypeChildren {
		id, err := c.compileAnonymousSimple(child, ctx)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinal(id), runtime.DerivationUnion, SimpleTypeFinalUnionMember); err != nil {
			return runtime.SimpleType{}, withSchemaCompileLocation(child, err)
		}
		if err := appendMember(child, id); err != nil {
			return runtime.SimpleType{}, err
		}
	}
	if len(st.Union) == 0 {
		return runtime.SimpleType{}, xsderrors.InternalInvariant("union source validator admitted missing member types")
	}
	return st, nil
}

func (c *compiler) chargeSimpleUnionMemberEntries(node *rawNode, count int) error {
	if count > c.limits.MaxSimpleUnionMemberEntries-c.unionMemberEntries {
		return withSchemaCompileLocation(node, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple union members exceed MaxSimpleUnionMemberEntries"))
	}
	c.unionMemberEntries += count
	return nil
}
