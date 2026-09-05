package compile

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

const maxComponentDependencyDepth = 1024

// Compile compiles internal schema sources into a published validation runtime.
func Compile(opts Options, sources []source.Source) (*runtime.Schema, error) {
	return CompileMappedSources(opts, sources, func(src source.Source) source.Source { return src })
}

// CompileMappedSources compiles a caller-owned source slice without converting
// it until the normalized explicit-source bound has been enforced.
func CompileMappedSources[T any](opts Options, sources []T, sourceOf func(T) source.Source) (*runtime.Schema, error) {
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
	c, err := newCompiler(limits)
	if err != nil {
		return nil, err
	}
	owned, err := mapSchemaSources(sources, sourceOf)
	if err != nil {
		return nil, err
	}
	return c.compileMappedSources(owned)
}

func mapSchemaSources[T any](sources []T, sourceOf func(T) source.Source) ([]source.Source, error) {
	owned := make([]source.Source, len(sources))
	for i, input := range sources {
		owned[i] = sourceOf(input)
		if owned[i].Name() == "" {
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source name is required")
		}
	}
	return owned, nil
}

func (c *compiler) compileMappedSources(owned []source.Source) (*runtime.Schema, error) {
	var err error
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

type compiler struct {
	compilerBuildState
	compilerCycleState
	compilerIndexState
	compilerModelState

	simpleValues    runtime.SimpleValueCallbacks
	builtinFacets   runtime.BuiltinSimpleFacetStorage
	contentAnalysis *runtime.ContentModelAnalysis
	plan            schemaPlan
	rt              compilerSchemaBuild
	contentWork     workBudget
	dependencyWork  workBudget
	componentDepth  int
	limits          Limits
	missingSimple   runtime.SimpleTypeID
}

func (c *compiler) spendComponentDependency(n *rawNode) error {
	if err := c.dependencyWork.spend(1); err != nil {
		if n != nil {
			return withSchemaCompileLocation(n, err)
		}
		return err
	}
	return nil
}

func (c *compiler) enterComponent(n *rawNode) (func(), error) {
	if c.componentDepth >= maxComponentDependencyDepth {
		err := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema component dependency depth limit exceeded")
		if n != nil {
			err = withSchemaCompileLocation(n, err)
		}
		return nil, err
	}
	c.componentDepth++
	return func() {
		c.componentDepth--
	}, nil
}

func newCompiler(limits Limits) (*compiler, error) {
	names, err := NewNameTable(limits.MaxSchemaNames)
	if err != nil {
		return nil, err
	}
	builtinSimpleTypeCount := runtime.BuiltinSimpleTypeCount()
	builtinAttributeCount := runtime.BuiltinAttributeCount()
	builtinComplexTypeCount := runtime.BuiltinComplexTypeCount()
	rt := newCompilerSchemaBuild(names)
	c := &compiler{
		builtinFacets:    runtime.NewBuiltinSimpleFacetStorage(),
		simpleRaw:        make(map[runtime.QName]rawComponent),
		complexRaw:       make(map[runtime.QName]rawComponent),
		elementRaw:       make(map[runtime.QName]rawComponent),
		attributeRaw:     make(map[runtime.QName]rawComponent),
		groupRaw:         make(map[runtime.QName]rawComponent),
		attrGroupRaw:     make(map[runtime.QName]rawComponent),
		simpleDone:       make(map[runtime.QName]runtime.SimpleTypeID, builtinSimpleTypeCount),
		complexDone:      make(map[runtime.QName]runtime.ComplexTypeID, builtinComplexTypeCount),
		attributeDone:    make(map[runtime.QName]runtime.AttributeID, builtinAttributeCount),
		attrGroupDone:    make(map[runtime.QName]runtime.AttributeUseSetID),
		elementDone:      make(map[runtime.QName]runtime.ElementID),
		localDone:        make(map[*rawNode]runtime.ElementID),
		identityDeclared: make(map[*rawNode]runtime.IdentityConstraintID),
		compilingSimple:  make(map[runtime.QName]bool),
		compilingComplex: make(map[runtime.QName]bool),
		compilingAttrGrp: make(map[runtime.QName]bool),
		compilingModel:   make(map[*rawNode]bool),
		modelDone:        make(map[*rawNode]runtime.ContentModelID),
		modelDepth:       make(map[*rawNode]int),
		rt:               rt,
		missingSimple:    runtime.NoSimpleType,
		limits:           limits,
		contentWork:      newWorkBudget(contentModelWorkBudget, limits.MaxContentModelAnalysisSteps),
		dependencyWork:   newWorkBudget(dependencyWorkBudget, limits.MaxSchemaDependencySteps),
	}
	if err = c.addBuiltins(); err != nil {
		return nil, err
	}
	c.contentAnalysis, err = runtime.NewContentModelAnalysis(&c.rt, c.contentWork.spend)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *compiler) compileGlobals() error {
	if err := c.compileGlobalTypes(); err != nil {
		return err
	}
	if err := c.compileGlobalAttributeAndModelGroups(); err != nil {
		return err
	}
	if err := c.compileGlobalElements(); err != nil {
		return err
	}
	return c.finalizeCompiledGlobals()
}

func (c *compiler) compileGlobalTypes() error {
	for _, q := range sortedBuildQNames(&c.rt, c.simpleRaw) {
		if _, err := c.compileSimpleByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.complexRaw) {
		if _, err := c.compileComplexByQName(q); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileGlobalAttributeAndModelGroups() error {
	for _, q := range sortedBuildQNames(&c.rt, c.attributeRaw) {
		if _, err := c.compileAttributeByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.attrGroupRaw) {
		if _, _, err := c.compileAttributeGroupByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.groupRaw) {
		if err := c.compileModelGroupByQName(q); err != nil {
			return err
		}
	}
	return c.declareAllIdentityConstraints()
}

func (c *compiler) compileGlobalElements() error {
	for _, q := range sortedBuildQNames(&c.rt, c.elementRaw) {
		if _, err := c.compileElementByQName(q); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) finalizeCompiledGlobals() error {
	if err := c.drainDeferredAnonymousComplex(); err != nil {
		return err
	}
	if err := c.compileSubstitutions(); err != nil {
		return err
	}
	if err := c.validateCompiledComplexRestrictions(); err != nil {
		return err
	}
	if err := c.checkCompiledElementDeclarationsConsistent(); err != nil {
		return err
	}
	if err := c.validateIdentityReferences(); err != nil {
		return err
	}
	if err := c.checkCompiledModelsUPA(); err != nil {
		return err
	}
	return c.compileContentModels()
}

func (c *compiler) compileModelGroupByQName(q runtime.QName) error {
	label := c.rt.formatName(q)
	raw, ok := c.groupRaw[q]
	if !ok {
		return SchemaComponentMissingError(SchemaComponentModelGroup, label)
	}
	modelNode, err := checkTopLevelGroupChildren(raw.node)
	if err != nil {
		return err
	}
	_, err = c.compileModel(modelNode, raw.ctx)
	return err
}

func (c *compiler) validateCompiledComplexRestrictions() error {
	if err := c.validateComplexRestrictionModels(); err != nil {
		return err
	}
	updates, err := c.restrictionChoiceLimitUpdates()
	if err != nil {
		return contentRestrictionCompileError(err)
	}
	return c.applyRestrictionChoiceLimitUpdates(updates)
}

func (c *compiler) validateComplexRestrictionModels() error {
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
		if err := runtime.ValidateContentRestriction(
			&c.rt,
			base.Content,
			ct.Content,
			c.contentWork.spend,
			c.contentAnalysis,
		); err != nil {
			return contentRestrictionCompileError(err)
		}
	}
	return nil
}

func (c *compiler) applyRestrictionChoiceLimitUpdates(updates []runtime.RestrictionChoiceLimitUpdate) error {
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

func contentRestrictionCompileError(err error) error {
	var diagnostic *xsderrors.Error
	switch {
	case err == nil:
		return nil
	case runtime.IsContentRestrictionMismatch(err):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, err.Error())
	case errors.As(err, &diagnostic):
		return err
	default:
		return xsderrors.InternalInvariant(err.Error())
	}
}

func complexBlockMaskWithDefault(n *rawNode, def runtime.DerivationMask) (runtime.DerivationMask, error) {
	return derivationMaskWithDefaultChecked(n, def, complexTypeBlockDerivation())
}

func simpleFinalMaskWithDefaultChecked(n *rawNode, def runtime.DerivationMask) (runtime.DerivationMask, error) {
	return derivationMaskWithDefaultChecked(n, def, simpleTypeFinalDerivation())
}

func derivationMaskWithDefaultChecked(n *rawNode, def runtime.DerivationMask, rule DerivationAttrRule) (runtime.DerivationMask, error) {
	mask, err := ParseDerivationAttrWithDefault(rawLexicalAttribute(n, rule.Name), def, rule)
	return mask, withSchemaCompileLocation(n, err)
}

func (c *compiler) resolveQNameChecked(n *rawNode, ctx *schemaContext, lexical string) (runtime.QName, error) {
	name, err := n.resolveQName(lexical)
	if err != nil {
		return runtime.QName{}, err
	}
	namespace, err := checkReferenceNamespace(n, ctx, name.Space)
	if err != nil {
		return runtime.QName{}, err
	}
	return c.rt.internQName(namespace, name.Local)
}

func (c *compiler) validateAttributeDeclValueConstraintIdentity(decl *runtime.AttributeDecl) error {
	if err := runtime.ValidateAttributeDeclValueConstraintRuntime(&c.rt, decl.Type, runtime.DeclarationValueConstraintOf(decl.Default, decl.Fixed)); err != nil {
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

func validateElementDeclValueConstraintAtNode(n *rawNode, constraint runtime.DeclarationValueConstraint) error {
	if err := ValidateElementDeclValueConstraintAdmission(constraint); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func validateAttributeDeclValueConstraintAtNode(n *rawNode, constraint runtime.DeclarationValueConstraint) error {
	if err := ValidateAttributeDeclValueConstraintAdmission(constraint); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func parseAttributeUseModeChecked(n *rawNode, modeSource LexicalAttribute) (AttributeUseMode, error) {
	if !modeSource.Present {
		return AttributeUseOptional, nil
	}
	parsed, err := ParseAttributeUseMode(modeSource.Value)
	if err != nil {
		return AttributeUseOptional, withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return parsed, nil
}

func validateAttributeUseValueConstraintAtNode(n *rawNode, mode AttributeUseMode, hasDefault, hasFixed, refHasFixed bool) error {
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

func applyAttributeUseModeAtNode(n *rawNode, mode AttributeUseMode, hasFixed bool) (AttributeUseModeState, error) {
	state, err := ApplyAttributeUseMode(AttributeUseModeApplication{
		Mode:     mode,
		HasFixed: hasFixed,
	})
	if err != nil {
		return AttributeUseModeState{}, withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return state, nil
}

func validateAttributeUseFixedValueAtNode(
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
	if err := c.rejectSimpleTypeCycle(q, label); err != nil {
		return runtime.NoSimpleType, err
	}
	raw, exists := c.simpleRaw[q]
	var sourceNode *rawNode
	if exists {
		sourceNode = raw.node
	}
	if err := c.spendComponentDependency(sourceNode); err != nil {
		return runtime.NoSimpleType, err
	}
	if id, ok := c.simpleDone[q]; ok {
		return id, nil
	}
	if !exists {
		return runtime.NoSimpleType, SchemaComponentMissingError(SchemaComponentSimpleType, label)
	}
	leave, err := c.enterComponent(raw.node)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	defer leave()
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
	if err := c.completeGlobalSimpleType(raw, q, id, &st); err != nil {
		return runtime.NoSimpleType, err
	}
	return id, nil
}

func (c *compiler) rejectSimpleTypeCycle(q runtime.QName, label string) error {
	if !c.compilingSimple[q] {
		return nil
	}
	err := SchemaComponentCycleError(SchemaComponentSimpleType, label)
	if raw, ok := c.simpleRaw[q]; ok {
		return withSchemaCompileLocation(raw.node, err)
	}
	return err
}

func (c *compiler) completeGlobalSimpleType(raw rawComponent, q runtime.QName, id runtime.SimpleTypeID, st *runtime.SimpleType) error {
	st.Name = q
	final, err := simpleFinalMaskWithDefaultChecked(raw.node, raw.ctx.finalDefault)
	if err != nil {
		return err
	}
	st.Final = final
	st.Identity = c.rt.DerivedSimpleIdentity(*st)
	st.Fast = runtime.DeriveSimpleFastPathForSimpleType(*st)
	c.completeSimpleType(id, *st)
	return nil
}

func (c *compiler) compileAnonymousSimple(n *rawNode, ctx *schemaContext) (runtime.SimpleTypeID, error) {
	if err := c.spendComponentDependency(n); err != nil {
		return runtime.NoSimpleType, err
	}
	leave, err := c.enterComponent(n)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	defer leave()
	if err = checkLocalSimpleTypeAttributes(n); err != nil {
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
		if child.Name.Space != vocab.XSDNamespaceURI || child.Name.Local == vocab.XSDElemAnnotation {
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
	baseID, err := c.compileRestrictionBase(n, ctx)
	if err != nil {
		return runtime.SimpleType{}, err
	}
	if err := CheckSimpleRestrictionBase(baseID, c.rt.builtinIDs().AnySimpleType); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinalMask(baseID), runtime.DerivationRestriction, SimpleTypeFinalBaseRestriction); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	st := c.rt.derivedSimpleType(baseID, name)
	if err := c.compileRestrictionFacets(n, &st, baseID); err != nil {
		return runtime.SimpleType{}, err
	}
	if st.Variety == runtime.SimpleVarietyUnion {
		if err := c.chargeSimpleUnionMemberEntries(n, len(st.Union)); err != nil {
			return runtime.SimpleType{}, err
		}
	}
	return st, nil
}

func (c *compiler) compileRestrictionBase(n *rawNode, ctx *schemaContext) (runtime.SimpleTypeID, error) {
	children := n.xsSimpleTypeChildren()
	if len(children) > 1 {
		return runtime.NoSimpleType, xsderrors.InternalInvariant("restriction child validator admitted multiple simpleType children")
	}
	base := rawLexicalAttribute(n, vocab.XSDAttrBase)
	if err := ValidateSimpleRestrictionTypeSource(SimpleRestrictionTypeSource{Base: base, HasSimpleTypeChild: len(children) != 0}); err != nil {
		return runtime.NoSimpleType, withSchemaCompileLocation(n, err)
	}
	if base.Present {
		q, err := c.resolveQNameChecked(n, ctx, base.Value)
		if err != nil {
			return runtime.NoSimpleType, err
		}
		id, err := c.compileSimpleByQName(q)
		return id, withSchemaCompileLocation(n, err)
	}
	if len(children) == 1 {
		return c.compileAnonymousSimple(children[0], ctx)
	}
	return runtime.NoSimpleType, nil
}

func (c *compiler) compileRestrictionFacets(n *rawNode, st *runtime.SimpleType, base runtime.SimpleTypeID) error {
	if c.simpleTypeUnavailable[base] {
		return withSchemaCompileLocation(n, c.validateUnavailableFacetChildren(n.Children, st, base, facetChildModeDerivation))
	}
	return withSchemaCompileLocation(n, c.compileFacets(n, st, base, base))
}

func (c *compiler) compileList(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.SimpleType, error) {
	if err := checkChildOrderRules(n, simpleListChildOrder); err != nil {
		return runtime.SimpleType{}, err
	}
	item := runtime.NoSimpleType
	simpleTypeChildren := n.xsSimpleTypeChildren()
	itemType := rawLexicalAttribute(n, vocab.XSDAttrItemType)
	if err := ValidateSimpleListItemTypeSource(SimpleListItemTypeSource{ItemType: itemType, HasSimpleTypeChild: len(simpleTypeChildren) != 0}); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	switch {
	case itemType.Present:
		id, err := c.compileListItemType(n, ctx, itemType.Value)
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
	if err := CheckSimpleTypeFinalAllows(c.rt.simpleTypeFinalMask(item), runtime.DerivationList, SimpleTypeFinalListItem); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	if c.simpleListItemReachesList(item) {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, simpleListItemListReachError())
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
		missingErr := SchemaComponentMissingError(SchemaComponentSimpleType, c.rt.formatName(q))
		return runtime.NoSimpleType, withSchemaCompileLocation(n, missingErr)
	}
	if !c.typeQNameMayBeUnavailable(q) {
		missingErr := SchemaComponentMissingError(SchemaComponentSimpleType, c.rt.formatName(q))
		return runtime.NoSimpleType, withSchemaCompileLocation(n, missingErr)
	}
	return c.missingSimpleType()
}

func (c *compiler) compileUnion(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.SimpleType, error) {
	if err := checkChildOrderRules(n, simpleUnionChildOrder); err != nil {
		return runtime.SimpleType{}, err
	}
	compilation := simpleUnionCompilation{
		compiler: c,
		ctx:      ctx,
		typeDef:  runtime.SimpleType{Name: name, Variety: runtime.SimpleVarietyUnion, Primitive: runtime.PrimitiveString, Base: c.rt.builtinIDs().AnySimpleType, ListItem: runtime.NoSimpleType, Whitespace: runtime.WhitespaceCollapse},
		seen:     make(map[runtime.SimpleTypeID]struct{}),
	}
	simpleTypeChildren := n.xsSimpleTypeChildren()
	memberSource := UnionMemberTypeSource{
		MemberTypes:        rawLexicalAttribute(n, vocab.XSDAttrMemberTypes),
		HasSimpleTypeChild: len(simpleTypeChildren) != 0,
	}
	memberTypes, err := parseUnionMemberTypesAtNode(n, memberSource)
	if err != nil {
		return runtime.SimpleType{}, err
	}
	for _, part := range memberTypes {
		if err := compilation.addNamed(n, part); err != nil {
			return runtime.SimpleType{}, err
		}
	}
	for _, child := range simpleTypeChildren {
		if err := compilation.addAnonymous(child); err != nil {
			return runtime.SimpleType{}, err
		}
	}
	if len(compilation.typeDef.Union) == 0 {
		return runtime.SimpleType{}, xsderrors.InternalInvariant("union source validator admitted missing member types")
	}
	return compilation.typeDef, nil
}

type simpleUnionCompilation struct {
	compiler *compiler
	ctx      *schemaContext
	seen     map[runtime.SimpleTypeID]struct{}
	typeDef  runtime.SimpleType
}

func (c *simpleUnionCompilation) addNamed(node *rawNode, lexical string) error {
	q, err := c.compiler.resolveQNameChecked(node, c.ctx, lexical)
	if err != nil {
		return err
	}
	id, err := c.compiler.compileSimpleTypeReference(node, q)
	if err != nil {
		return err
	}
	return c.add(node, id)
}

func (c *simpleUnionCompilation) addAnonymous(node *rawNode) error {
	id, err := c.compiler.compileAnonymousSimple(node, c.ctx)
	if err != nil {
		return err
	}
	return c.add(node, id)
}

func (c *simpleUnionCompilation) add(node *rawNode, id runtime.SimpleTypeID) error {
	if err := CheckSimpleTypeFinalAllows(c.compiler.rt.simpleTypeFinalMask(id), runtime.DerivationUnion, SimpleTypeFinalUnionMember); err != nil {
		return withSchemaCompileLocation(node, err)
	}
	c.typeDef.UnionSources = append(c.typeDef.UnionSources, id)
	remaining := c.compiler.limits.MaxSimpleUnionMemberEntries - c.compiler.unionMemberEntries
	added, ok := c.compiler.rt.appendFlattenedUnionMember(&c.typeDef.Union, id, c.seen, remaining)
	c.compiler.unionMemberEntries += added
	if !ok {
		return withSchemaCompileLocation(node, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple union members exceed MaxSimpleUnionMemberEntries"))
	}
	return nil
}

func (c *compiler) chargeSimpleUnionMemberEntries(node *rawNode, count int) error {
	if count > c.limits.MaxSimpleUnionMemberEntries-c.unionMemberEntries {
		return withSchemaCompileLocation(node, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple union members exceed MaxSimpleUnionMemberEntries"))
	}
	c.unionMemberEntries += count
	return nil
}
