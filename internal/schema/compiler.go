package schema

import (
	"encoding/xml"
	"errors"
	"math"
	"slices"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// maxComponentDependencyDepth caps nested active component compilation
// (derivation bases, group and declaration references), restriction
// normalization and value dependency chains,
// independently of XML nesting depth. Dependency work bounds total effort;
// this bounds stack depth.
const maxComponentDependencyDepth = 1024

// Compile compiles internal schema sources into a published validation runtime.
func Compile(opts Options, sources []source.Source) (*Schema, error) {
	return CompileMappedSources(opts, sources, func(src source.Source) source.Source { return src })
}

// CompileMappedSources compiles a caller-owned source slice without converting
// it until the normalized explicit-source bound has been enforced.
func CompileMappedSources[T any](opts Options, sources []T, sourceOf func(T) source.Source) (*Schema, error) {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaNoSources, "at least one schema source is required", nil)
	}
	// Bounds the caller's explicit list; documents, explicit and
	// resolver-loaded, are counted when the loader registers them.
	if len(sources) > limits.MaxSchemaSources {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema source count exceeds MaxSchemaSources", nil)
	}
	if sourceOf == nil {
		return nil, xsderrors.InternalInvariant("schema source mapper is nil", nil)
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
			return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "schema source name is required", nil)
		}
	}
	return owned, nil
}

// compileMappedSources indexes only after loading has planned every effective
// target namespace, because chameleon documents are indexed once per context.
func (c *compiler) compileMappedSources(owned []source.Source) (*Schema, error) {
	if err := c.loadOwned(owned); err != nil {
		return nil, err
	}
	rt, err := c.compileLoaded()
	if err != nil {
		return nil, c.locateSetWideLimit(untranslatedBuildDefect(err))
	}
	return rt, nil
}

func (c *compiler) compileLoaded() (*Schema, error) {
	if err := c.index(); err != nil {
		return nil, err
	}
	if err := c.reserveIndexedComponentStorage(); err != nil {
		return nil, err
	}
	if err := c.compileGlobals(); err != nil {
		return nil, err
	}
	return c.publishSchema()
}

// locateSetWideLimit locates a schema.limit failure that no component scope
// located. Name indexing, substitution closure, the checks of the builtin
// anyType content model and the publication audit work on the whole schema
// set, so locateSetWide places their limit failures.
// Other diagnostics are returned unchanged.
func (c *compiler) locateSetWideLimit(err error) error {
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic == nil || diagnostic.Code() != xsderrors.CodeSchemaLimit {
		return err
	}
	return locateSetWide(c.plan.documents, err)
}

// reserveIndexedComponentStorage avoids repeated growth while the indexed
// global declarations are converted into compiler-owned records. Anonymous
// components may still extend these slices during compilation.
func (c *compiler) reserveIndexedComponentStorage() error {
	c.rt.SimpleTypes = slices.Grow(c.rt.SimpleTypes, len(c.simpleComponents))
	c.rt.simpleTypeUnavailable = slices.Grow(c.rt.simpleTypeUnavailable, len(c.simpleComponents))
	// Value capacity counts total nonbuiltin slots, including existing intrinsics.
	valueTypes := len(c.rt.SimpleTypes) - int(value.BuiltinTypeCount) + len(c.simpleComponents)
	if err := c.ensureValueBuilder().ReserveCapacity(valueTypes); err != nil {
		return schemaValueError(c.limits, xsderrors.CodeSchemaFacet, "simple type", err)
	}
	return nil
}

// Compiler bindings borrow completed plan rows through deferred compilation.
type schemaComponent struct {
	ctx *schemaSetDocument
	id  schemaNodeID
}

func (c schemaComponent) sourceNode() *schemaNode {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.doc.node(c.id)
}

type compilerIndexState struct {
	simpleComponents         map[QName]schemaComponent
	complexComponents        map[QName]schemaComponent
	elementComponents        map[QName]schemaComponent
	attributeComponents      map[QName]schemaComponent
	groupComponents          map[QName]schemaComponent
	attributeGroupComponents map[QName]schemaComponent
}

type compilerBuildState struct {
	attrGroupDone    map[QName]attributeUseSetID
	localDone        map[schemaNodeKey]ElementID
	identityDeclared map[schemaNodeKey]IdentityConstraintID
	identitySources  []*schemaNode
	// identityPathsByNode holds the interned paths of each compiled identity
	// constraint node, keyed without a document context; see identityPaths.
	identityPathsByNode map[schemaNodeKey]identityConstraintPaths
	// identityDeclarations is the expanded name of every identity constraint
	// declareAllIdentityConstraints declares, by ID, so a keyref refer
	// resolves before the referenced constraint's name is interned
	// (nameIdentityConstraint). identityNames indexes it by name; reserveIdentity
	// writes both.
	identityDeclarations      []ExpandedName
	identityNames             map[ExpandedName]IdentityConstraintID
	complexDefinitions        []complexTypeDefinition
	pendingElementConstraints []pendingElementConstraint
	unionMemberEntries        int
	attributeUseCount         uint32
}

type compilerCycleState struct {
	compilingSimple  map[QName]bool
	compilingAttrGrp map[QName]bool
	compilingModel   map[schemaNodeKey]bool
}

// modelDefiners holds, for each model, the complex type whose definition
// created it, or noComplexType for a model a named group, a zero-count
// particle or a builtin created.
type compilerModelState struct {
	modelDone             map[schemaNodeKey]compiledModelGroup
	modelSources          []*schemaNode
	modelDefiners         []ComplexTypeID
	deferredModelChildren []schemaComponent
}

type compiler struct {
	compilerBuildState
	compilerCycleState
	compilerIndexState
	compilerModelState

	contentAnalysis *contentModelAnalysis

	// patterns is lent to start-tag admission while the schema set loads;
	// see schemaPatterns.
	patterns schemaPatterns

	plan           schemaPlan
	rt             schemaBuild
	contentWork    workBudget
	dependencyWork workBudget
	componentDepth int
	limits         Limits
	missingSimple  SimpleTypeID
}

// spendComponentDependency charges one component reference. Callers spend
// before consulting their compiled-component cache, so a cache hit is still
// charged and repeated references cannot evade the dependency budget.
func (c *compiler) spendComponentDependency(n *schemaNode) error {
	if err := c.dependencyWork.spend(1); err != nil {
		if n != nil {
			return withSchemaCompileLocation(n, err)
		}
		return err
	}
	return nil
}

// componentScope is the open compile scope of one component, opened both to
// compile the component and for each check made on it after compilation:
// Element Declarations Consistent, the walk to the models complex types reach,
// Unique Particle Attribution and automaton construction at the source of the
// content model, restriction at the derived type's definition, and keyref
// references at the keyref.
// leave fills the missing path and line of a diagnostic leaving the component
// from its node, so a budget or admission failure raised anywhere inside the
// component names its document. A location attached deeper (an inner
// component or a reference node) is not overwritten, so the innermost
// enclosing component wins over the referring site. A scope with no node is
// set-wide work; locateSetWideLimit locates its limit failures.
type componentScope struct {
	compiler *compiler
	node     *schemaNode
}

// enterComponent opens the compile scope of the component at n. The caller
// defers leave with the address of its named error result in the same
// function, so the depth is restored when the component's work returns.
func (c *compiler) enterComponent(n *schemaNode) (componentScope, error) {
	if c.componentDepth >= maxComponentDependencyDepth {
		err := xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "schema component dependency depth limit exceeded", nil)
		if n != nil {
			err = withSchemaCompileLocation(n, err)
		}
		return componentScope{}, err
	}
	c.componentDepth++
	return componentScope{compiler: c, node: n}, nil
}

// leave closes the scope and locates *errp at the scope's node when *errp
// carries no location yet.
func (s componentScope) leave(errp *error) {
	s.compiler.componentDepth--
	if s.node != nil && *errp != nil {
		*errp = withSchemaCompileLocation(s.node, *errp)
	}
}

func newCompiler(limits Limits) (*compiler, error) {
	names, err := newRuntimeNameTable(limits.MaxSchemaNames)
	if err != nil {
		return nil, err
	}
	rt := newSchemaBuild(names)
	rt.valueBuilder = newSchemaValueBuilder(limits)
	c := &compiler{
		simpleComponents:         make(map[QName]schemaComponent),
		complexComponents:        make(map[QName]schemaComponent),
		elementComponents:        make(map[QName]schemaComponent),
		attributeComponents:      make(map[QName]schemaComponent),
		groupComponents:          make(map[QName]schemaComponent),
		attributeGroupComponents: make(map[QName]schemaComponent),
		attrGroupDone:            make(map[QName]attributeUseSetID),
		localDone:                make(map[schemaNodeKey]ElementID),
		identityDeclared:         make(map[schemaNodeKey]IdentityConstraintID),
		compilingSimple:          make(map[QName]bool),
		compilingAttrGrp:         make(map[QName]bool),
		compilingModel:           make(map[schemaNodeKey]bool),
		modelDone:                make(map[schemaNodeKey]compiledModelGroup),
		rt:                       rt,
		missingSimple:            NoSimpleType,
		limits:                   limits,
		contentWork:              newWorkBudget(contentModelWorkBudget, limits.MaxContentModelAnalysisSteps),
		dependencyWork:           newWorkBudget(dependencyWorkBudget, math.MaxInt),
		patterns:                 newSchemaPatterns(limits),
	}
	c.rt.typeWork = &c.dependencyWork
	// The builtin components are a fixed set, not admitted from the caller's
	// sources, so seeding them spends no MaxSchemaDependencySteps work and a
	// limit failure always has a schema document to locate it.
	if err = c.addBuiltins(); err != nil {
		return nil, err
	}
	c.dependencyWork = newWorkBudget(dependencyWorkBudget, limits.MaxSchemaDependencySteps)
	c.contentAnalysis, err = newContentModelAnalysis(&c.rt, c.contentWork.spend)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *compiler) compileGlobals() error {
	if err := c.declareAllIdentityConstraints(); err != nil {
		return err
	}
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
	for _, q := range sortedBuildQNames(&c.rt, c.simpleComponents) {
		if _, err := c.compileSimpleByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.complexComponents) {
		if _, err := c.compileComplexByQName(q, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileGlobalAttributeAndModelGroups() error {
	for _, q := range sortedBuildQNames(&c.rt, c.attributeComponents) {
		if _, err := c.compileAttributeByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.attributeGroupComponents) {
		if _, _, err := c.compileAttributeGroupByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedBuildQNames(&c.rt, c.groupComponents) {
		if err := c.compileModelGroupByQName(q); err != nil {
			return err
		}
	}
	return c.nameAllIdentityConstraints()
}

func (c *compiler) compileGlobalElements() error {
	for _, q := range sortedBuildQNames(&c.rt, c.elementComponents) {
		if _, err := c.compileElementByQName(q); err != nil {
			return err
		}
	}
	return nil
}

// Both append-only work lists can discover definitions owned by the other.
// Each admitted source/context is completed once before semantic finalization.
func (c *compiler) completeComponentDefinitions() error {
	var complexNext, modelNext int
	for complexNext < len(c.complexDefinitions) || modelNext < len(c.deferredModelChildren) {
		if complexNext < len(c.complexDefinitions) {
			if err := c.completeComplexDefinition(ComplexTypeID(complexNext)); err != nil {
				return err
			}
			complexNext++
			continue
		}
		if err := c.compileDeferredModelChildren(c.deferredModelChildren[modelNext]); err != nil {
			return err
		}
		modelNext++
	}
	c.deferredModelChildren = nil
	return nil
}

func (c *compiler) finalizeCompiledGlobals() error {
	if err := c.completeComponentDefinitions(); err != nil {
		return err
	}
	if err := c.compileSubstitutions(); err != nil {
		return err
	}
	if err := c.validateComplexRestrictionModels(); err != nil {
		return err
	}
	if err := c.checkCompiledElementDeclarationsConsistent(); err != nil {
		return err
	}
	if err := c.validateIdentityReferences(); err != nil {
		return err
	}
	reached, err := c.complexTypeReachableModels()
	if err != nil {
		return err
	}
	if err := c.detachUnreachableComponents(); err != nil {
		return err
	}
	if err := c.checkCompiledModelsUPA(reached); err != nil {
		return err
	}
	return c.compileContentModels(reached)
}

func (c *compiler) compileModelGroupByQName(q QName) error {
	label := c.rt.formatName(q)
	raw, ok := c.groupComponents[q]
	if !ok {
		return schemaComponentMissingError(schemaComponentModelGroup, label)
	}
	modelNode, err := topLevelGroupModel(raw.sourceNode())
	if err != nil {
		return err
	}
	_, err = c.compileGroupModel(raw.ctx, modelNode)
	return err
}

// validateComplexRestrictionModels checks each complex restriction against
// its base in the scope of the derived type's definition. The content models
// may be shared groups declared in other documents, so the derived type, not
// a model, names the violation.
func (c *compiler) validateComplexRestrictionModels() error {
	for id := range len(c.rt.ComplexTypes) {
		if err := c.validateComplexRestrictionModel(ComplexTypeID(id)); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) validateComplexRestrictionModel(id ComplexTypeID) (err error) {
	ct := c.rt.complexType(id)
	if ct.Derivation != derivationKindRestriction || baseIsUrType(&c.rt, ct) {
		return nil
	}
	baseID, ok := ct.Base.Complex()
	if !ok {
		return nil
	}
	n := c.complexDefinitions[id].source.sourceNode()
	if n == nil {
		return xsderrors.InternalInvariant("restricted complex type has no source", nil)
	}
	scope, err := c.enterComponent(n)
	if err != nil {
		return err
	}
	defer scope.leave(&err)
	base := c.rt.complexType(baseID)
	return contentRestrictionCompileError(validateContentRestriction(
		&c.rt,
		base.Content,
		ct.Content,
		c.contentWork.spend,
		c.contentAnalysis,
	))
}

func contentRestrictionCompileError(err error) error {
	var diagnostic *xsderrors.Error
	switch {
	case err == nil:
		return nil
	case isContentRestrictionMismatch(err):
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, err.Error(), nil)
	case errors.As(err, &diagnostic):
		return err
	default:
		return xsderrors.InternalInvariant("", err)
	}
}

func complexBlockMaskWithDefault(n *schemaNode, def DerivationMask) (DerivationMask, error) {
	source, err := requiredSemanticSource[*schemaComplexTypeSource](n)
	if err != nil {
		return 0, err
	}
	return derivationMaskWithDefaultChecked(n, source.Block, def, complexTypeBlockDerivation())
}

func simpleFinalMaskWithDefaultChecked(n *schemaNode, def DerivationMask) (DerivationMask, error) {
	typedSource, ok := n.semantic.kind.(*schemaSimpleTypeSource)
	if !ok || typedSource == nil {
		return 0, withSchemaCompileLocation(n, xsderrors.InternalInvariant("simpleType node has no typed simpleType source", nil))
	}
	return derivationMaskWithDefaultChecked(n, typedSource.Final, def, simpleTypeFinalDerivation())
}

func derivationMaskWithDefaultChecked(n *schemaNode, attr lexicalAttribute, def DerivationMask, rule derivationAttrRule) (DerivationMask, error) {
	mask, err := parseDerivationAttrWithDefault(attr, def, rule)
	return mask, withSchemaCompileLocation(n, err)
}

// referenceQName applies effective-context rules without changing the shared
// admitted name when a chameleon source serves multiple target namespaces.
func (c *compiler) referenceQName(ctx *schemaSetDocument, n *schemaNode, name xml.Name) (QName, error) {
	namespace, err := checkReferenceNamespace(ctx, n, name)
	if err != nil {
		return QName{}, err
	}
	return c.rt.internQName(namespace, name.Local)
}

func (c *compiler) validateAttributeDeclName(n *schemaNode, q QName) error {
	if err := c.validateAttributeDeclNameBuild(q); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func parseAttributeUseModeChecked(n *schemaNode, modeSource lexicalAttribute) (attributeUseMode, error) {
	if !modeSource.Present {
		return attributeUseOptional, nil
	}
	parsed, err := parseAttributeUseMode(modeSource.Value)
	if err != nil {
		return attributeUseOptional, withSchemaCompileLocation(n, admittedValueInvariant(err))
	}
	return parsed, nil
}

func validateAttributeUseValueConstraintAtNode(n *schemaNode, hasDefault, refHasFixed bool) error {
	if err := validateAttributeUseValueConstraintAdmission(attributeUseValueConstraintAdmission{
		HasDefault:             hasDefault,
		ReferencedDeclHasFixed: refHasFixed,
	}); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func validateAttributeUseFixedValueAtNode(
	n *schemaNode,
	fixed, refFixed constraintIdentity,
) error {
	if err := validateAttributeUseFixedValueAdmission(attributeUseFixedValueAdmission{
		Fixed:               fixed,
		ReferencedDeclFixed: refFixed,
	}); err != nil {
		return withSchemaCompileLocation(n, invalidAttributeError(err))
	}
	return nil
}

func (c *compiler) validateAttributeUseSet(set attributeUseSet) error {
	if err := c.validateAttributeUseSetBuild(set); err != nil {
		return invalidAttributeError(err)
	}
	return nil
}

func (c *compiler) compileSimpleByQName(q QName) (result SimpleTypeID, err error) {
	label := c.rt.formatName(q)
	if err = c.rejectSimpleTypeCycle(q, label); err != nil {
		return NoSimpleType, err
	}
	raw, exists := c.simpleComponents[q]
	var sourceNode *schemaNode
	if exists {
		sourceNode = raw.sourceNode()
	}
	if err = c.spendComponentDependency(sourceNode); err != nil {
		return NoSimpleType, err
	}
	if id, ok := c.rt.GlobalTypes[q].Simple(); ok {
		return id, nil
	}
	if !exists {
		return NoSimpleType, schemaComponentMissingError(schemaComponentSimpleType, label)
	}
	scope, err := c.enterComponent(raw.sourceNode())
	if err != nil {
		return NoSimpleType, err
	}
	defer scope.leave(&err)
	c.compilingSimple[q] = true
	defer delete(c.compilingSimple, q)
	id, err := c.registerGlobalSimpleType(q, simpleType{
		Name: q,
		ValueSpec: value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespacePreserve, WhitespacePresent: true,
			Base: value.AnySimpleTypeID, ListItem: value.NoType,
		},
	})
	if err != nil {
		return NoSimpleType, err
	}
	st, err := c.compileSimpleType(raw.ctx, raw.sourceNode(), q)
	if err != nil {
		return NoSimpleType, err
	}
	if err := c.completeGlobalSimpleType(raw, q, id, &st); err != nil {
		return NoSimpleType, err
	}
	return id, nil
}

func (c *compiler) rejectSimpleTypeCycle(q QName, label string) error {
	if !c.compilingSimple[q] {
		return nil
	}
	err := schemaComponentCycleError(schemaComponentSimpleTypeLabel + " " + label)
	if raw, ok := c.simpleComponents[q]; ok {
		return withSchemaCompileLocation(raw.sourceNode(), err)
	}
	return err
}

func (c *compiler) completeGlobalSimpleType(raw schemaComponent, q QName, id SimpleTypeID, st *simpleType) error {
	st.Name = q
	final, err := simpleFinalMaskWithDefaultChecked(raw.sourceNode(), raw.ctx.doc.defaults.FinalDefault)
	if err != nil {
		return err
	}
	st.Final = final
	if err := c.completeSimpleType(id, *st); err != nil {
		return withSchemaCompileLocation(raw.sourceNode(), err)
	}
	return nil
}

func (c *compiler) compileAnonymousSimple(ctx *schemaSetDocument, n *schemaNode) (result SimpleTypeID, err error) {
	if err = c.spendComponentDependency(n); err != nil {
		return NoSimpleType, err
	}
	scope, err := c.enterComponent(n)
	if err != nil {
		return NoSimpleType, err
	}
	defer scope.leave(&err)
	id, err := c.addSimpleType(simpleType{
		Name: noQName(),
		ValueSpec: value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespacePreserve, WhitespacePresent: true,
			Base: value.AnySimpleTypeID, ListItem: value.NoType,
		},
	})
	if err != nil {
		return NoSimpleType, err
	}
	st, err := c.compileSimpleType(ctx, n, noQName())
	if err != nil {
		return NoSimpleType, err
	}
	final, err := simpleFinalMaskWithDefaultChecked(n, ctx.doc.defaults.FinalDefault)
	if err != nil {
		return NoSimpleType, err
	}
	st.Final = final
	if err := c.completeSimpleType(id, st); err != nil {
		return NoSimpleType, withSchemaCompileLocation(n, err)
	}
	return id, nil
}

func (c *compiler) compileSimpleType(ctx *schemaSetDocument, n *schemaNode, name QName) (simpleType, error) {
	child := simpleTypeDerivationChild(n)
	if child == nil {
		return simpleType{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("simpleType child order admitted no derivation", nil))
	}
	switch child.local {
	case vocab.XSDElemRestriction:
		return c.compileRestriction(ctx, child, name)
	case vocab.XSDElemList:
		return c.compileList(ctx, child, name)
	case vocab.XSDElemUnion:
		return c.compileUnion(ctx, child, name)
	default:
		return simpleType{}, xsderrors.InternalInvariant("simpleType child validator admitted "+child.local, nil)
	}
}

func simpleTypeDerivationChild(n *schemaNode) *schemaNode {
	for child := range n.xsdChildren() {
		if child.local == vocab.XSDElemAnnotation {
			continue
		}
		return child
	}
	return nil
}

func (c *compiler) compileRestriction(ctx *schemaSetDocument, n *schemaNode, name QName) (simpleType, error) {
	baseID, err := c.compileRestrictionBase(ctx, n)
	if err != nil {
		return simpleType{}, err
	}
	if err := checkSimpleRestrictionBase(baseID, value.AnySimpleTypeID); err != nil {
		return simpleType{}, withSchemaCompileLocation(n, err)
	}
	if err := checkSimpleTypeFinalAllows(c.rt.simpleTypeFinalMask(baseID), DerivationRestriction, simpleTypeFinalBaseRestriction); err != nil {
		return simpleType{}, withSchemaCompileLocation(n, err)
	}
	st := c.rt.derivedSimpleType(baseID, name)
	if err := withSchemaCompileLocation(n, c.compileFacetChildren(n, &st, baseID)); err != nil {
		return simpleType{}, err
	}
	if st.ValueSpec.Variety == value.Union {
		if err := c.chargeSimpleUnionMemberEntries(n, len(st.ValueSpec.Union)); err != nil {
			return simpleType{}, err
		}
	}
	return st, nil
}

func (c *compiler) compileRestrictionBase(ctx *schemaSetDocument, n *schemaNode) (SimpleTypeID, error) {
	children := schemaSimpleTypeChildren(n)
	if len(children) > 1 {
		return NoSimpleType, xsderrors.InternalInvariant("restriction child validator admitted multiple simpleType children", nil)
	}
	derivation, ok := semanticSource[*schemaBaseDerivationSource](&n.semantic)
	if !ok {
		return NoSimpleType, withSchemaCompileLocation(n, xsderrors.InternalInvariant("restriction node has no typed derivation source", nil))
	}
	baseSource := derivation.Base
	if baseSource.Lexical.Present {
		q, err := c.referenceQName(ctx, n, baseSource.Name)
		if err != nil {
			return NoSimpleType, err
		}
		id, err := c.compileSimpleByQName(q)
		return id, withSchemaCompileLocation(n, err)
	}
	if len(children) == 1 {
		return c.compileAnonymousSimple(ctx, children[0])
	}
	return NoSimpleType, withSchemaCompileLocation(n, xsderrors.InternalInvariant("restriction child order admitted no base type", nil))
}

func (c *compiler) compileList(ctx *schemaSetDocument, n *schemaNode, name QName) (simpleType, error) {
	item := NoSimpleType
	simpleTypeChildren := schemaSimpleTypeChildren(n)
	list, err := requiredSemanticSource[*schemaListDerivationSource](n)
	if err != nil {
		return simpleType{}, err
	}
	itemTypeSource := list.ItemType
	switch {
	case itemTypeSource.Lexical.Present:
		id, err := c.compileListItemType(ctx, n, itemTypeSource.Name)
		if err != nil {
			return simpleType{}, err
		}
		item = id
	case len(simpleTypeChildren) == 1:
		id, err := c.compileAnonymousSimple(ctx, simpleTypeChildren[0])
		if err != nil {
			return simpleType{}, err
		}
		item = id
	case len(simpleTypeChildren) > 1:
		return simpleType{}, xsderrors.InternalInvariant("list child validator admitted multiple simpleType children", nil)
	}
	if item == NoSimpleType {
		return simpleType{}, xsderrors.InternalInvariant("list source validator admitted missing item type", nil)
	}
	if err := c.admitListItemType(n, item); err != nil {
		return simpleType{}, err
	}
	return simpleType{
		Name: name,
		ValueSpec: value.TypeSpec{
			Variety: value.List, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Base: value.AnySimpleTypeID, ListItem: item,
		},
	}, nil
}

func (c *compiler) admitListItemType(n *schemaNode, item SimpleTypeID) error {
	if err := checkSimpleTypeFinalAllows(c.rt.simpleTypeFinalMask(item), derivationList, simpleTypeFinalListItem); err != nil {
		return withSchemaCompileLocation(n, err)
	}
	if c.simpleListItemReachesList(item) {
		return withSchemaCompileLocation(n, simpleListItemListReachError())
	}
	return c.admitNotationTypeUse(n, item)
}

func (c *compiler) compileListItemType(ctx *schemaSetDocument, n *schemaNode, itemType xml.Name) (SimpleTypeID, error) {
	q, err := c.referenceQName(ctx, n, itemType)
	if err != nil {
		return NoSimpleType, err
	}
	return c.compileSimpleTypeReference(n, q)
}

func (c *compiler) compileSimpleTypeReference(n *schemaNode, q QName) (SimpleTypeID, error) {
	if c.simpleTypeQNameKnown(q) {
		id, compileErr := c.compileSimpleByQName(q)
		return id, withSchemaCompileLocation(n, compileErr)
	}
	if c.typeQNameKnown(q) {
		missingErr := schemaComponentMissingError(schemaComponentSimpleType, c.rt.formatName(q))
		return NoSimpleType, withSchemaCompileLocation(n, missingErr)
	}
	if !c.typeQNameMayBeUnavailable(q) {
		missingErr := schemaComponentMissingError(schemaComponentSimpleType, c.rt.formatName(q))
		return NoSimpleType, withSchemaCompileLocation(n, missingErr)
	}
	return c.missingSimpleType()
}

func (c *compiler) compileUnion(ctx *schemaSetDocument, n *schemaNode, name QName) (simpleType, error) {
	compilation := simpleUnionCompilation{
		compiler: c,
		ctx:      ctx,
		typeDef: simpleType{
			Name: name,
			ValueSpec: value.TypeSpec{
				Variety: value.Union, Primitive: value.PrimitiveString,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
				Base: value.AnySimpleTypeID, ListItem: value.NoType,
			},
		},
		seen: make(map[SimpleTypeID]struct{}),
	}
	simpleTypeChildren := schemaSimpleTypeChildren(n)
	union, ok := semanticSource[*schemaUnionDerivationSource](&n.semantic)
	if !ok {
		return simpleType{}, withSchemaCompileLocation(n, xsderrors.InternalInvariant("union node has no typed derivation source", nil))
	}
	memberTypes := union.MemberTypes.Names
	for _, member := range memberTypes {
		if err := compilation.addNamedQName(n, member); err != nil {
			return simpleType{}, err
		}
	}
	for _, child := range simpleTypeChildren {
		if err := compilation.addAnonymous(child); err != nil {
			return simpleType{}, err
		}
	}
	if len(compilation.typeDef.ValueSpec.Union) == 0 {
		return simpleType{}, xsderrors.InternalInvariant("union source validator admitted missing member types", nil)
	}
	return compilation.typeDef, nil
}

type simpleUnionCompilation struct {
	compiler *compiler
	ctx      *schemaSetDocument
	seen     map[SimpleTypeID]struct{}
	typeDef  simpleType
}

func (c *simpleUnionCompilation) addNamedQName(node *schemaNode, name xml.Name) error {
	q, err := c.compiler.referenceQName(c.ctx, node, name)
	if err != nil {
		return err
	}
	id, err := c.compiler.compileSimpleTypeReference(node, q)
	if err != nil {
		return err
	}
	return c.add(node, id)
}

func (c *simpleUnionCompilation) addAnonymous(node *schemaNode) error {
	id, err := c.compiler.compileAnonymousSimple(c.ctx, node)
	if err != nil {
		return err
	}
	return c.add(node, id)
}

func (c *simpleUnionCompilation) add(node *schemaNode, id SimpleTypeID) error {
	if err := checkSimpleTypeFinalAllows(c.compiler.rt.simpleTypeFinalMask(id), derivationUnion, simpleTypeFinalUnionMember); err != nil {
		return withSchemaCompileLocation(node, err)
	}
	if err := c.compiler.admitNotationTypeUse(node, id); err != nil {
		return err
	}
	remaining := c.compiler.limits.MaxSimpleUnionMemberEntries - c.compiler.unionMemberEntries
	added, ok := c.compiler.rt.appendFlattenedUnionMember(&c.typeDef.ValueSpec.Union, id, c.seen, remaining)
	c.compiler.unionMemberEntries += added
	if !ok {
		return withSchemaCompileLocation(node, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple union members exceed MaxSimpleUnionMemberEntries", nil))
	}
	c.typeDef.unionContainsList = c.typeDef.unionContainsList || c.compiler.simpleListItemReachesList(id)
	return nil
}

func (c *compiler) chargeSimpleUnionMemberEntries(node *schemaNode, count int) error {
	if count > c.limits.MaxSimpleUnionMemberEntries-c.unionMemberEntries {
		return withSchemaCompileLocation(node, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "simple union members exceed MaxSimpleUnionMemberEntries", nil))
	}
	c.unionMemberEntries += count
	return nil
}
