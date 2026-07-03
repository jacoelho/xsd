package compile

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

// Compile compiles internal schema sources into a frozen validation runtime.
func Compile(opts Options, sources []source.Source) (*runtime.Schema, error) {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaNoSources, "at least one schema source is required")
	}
	c, err := newCompiler(limits)
	if err != nil {
		return nil, err
	}
	if err = c.load(sources); err != nil {
		return nil, err
	}
	if err = c.index(); err != nil {
		return nil, err
	}
	if err = c.compileGlobals(); err != nil {
		return nil, err
	}
	rt, err := freezeCompilerRuntime(c)
	if err != nil {
		return nil, err
	}
	return rt, nil
}

type schemaContext struct {
	doc              *rawDoc
	imports          map[string]bool
	targetNS         string
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
	simpleDone               map[runtime.QName]runtime.SimpleTypeID
	complexDone              map[runtime.QName]runtime.ComplexTypeID
	attributeDone            map[runtime.QName]runtime.AttributeID
	attrGroupDone            map[runtime.QName]runtime.AttributeUseSetID
	elementDone              map[runtime.QName]runtime.ElementID
	localDone                map[*rawNode]runtime.ElementID
	identityDeclared         map[*rawNode]runtime.IdentityConstraintID
	regexCategories          RegexCategoryCache
	deferredAnonymousComplex []deferredAnonymousComplex
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
	compilingElement map[runtime.QName]bool
	compilingAttr    map[runtime.QName]bool
	compilingLocal   map[*rawNode]bool
	compilingAttrGrp map[runtime.QName]bool
	compilingModel   map[*rawNode]bool
}

type compilerModelState struct {
	modelDone    map[*rawNode]runtime.ContentModelID
	modelDepth   map[*rawNode]int
	elementDepth int
}

type compiler struct {
	compilerBuildState
	compilerCycleState
	simpleValues  runtime.SimpleValueCallbacks
	builtinFacets runtime.BuiltinSimpleFacetStorage
	compilerIndexState
	names NameInterner
	compilerModelState
	compilerSourceState
	rt            runtime.Schema
	limits        Limits
	missingSimple runtime.SimpleTypeID
}

func newCompiler(limits Limits) (*compiler, error) {
	names, err := NewNameTable(limits.MaxSchemaNames)
	if err != nil {
		return nil, err
	}
	builtinSimpleTypeCount := runtime.BuiltinSimpleTypeCount()
	builtinAttributeCount := runtime.BuiltinAttributeCount()
	builtinComplexTypeCount := runtime.BuiltinComplexTypeCount()
	builtinGlobalTypeCount := runtime.BuiltinGlobalTypeCount()
	rt := runtime.Schema{
		Names:              names,
		GlobalElements:     make(map[runtime.QName]runtime.ElementID),
		GlobalAttributes:   make(map[runtime.QName]runtime.AttributeID, builtinAttributeCount),
		GlobalTypes:        make(map[runtime.QName]runtime.TypeID, builtinGlobalTypeCount),
		GlobalIdentities:   make(map[runtime.QName]runtime.IdentityConstraintID),
		Notations:          make(map[runtime.QName]bool),
		Substitutions:      make(map[runtime.ElementID][]runtime.ElementID),
		SubstitutionLookup: make(map[runtime.ElementID]map[runtime.QName]runtime.ElementID),
		SimpleTypes:        make([]runtime.SimpleType, 0, builtinSimpleTypeCount),
		Attributes:         make([]runtime.AttributeDecl, 0, builtinAttributeCount),
		ComplexTypes:       make([]runtime.ComplexType, 0, builtinComplexTypeCount),
		Wildcards:          make([]runtime.Wildcard, 0, 1),
		AttributeUseSets:   make([]runtime.AttributeUseSet, 0, 1),
		Models:             make([]runtime.ContentModel, 0, 1),
	}
	c := &compiler{
		builtinFacets:       runtime.NewBuiltinSimpleFacetStorage(),
		compilerSourceState: newCompilerSourceState(),
		compilerIndexState: compilerIndexState{
			simpleRaw:    make(map[runtime.QName]rawComponent),
			complexRaw:   make(map[runtime.QName]rawComponent),
			elementRaw:   make(map[runtime.QName]rawComponent),
			attributeRaw: make(map[runtime.QName]rawComponent),
			groupRaw:     make(map[runtime.QName]rawComponent),
			attrGroupRaw: make(map[runtime.QName]rawComponent),
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
			compilingElement: make(map[runtime.QName]bool),
			compilingAttr:    make(map[runtime.QName]bool),
			compilingLocal:   make(map[*rawNode]bool),
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
	c.names = NewNameInterner(&c.rt.Names)
	if err := c.addBuiltins(); err != nil {
		return nil, err
	}
	return c, nil
}

// The registerGlobal* helpers append a declaration and publish it in the
// matching Global* map in one step, so a slice entry and its map key cannot
// diverge (validateRuntimeGlobals checks the names still match at freeze).

func (c *compiler) registerGlobalElement(q runtime.QName, decl runtime.ElementDecl) (runtime.ElementID, error) {
	id, err := NextElementID(len(c.rt.Elements))
	if err != nil {
		return runtime.NoElement, err
	}
	c.rt.Elements = append(c.rt.Elements, decl)
	c.rt.GlobalElements[q] = id
	return id, nil
}

func (c *compiler) registerGlobalAttribute(q runtime.QName, decl runtime.AttributeDecl) (runtime.AttributeID, error) {
	id, err := NextAttributeID(len(c.rt.Attributes))
	if err != nil {
		return 0, err
	}
	c.rt.Attributes = append(c.rt.Attributes, decl)
	c.rt.GlobalAttributes[q] = id
	return id, nil
}

func (c *compiler) registerGlobalComplexType(q runtime.QName, ct runtime.ComplexType) (runtime.ComplexTypeID, error) {
	id, err := NextComplexTypeID(len(c.rt.ComplexTypes))
	if err != nil {
		return runtime.NoComplexType, err
	}
	c.rt.ComplexTypes = append(c.rt.ComplexTypes, ct)
	c.rt.GlobalTypes[q] = runtime.ComplexRef(id)
	return id, nil
}

func (c *compiler) registerGlobalSimpleType(q runtime.QName, st runtime.SimpleType) (runtime.SimpleTypeID, error) {
	id, err := NextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, st)
	c.rt.GlobalTypes[q] = runtime.SimpleRef(id)
	return id, nil
}

func (c *compiler) registerGlobalIdentity(q runtime.QName, ic runtime.IdentityConstraint) (runtime.IdentityConstraintID, error) {
	id, err := NextIdentityConstraintID(len(c.rt.Identities))
	if err != nil {
		return runtime.NoIdentityConstraint, err
	}
	c.rt.Identities = append(c.rt.Identities, ic)
	c.rt.GlobalIdentities[q] = id
	return id, nil
}

func (c *compiler) compileGlobals() error {
	for _, q := range SortedQNames(c.simpleRaw, c.rt.Names) {
		if _, err := c.compileSimpleByQName(q); err != nil {
			return err
		}
	}
	for _, q := range SortedQNames(c.complexRaw, c.rt.Names) {
		if _, err := c.compileComplexByQName(q); err != nil {
			return err
		}
	}
	for _, q := range SortedQNames(c.attributeRaw, c.rt.Names) {
		if _, err := c.compileAttributeByQName(q); err != nil {
			return err
		}
	}
	for _, q := range SortedQNames(c.attrGroupRaw, c.rt.Names) {
		if _, _, err := c.compileAttributeGroupByQName(q); err != nil {
			return err
		}
	}
	for _, q := range SortedQNames(c.groupRaw, c.rt.Names) {
		if err := c.compileModelGroupByQName(q); err != nil {
			return err
		}
	}
	if err := c.declareAllIdentityConstraints(); err != nil {
		return err
	}
	for _, q := range SortedQNames(c.elementRaw, c.rt.Names) {
		if _, err := c.compileElementByQName(q); err != nil {
			return err
		}
	}
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
	label := c.rt.Names.Format(q)
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
	for id, ct := range c.rt.ComplexTypes {
		if id == int(c.rt.Builtin.AnyType) || ct.Derivation != runtime.DerivationKindRestriction {
			continue
		}
		baseID, ok := ct.Base.Complex()
		if !ok || baseID == c.rt.Builtin.AnyType {
			continue
		}
		base := c.rt.ComplexTypes[baseID]
		if err := ValidateContentRestrictionWithModels(&c.rt, c.rt.Models, base.Content, ct.Content); err != nil {
			return err
		}
	}
	modelRT := newContentModelCompiler(&c.rt.Names, &c.rt, c.limits.MaxContentModelStates)
	updates, err := runtime.RestrictionChoiceLimitUpdates(
		&modelRT,
		c.rt.ComplexTypes,
		c.rt.Models,
		c.rt.Builtin.AnyType,
	)
	if err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	for _, update := range updates {
		id, err := c.addModel(update.Model)
		if err != nil {
			return err
		}
		ct := c.rt.ComplexTypes[update.ComplexType]
		ct.Content = id
		c.rt.ComplexTypes[update.ComplexType] = ct
	}
	return nil
}

func complexBlockMaskWithDefault(n *rawNode, def runtime.DerivationMask) (runtime.DerivationMask, error) {
	return derivationMaskWithDefaultChecked(n, def, ComplexTypeBlockDerivation)
}

func simpleFinalMaskWithDefaultChecked(n *rawNode, def runtime.DerivationMask) (runtime.DerivationMask, error) {
	return derivationMaskWithDefaultChecked(n, def, SimpleTypeFinalDerivation)
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
	return c.names.InternQName(ns, local)
}

func (c *compiler) validateAttributeDeclValueConstraintIdentity(decl *runtime.AttributeDecl) error {
	if err := runtime.ValidateAttributeDeclValueConstraintRuntime(&c.rt, decl.Type, decl.Default != nil, decl.Fixed != nil); err != nil {
		return invalidAttributeError(err)
	}
	return nil
}

func (c *compiler) validateAttributeDeclName(n *rawNode, q runtime.QName) error {
	if err := runtime.ValidateAttributeDeclName(&c.rt.Names, q); err != nil {
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
	if err := runtime.ValidateAttributeUseSetRecord(&c.rt.Names, &c.rt, set); err != nil {
		return invalidAttributeError(err)
	}
	return nil
}

func (c *compiler) compileSimpleByQName(q runtime.QName) (runtime.SimpleTypeID, error) {
	label := c.rt.Names.Format(q)
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
	id, err := c.registerGlobalSimpleType(q, runtime.SimpleType{Name: q, Variety: runtime.SimpleVarietyAtomic, Primitive: runtime.PrimitiveString, Base: c.rt.Builtin.AnySimpleType, ListItem: runtime.NoSimpleType, Whitespace: runtime.WhitespacePreserve})
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
	c.rt.SimpleTypes[id] = st
	return id, nil
}

func (c *compiler) compileAnonymousSimple(n *rawNode, ctx *schemaContext) (runtime.SimpleTypeID, error) {
	if err := checkLocalSimpleTypeAttributes(n); err != nil {
		return runtime.NoSimpleType, err
	}
	q, err := c.names.InternQName("", fmt.Sprintf("$simple%d", len(c.rt.SimpleTypes)))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	id, err := NextSimpleTypeID(len(c.rt.SimpleTypes))
	if err != nil {
		return runtime.NoSimpleType, err
	}
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, runtime.SimpleType{Name: q, Variety: runtime.SimpleVarietyAtomic, Primitive: runtime.PrimitiveString, Base: c.rt.Builtin.AnySimpleType, ListItem: runtime.NoSimpleType, Whitespace: runtime.WhitespacePreserve})
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
	c.rt.SimpleTypes[id] = st
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
	if err := CheckSimpleRestrictionBase(baseID, c.rt.Builtin.AnySimpleType); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	base := c.rt.SimpleTypes[baseID]
	if err := CheckSimpleTypeFinalAllows(base.Final, runtime.DerivationRestriction, SimpleTypeFinalBaseRestriction); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	st := derivedSimpleType(base, baseID, name)
	if err := c.compileFacets(n, &st, baseID, baseID); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	return st, nil
}

// derivedSimpleType copies base as the starting point of a restriction step.
// Facet slices are cloned lazily when the restriction declares a facet.
func derivedSimpleType(base runtime.SimpleType, baseID runtime.SimpleTypeID, name runtime.QName) runtime.SimpleType {
	st := base
	st.Name = name
	st.Base = baseID
	st.Final = 0
	st.Union = slices.Clone(base.Union)
	return st
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
	if err := CheckSimpleTypeFinalAllows(c.rt.SimpleTypes[item].Final, runtime.DerivationList, SimpleTypeFinalListItem); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	if err := CheckSimpleListItemType(c.rt.SimpleTypes, item); err != nil {
		return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
	}
	return runtime.SimpleType{Name: name, Variety: runtime.SimpleVarietyList, Primitive: runtime.PrimitiveString, Base: c.rt.Builtin.AnySimpleType, Whitespace: runtime.WhitespaceCollapse, ListItem: item}, nil
}

func (c *compiler) compileListItemType(n *rawNode, ctx *schemaContext, itemType string) (runtime.SimpleTypeID, error) {
	q, err := c.resolveQNameChecked(n, ctx, itemType)
	if err != nil {
		return runtime.NoSimpleType, err
	}
	if c.simpleTypeQNameKnown(q) {
		id, err := c.compileSimpleByQName(q)
		return id, withSchemaCompileLocation(n, err)
	}
	if c.typeQNameKnown(q) {
		err := CheckSchemaComponentExists(SchemaComponentSimpleType, false, c.rt.Names.Format(q))
		return runtime.NoSimpleType, withSchemaCompileLocation(n, err)
	}
	return c.missingSimpleType()
}

func (c *compiler) compileUnion(n *rawNode, ctx *schemaContext, name runtime.QName) (runtime.SimpleType, error) {
	if err := checkChildOrderRules(n, simpleUnionChildOrder); err != nil {
		return runtime.SimpleType{}, err
	}
	st := runtime.SimpleType{Name: name, Variety: runtime.SimpleVarietyUnion, Primitive: runtime.PrimitiveString, Base: c.rt.Builtin.AnySimpleType, ListItem: runtime.NoSimpleType, Whitespace: runtime.WhitespaceCollapse}
	simpleTypeChildren := n.xsSimpleTypeChildren()
	mt, hasMemberTypes := n.attr(vocab.XSDAttrMemberTypes)
	memberTypes, err := parseUnionMemberTypes(n, mt, hasMemberTypes, len(simpleTypeChildren) != 0)
	if err != nil {
		return runtime.SimpleType{}, err
	}
	for _, part := range memberTypes {
		q, err := c.resolveQNameChecked(n, ctx, part)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		id, err := c.compileSimpleByQName(q)
		if err != nil {
			return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
		}
		if err := CheckSimpleTypeFinalAllows(c.rt.SimpleTypes[id].Final, runtime.DerivationUnion, SimpleTypeFinalUnionMember); err != nil {
			return runtime.SimpleType{}, withSchemaCompileLocation(n, err)
		}
		st.Union = append(st.Union, id)
	}
	for _, child := range simpleTypeChildren {
		id, err := c.compileAnonymousSimple(child, ctx)
		if err != nil {
			return runtime.SimpleType{}, err
		}
		if err := CheckSimpleTypeFinalAllows(c.rt.SimpleTypes[id].Final, runtime.DerivationUnion, SimpleTypeFinalUnionMember); err != nil {
			return runtime.SimpleType{}, withSchemaCompileLocation(child, err)
		}
		st.Union = append(st.Union, id)
	}
	if len(st.Union) == 0 {
		return runtime.SimpleType{}, xsderrors.InternalInvariant("union source validator admitted missing member types")
	}
	return st, nil
}
