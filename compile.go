package xsd

import (
	"fmt"
	"slices"
	"strings"
)

// Engine is an immutable compiled schema validator.
type Engine struct {
	rt *runtimeSchema
}

// CompileOptions controls schema compilation resource limits.
type CompileOptions struct {
	// MaxSchemaDepth caps nested schema XML elements. Zero uses the default.
	MaxSchemaDepth int
	// MaxSchemaAttributes caps attributes on one schema XML element. Zero uses the default.
	MaxSchemaAttributes int
	// MaxSchemaTokenBytes caps retained schema XML token payloads. Zero uses the default.
	MaxSchemaTokenBytes int
	// MaxSchemaNames caps interned schema names, including built-ins. Zero means no explicit limit.
	MaxSchemaNames int
	// MaxFiniteOccurs caps finite maxOccurs values. Zero means no explicit limit.
	MaxFiniteOccurs uint64
}

const (
	defaultMaxSchemaDepth      = 256
	defaultMaxSchemaAttributes = 256
	defaultMaxSchemaTokenBytes = 4 << 20
)

type compileLimits struct {
	maxSchemaDepth      int
	maxSchemaAttributes int
	maxSchemaTokenBytes int
	maxSchemaNames      int
	maxFiniteOccurs     uint64
}

// Compile compiles schema sources into an immutable validation engine.
func Compile(sources ...SchemaSource) (*Engine, error) {
	return compileWithOptions(CompileOptions{}, sources)
}

// CompileWithOptions compiles schema sources with explicit resource limits.
func CompileWithOptions(opts CompileOptions, sources ...SchemaSource) (*Engine, error) {
	return compileWithOptions(opts, sources)
}

func compileWithOptions(opts CompileOptions, sources []SchemaSource) (*Engine, error) {
	limits, err := normalizeCompileOptions(opts)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, schemaCompile(ErrSchemaNoSources, "at least one schema source is required")
	}
	c := newCompiler(limits)
	if err := c.checkLimits(); err != nil {
		return nil, err
	}
	if err := c.load(sources); err != nil {
		return nil, err
	}
	if err := c.checkLimits(); err != nil {
		return nil, err
	}
	if err := c.index(); err != nil {
		return nil, err
	}
	if err := c.checkLimits(); err != nil {
		return nil, err
	}
	if err := c.compileGlobals(); err != nil {
		return nil, err
	}
	if err := c.checkLimits(); err != nil {
		return nil, err
	}
	rt := c.rt
	return &Engine{rt: &rt}, nil
}

func normalizeCompileOptions(opts CompileOptions) (compileLimits, error) {
	depth, err := compileLimitOrDefault("MaxSchemaDepth", opts.MaxSchemaDepth, defaultMaxSchemaDepth)
	if err != nil {
		return compileLimits{}, err
	}
	attrs, err := compileLimitOrDefault("MaxSchemaAttributes", opts.MaxSchemaAttributes, defaultMaxSchemaAttributes)
	if err != nil {
		return compileLimits{}, err
	}
	tokenBytes, err := compileLimitOrDefault("MaxSchemaTokenBytes", opts.MaxSchemaTokenBytes, defaultMaxSchemaTokenBytes)
	if err != nil {
		return compileLimits{}, err
	}
	if opts.MaxSchemaNames < 0 {
		return compileLimits{}, schemaCompile(ErrSchemaLimit, "MaxSchemaNames cannot be negative")
	}
	return compileLimits{
		maxSchemaDepth:      depth,
		maxSchemaAttributes: attrs,
		maxSchemaTokenBytes: tokenBytes,
		maxSchemaNames:      opts.MaxSchemaNames,
		maxFiniteOccurs:     opts.MaxFiniteOccurs,
	}, nil
}

func compileLimitOrDefault(name string, value, def int) (int, error) {
	if value < 0 {
		return 0, schemaCompile(ErrSchemaLimit, name+" cannot be negative")
	}
	if value == 0 {
		return def, nil
	}
	return value, nil
}

type schemaContext struct {
	doc              *rawDoc
	imports          map[string]bool
	targetNS         string
	elementQualified bool
	attrQualified    bool
	blockDefault     derivationMask
	finalDefault     derivationMask
}

type rawComponent struct {
	node *rawNode
	ctx  *schemaContext
}

type compiler struct {
	elementDone      map[qName]elementID
	compilingComplex map[qName]bool
	sources          map[string][]byte
	imports          map[string]map[string]bool
	adoptTarget      map[string]string
	contexts         map[*rawDoc]*schemaContext
	simpleRaw        map[qName]rawComponent
	complexRaw       map[qName]rawComponent
	elementRaw       map[qName]rawComponent
	attributeRaw     map[qName]rawComponent
	groupRaw         map[qName]rawComponent
	attrGroupRaw     map[qName]rawComponent
	simpleDone       map[qName]simpleTypeID
	complexDone      map[qName]complexTypeID
	identityDeclared map[*rawNode]identityConstraintID
	compilingModel   map[*rawNode]bool
	compilingAttr    map[qName]bool
	modelDepth       map[*rawNode]int
	localDone        map[*rawNode]elementID
	compilingSimple  map[qName]bool
	attributeDone    map[qName]attributeID
	compilingElement map[qName]bool
	modelDone        map[*rawNode]contentModelID
	compilingLocal   map[*rawNode]bool
	compilingAttrGrp map[qName]bool
	docs             []*rawDoc
	rt               runtimeSchema
	limits           compileLimits
	elementDepth     int
	missingSimple    simpleTypeID
}

func newCompiler(limits compileLimits) *compiler {
	names := newNameTable(limits.maxSchemaNames)
	rt := runtimeSchema{
		Names:            names,
		GlobalElements:   make(map[qName]elementID),
		GlobalAttributes: make(map[qName]attributeID),
		GlobalTypes:      make(map[qName]typeID),
		GlobalIdentities: make(map[qName]identityConstraintID),
		Notations:        make(map[string]bool),
		Substitutions:    make(map[elementID][]elementID),
	}
	c := &compiler{
		rt:               rt,
		sources:          make(map[string][]byte),
		imports:          make(map[string]map[string]bool),
		adoptTarget:      make(map[string]string),
		contexts:         make(map[*rawDoc]*schemaContext),
		simpleRaw:        make(map[qName]rawComponent),
		complexRaw:       make(map[qName]rawComponent),
		elementRaw:       make(map[qName]rawComponent),
		attributeRaw:     make(map[qName]rawComponent),
		groupRaw:         make(map[qName]rawComponent),
		attrGroupRaw:     make(map[qName]rawComponent),
		simpleDone:       make(map[qName]simpleTypeID),
		complexDone:      make(map[qName]complexTypeID),
		elementDone:      make(map[qName]elementID),
		attributeDone:    make(map[qName]attributeID),
		modelDone:        make(map[*rawNode]contentModelID),
		modelDepth:       make(map[*rawNode]int),
		localDone:        make(map[*rawNode]elementID),
		compilingSimple:  make(map[qName]bool),
		compilingComplex: make(map[qName]bool),
		compilingElement: make(map[qName]bool),
		compilingAttr:    make(map[qName]bool),
		compilingLocal:   make(map[*rawNode]bool),
		compilingAttrGrp: make(map[qName]bool),
		compilingModel:   make(map[*rawNode]bool),
		identityDeclared: make(map[*rawNode]identityConstraintID),
		missingSimple:    noSimpleType,
		limits:           limits,
	}
	c.addBuiltins()
	return c
}

func (c *compiler) checkLimits() error {
	return c.rt.Names.limitErr
}

func (c *compiler) compileGlobals() error {
	for _, q := range sortedQNames(c.simpleRaw, c.rt.Names) {
		if _, err := c.compileSimpleByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedQNames(c.complexRaw, c.rt.Names) {
		if _, err := c.compileComplexByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedQNames(c.attributeRaw, c.rt.Names) {
		if _, err := c.compileAttributeByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedQNames(c.attrGroupRaw, c.rt.Names) {
		if _, _, err := c.compileAttributeGroupByQName(q); err != nil {
			return err
		}
	}
	for _, q := range sortedQNames(c.groupRaw, c.rt.Names) {
		if err := c.compileModelGroupByQName(q); err != nil {
			return err
		}
	}
	if err := c.declareAllIdentityConstraints(); err != nil {
		return err
	}
	for _, q := range sortedQNames(c.elementRaw, c.rt.Names) {
		if _, err := c.compileElementByQName(q); err != nil {
			return err
		}
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
	c.classifySimpleIdentities()
	return nil
}

func (c *compiler) classifySimpleIdentities() {
	memo := make([]simpleIdentityKind, len(c.rt.SimpleTypes))
	visiting := make([]bool, len(c.rt.SimpleTypes))
	for id := range c.rt.SimpleTypes {
		c.rt.SimpleTypes[id].Identity = c.simpleIdentityKind(simpleTypeID(id), memo, visiting)
	}
}

func (c *compiler) simpleIdentityKind(id simpleTypeID, memo []simpleIdentityKind, visiting []bool) simpleIdentityKind {
	if id == noSimpleType || int(id) >= len(c.rt.SimpleTypes) {
		return simpleIdentityNone
	}
	if memo[id] != simpleIdentityNone {
		return memo[id]
	}
	if id == c.rt.Builtin.ID {
		memo[id] = simpleIdentityID
		return simpleIdentityID
	}
	if id == c.rt.Builtin.IDREF {
		memo[id] = simpleIdentityIDREF
		return simpleIdentityIDREF
	}
	if visiting[id] {
		return simpleIdentityNone
	}
	visiting[id] = true
	st := c.rt.SimpleTypes[id]
	kind := simpleIdentityNone
	switch st.Variety {
	case varietyAtomic:
		kind = c.simpleIdentityKind(st.Base, memo, visiting)
	case varietyList:
		if c.simpleIdentityKind(st.ListItem, memo, visiting) == simpleIdentityIDREF {
			kind = simpleIdentityIDREFList
		}
	}
	visiting[id] = false
	memo[id] = kind
	return kind
}

func (c *compiler) checkCompiledElementDeclarationsConsistent() error {
	for _, model := range c.rt.Models {
		if err := c.checkElementDeclarationsConsistent(model); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileModelGroupByQName(q qName) error {
	raw, ok := c.groupRaw[q]
	if !ok {
		return schemaCompile(ErrSchemaReference, "unknown model group "+c.rt.Names.Format(q))
	}
	modelNode := firstModelChild(raw.node)
	if modelNode == nil {
		return schemaCompile(ErrSchemaContentModel, "model group has no content "+c.rt.Names.Format(q))
	}
	_, err := c.compileModel(modelNode, raw.ctx)
	return err
}

func (c *compiler) validateCompiledComplexRestrictions() error {
	for id, ct := range c.rt.ComplexTypes {
		if id == int(c.rt.Builtin.AnyType) || ct.Derivation != derivationRestriction || ct.Base.Kind != typeComplex {
			continue
		}
		baseID := complexTypeID(ct.Base.ID)
		if baseID == noComplexType || baseID == c.rt.Builtin.AnyType {
			continue
		}
		base := c.rt.ComplexTypes[baseID]
		if err := c.validateContentRestriction(base.Content, ct.Content); err != nil {
			return err
		}
	}
	return nil
}

func parseDerivationMask(v string) derivationMask {
	var m derivationMask
	for p := range strings.FieldsSeq(v) {
		switch p {
		case "#all":
			return blockExtension | blockRestriction | blockSubstitution | blockList | blockUnion
		case "extension":
			m |= blockExtension
		case "restriction":
			m |= blockRestriction
		case "substitution":
			m |= blockSubstitution
		case "list":
			m |= blockList
		case "union":
			m |= blockUnion
		}
	}
	return m
}

func complexBlockMaskWithDefault(n *rawNode, def derivationMask) derivationMask {
	if v, ok := n.attr("block"); ok {
		return parseDerivationMask(v) & (blockExtension | blockRestriction)
	}
	return def & (blockExtension | blockRestriction)
}

func simpleFinalMaskWithDefaultChecked(n *rawNode, def derivationMask) (derivationMask, error) {
	if v, ok := n.attr("final"); ok {
		return parseSimpleFinalMaskChecked(v)
	}
	return def & (blockRestriction | blockList | blockUnion), nil
}

func parseSimpleFinalMaskChecked(v string) (derivationMask, error) {
	var m derivationMask
	fieldCount := 0
	for range strings.FieldsSeq(v) {
		fieldCount++
	}
	i := 0
	for p := range strings.FieldsSeq(v) {
		switch p {
		case "#all":
			if fieldCount != 1 || i != 0 {
				return 0, schemaCompile(ErrSchemaInvalidAttribute, "simpleType final cannot combine #all with other values")
			}
			return blockRestriction | blockList | blockUnion, nil
		case "restriction":
			m |= blockRestriction
		case "list":
			m |= blockList
		case "union":
			m |= blockUnion
		default:
			return 0, schemaCompile(ErrSchemaInvalidAttribute, "invalid simpleType final value "+p)
		}
		i++
	}
	return m, nil
}

func parseDerivationMaskChecked(v string, allowSubstitution bool, label string) (derivationMask, error) {
	var m derivationMask
	fieldCount := 0
	for range strings.FieldsSeq(v) {
		fieldCount++
	}
	i := 0
	for p := range strings.FieldsSeq(v) {
		switch p {
		case "#all":
			if fieldCount != 1 || i != 0 {
				return 0, schemaCompile(ErrSchemaInvalidAttribute, label+" cannot combine #all with other values")
			}
			all := blockExtension | blockRestriction
			if allowSubstitution {
				all |= blockSubstitution
			}
			return all, nil
		case "extension":
			m |= blockExtension
		case "restriction":
			m |= blockRestriction
		case "substitution":
			if !allowSubstitution {
				return 0, schemaCompile(ErrSchemaInvalidAttribute, label+" cannot contain substitution")
			}
			m |= blockSubstitution
		default:
			return 0, schemaCompile(ErrSchemaInvalidAttribute, "invalid "+label+" value "+p)
		}
		i++
	}
	return m, nil
}

func derivationMaskWithDefault(n *rawNode, attr string, def derivationMask) derivationMask {
	if v, ok := n.attr(attr); ok {
		return parseDerivationMask(v)
	}
	return def
}

func derivationMaskWithDefaultChecked(n *rawNode, attr string, def derivationMask, allowSubstitution bool, label string) (derivationMask, error) {
	if v, ok := n.attr(attr); ok {
		return parseDerivationMaskChecked(v, allowSubstitution, label)
	}
	return def, nil
}

func (c *compiler) resolveQNameChecked(n *rawNode, ctx *schemaContext, lexical string) (qName, error) {
	ns, local, err := n.resolveQName(lexical)
	if err != nil {
		return qName{}, err
	}
	if ns == "" && ctx != nil && ctx.targetNS != "" && c.adoptTarget[ctx.doc.name] != "" {
		ns = ctx.targetNS
	}
	if ctx != nil && !ctx.referenceNamespaceVisible(ns) {
		return qName{}, schemaCompile(ErrSchemaReference, "namespace is not imported: "+ns)
	}
	return c.rt.Names.InternQName(ns, local), nil
}

func (ctx *schemaContext) referenceNamespaceVisible(ns string) bool {
	if ns == xsdNamespaceURI || ns == xmlNamespaceURI {
		return true
	}
	if ns == ctx.targetNS {
		return true
	}
	return ctx.imports != nil && ctx.imports[ns]
}

func (c *compiler) compileSimpleByQName(q qName) (simpleTypeID, error) {
	if c.compilingSimple[q] {
		return noSimpleType, schemaCompile(ErrSchemaReference, "cyclic simple type "+c.rt.Names.Format(q))
	}
	if id, ok := c.simpleDone[q]; ok {
		return id, nil
	}
	raw, ok := c.simpleRaw[q]
	if !ok {
		return noSimpleType, schemaCompile(ErrSchemaReference, "unknown simple type "+c.rt.Names.Format(q))
	}
	c.compilingSimple[q] = true
	defer delete(c.compilingSimple, q)
	id := simpleTypeID(len(c.rt.SimpleTypes))
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{Name: q, Variety: varietyAtomic, Primitive: primString, Base: c.rt.Builtin.AnySimpleType, Whitespace: whitespacePreserve})
	c.simpleDone[q] = id
	c.rt.GlobalTypes[q] = typeID{Kind: typeSimple, ID: uint32(id)}
	st, err := c.compileSimpleType(raw.node, raw.ctx, q)
	if err != nil {
		return noSimpleType, err
	}
	st.Name = q
	final, err := simpleFinalMaskWithDefaultChecked(raw.node, raw.ctx.finalDefault)
	if err != nil {
		return noSimpleType, err
	}
	st.Final = final
	c.rt.SimpleTypes[id] = st
	return id, nil
}

func (c *compiler) compileAnonymousSimple(n *rawNode, ctx *schemaContext) (simpleTypeID, error) {
	if _, ok := n.attr("name"); ok {
		return noSimpleType, schemaCompile(ErrSchemaInvalidAttribute, "local simpleType cannot have name")
	}
	q := c.rt.Names.InternQName("", fmt.Sprintf("$simple%d", len(c.rt.SimpleTypes)))
	id := simpleTypeID(len(c.rt.SimpleTypes))
	c.rt.SimpleTypes = append(c.rt.SimpleTypes, simpleType{Name: q, Variety: varietyAtomic, Primitive: primString, Base: c.rt.Builtin.AnySimpleType, Whitespace: whitespacePreserve})
	st, err := c.compileSimpleType(n, ctx, q)
	if err != nil {
		return noSimpleType, err
	}
	st.Name = q
	final, err := simpleFinalMaskWithDefaultChecked(n, ctx.finalDefault)
	if err != nil {
		return noSimpleType, err
	}
	st.Final = final
	c.rt.SimpleTypes[id] = st
	return id, nil
}

func (c *compiler) compileSimpleType(n *rawNode, ctx *schemaContext, name qName) (simpleType, error) {
	if err := validateSimpleTypeChildren(n); err != nil {
		return simpleType{}, err
	}
	children := n.xsContentChildren()
	if len(children) != 1 {
		return simpleType{}, schemaCompile(ErrSchemaContentModel, "simpleType must contain one restriction, list, or union")
	}
	switch children[0].Name.Local {
	case "restriction":
		return c.compileRestriction(children[0], ctx, name)
	case "list":
		return c.compileList(children[0], ctx, name)
	case "union":
		return c.compileUnion(children[0], ctx, name)
	default:
		return simpleType{}, schemaCompile(ErrSchemaContentModel, "unsupported simpleType child "+children[0].Name.Local)
	}
}

func validateSimpleTypeChildren(n *rawNode) error {
	seenAnnotation := false
	seenVariety := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		if child.Name.Local == "annotation" {
			if seenAnnotation || seenVariety {
				return schemaCompile(ErrSchemaContentModel, "simpleType annotation must be first")
			}
			seenAnnotation = true
			continue
		}
		switch child.Name.Local {
		case "restriction", "list", "union":
			if seenVariety {
				return schemaCompile(ErrSchemaContentModel, "simpleType can contain one restriction, list, or union")
			}
			seenVariety = true
		default:
			return schemaCompile(ErrSchemaContentModel, "unsupported simpleType child "+child.Name.Local)
		}
	}
	return nil
}

func (c *compiler) compileRestriction(n *rawNode, ctx *schemaContext, name qName) (simpleType, error) {
	if err := validateSimpleDerivationChildren(n, false); err != nil {
		return simpleType{}, err
	}
	var baseID simpleTypeID
	simpleTypeChildren := n.xsChildren("simpleType")
	if len(simpleTypeChildren) > 1 {
		return simpleType{}, schemaCompile(ErrSchemaContentModel, "restriction can contain one simpleType")
	}
	if base, ok := n.attr("base"); ok {
		if len(simpleTypeChildren) != 0 {
			return simpleType{}, schemaCompile(ErrSchemaContentModel, "restriction cannot have both base and simpleType")
		}
		q, err := c.resolveQNameChecked(n, ctx, base)
		if err != nil {
			return simpleType{}, err
		}
		id, err := c.compileSimpleByQName(q)
		if err != nil {
			return simpleType{}, err
		}
		baseID = id
	} else if len(simpleTypeChildren) == 1 {
		id, err := c.compileAnonymousSimple(simpleTypeChildren[0], ctx)
		if err != nil {
			return simpleType{}, err
		}
		baseID = id
	} else {
		return simpleType{}, schemaCompile(ErrSchemaReference, "simple restriction missing base")
	}
	if baseID == c.rt.Builtin.AnySimpleType {
		return simpleType{}, schemaCompile(ErrSchemaReference, "simple type cannot restrict xs:anySimpleType")
	}
	base := c.rt.SimpleTypes[baseID]
	if base.Final&blockRestriction != 0 {
		return simpleType{}, schemaCompile(ErrSchemaReference, "base simple type final blocks restriction")
	}
	st := base
	st.Name = name
	st.Base = baseID
	st.Final = 0
	st.Facets = cloneFacetSet(base.Facets)
	st.Union = slices.Clone(base.Union)
	if ws, ok := n.attr("whiteSpace"); ok {
		mode, ok := parseWhitespaceChecked(ws)
		if !ok {
			return simpleType{}, schemaCompile(ErrSchemaFacet, "invalid whiteSpace facet "+ws)
		}
		if !validWhitespaceRestriction(base.Whitespace, mode) {
			return simpleType{}, schemaCompile(ErrSchemaFacet, "whiteSpace cannot loosen base whiteSpace")
		}
		st.Whitespace = mode
	}
	if err := c.compileFacets(n, &st, baseID); err != nil {
		return simpleType{}, err
	}
	return st, nil
}

func cloneFacetSet(f facetSet) facetSet {
	out := f
	out.Enumeration = slices.Clone(f.Enumeration)
	out.Patterns = make([]patternGroup, len(f.Patterns))
	for i := range f.Patterns {
		out.Patterns[i].Patterns = slices.Clone(f.Patterns[i].Patterns)
	}
	return out
}

func (c *compiler) compileList(n *rawNode, ctx *schemaContext, name qName) (simpleType, error) {
	if err := validateSimpleDerivationChildren(n, false); err != nil {
		return simpleType{}, err
	}
	item := noSimpleType
	simpleTypeChildren := n.xsChildren("simpleType")
	if itemType, ok := n.attr("itemType"); ok {
		if len(simpleTypeChildren) != 0 {
			return simpleType{}, schemaCompile(ErrSchemaContentModel, "list cannot have both itemType and simpleType")
		}
		q, err := c.resolveQNameChecked(n, ctx, itemType)
		if err != nil {
			return simpleType{}, err
		}
		if _, ok := c.simpleDone[q]; ok {
			id, err := c.compileSimpleByQName(q)
			if err != nil {
				return simpleType{}, err
			}
			item = id
		} else if _, ok := c.simpleRaw[q]; ok {
			id, err := c.compileSimpleByQName(q)
			if err != nil {
				return simpleType{}, err
			}
			item = id
		} else if c.typeQNameKnown(q) {
			return simpleType{}, schemaCompile(ErrSchemaReference, "unknown simple type "+c.rt.Names.Format(q))
		} else {
			item = c.missingSimpleType()
		}
	} else if len(simpleTypeChildren) == 1 {
		id, err := c.compileAnonymousSimple(simpleTypeChildren[0], ctx)
		if err != nil {
			return simpleType{}, err
		}
		item = id
	} else if len(simpleTypeChildren) > 1 {
		return simpleType{}, schemaCompile(ErrSchemaContentModel, "list can contain one simpleType")
	}
	if item == noSimpleType {
		return simpleType{}, schemaCompile(ErrSchemaReference, "list missing item type")
	}
	if c.rt.SimpleTypes[item].Final&blockList != 0 {
		return simpleType{}, schemaCompile(ErrSchemaReference, "item simple type final blocks list")
	}
	if c.simpleTypeHasListVariety(item, make(map[simpleTypeID]bool)) {
		return simpleType{}, schemaCompile(ErrSchemaContentModel, "list item type cannot be a list type")
	}
	st := simpleType{Name: name, Variety: varietyList, Primitive: primString, Base: c.rt.Builtin.AnySimpleType, Whitespace: whitespaceCollapse, ListItem: item}
	if err := c.compileFacets(n, &st, c.rt.Builtin.AnySimpleType); err != nil {
		return simpleType{}, err
	}
	return st, nil
}

func (c *compiler) compileUnion(n *rawNode, ctx *schemaContext, name qName) (simpleType, error) {
	if err := validateSimpleDerivationChildren(n, true); err != nil {
		return simpleType{}, err
	}
	st := simpleType{Name: name, Variety: varietyUnion, Primitive: primString, Base: c.rt.Builtin.AnySimpleType, Whitespace: whitespaceCollapse}
	if mt, ok := n.attr("memberTypes"); ok {
		for part := range strings.FieldsSeq(mt) {
			q, err := c.resolveQNameChecked(n, ctx, part)
			if err != nil {
				return simpleType{}, err
			}
			id, err := c.compileSimpleByQName(q)
			if err != nil {
				return simpleType{}, err
			}
			if c.rt.SimpleTypes[id].Final&blockUnion != 0 {
				return simpleType{}, schemaCompile(ErrSchemaReference, "member simple type final blocks union")
			}
			st.Union = append(st.Union, id)
		}
	}
	for _, child := range n.xsChildren("simpleType") {
		id, err := c.compileAnonymousSimple(child, ctx)
		if err != nil {
			return simpleType{}, err
		}
		if c.rt.SimpleTypes[id].Final&blockUnion != 0 {
			return simpleType{}, schemaCompile(ErrSchemaReference, "member simple type final blocks union")
		}
		st.Union = append(st.Union, id)
	}
	if len(st.Union) == 0 {
		return simpleType{}, schemaCompile(ErrSchemaReference, "union missing member types")
	}
	if err := c.compileFacets(n, &st, c.rt.Builtin.AnySimpleType); err != nil {
		return simpleType{}, err
	}
	return st, nil
}

func validateSimpleDerivationChildren(n *rawNode, multipleSimpleTypes bool) error {
	seenAnnotation := false
	seenSimpleType := false
	seenFacet := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		switch child.Name.Local {
		case "annotation":
			if seenAnnotation || seenSimpleType || seenFacet {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
			seenAnnotation = true
		case "simpleType":
			if seenFacet {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" simpleType must precede facets")
			}
			if seenSimpleType && !multipleSimpleTypes {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" can contain one simpleType")
			}
			seenSimpleType = true
		default:
			if !isFacetNode(child.Name.Local) {
				return schemaCompile(ErrSchemaContentModel, "invalid "+n.Name.Local+" child "+child.Name.Local)
			}
			seenFacet = true
		}
	}
	return nil
}
