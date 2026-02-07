package runtimecompile

import (
	"cmp"
	"fmt"
	"slices"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/pipeline"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemaops"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/typegraph"
	"github.com/jacoelho/xsd/internal/types"
)

// BuildConfig configures runtime schema compilation.
type BuildConfig struct {
	Limits         models.Limits
	MaxOccursLimit uint32
}

// BuildSchema compiles a parsed schema into a runtime schema model.
func BuildSchema(sch *parser.Schema, cfg BuildConfig) (*runtime.Schema, error) {
	prepared, err := pipeline.Prepare(sch)
	if err != nil {
		return nil, fmt.Errorf("runtime build: %w", err)
	}
	return BuildPrepared(prepared, cfg)
}

// BuildPrepared compiles a prepared schema into a runtime schema model.
func BuildPrepared(prepared *pipeline.PreparedSchema, cfg BuildConfig) (*runtime.Schema, error) {
	if prepared == nil || prepared.Schema == nil {
		return nil, fmt.Errorf("runtime build: prepared schema is nil")
	}
	if prepared.Registry == nil {
		return nil, fmt.Errorf("runtime build: prepared registry is nil")
	}
	if prepared.Refs == nil {
		return nil, fmt.Errorf("runtime build: prepared references are nil")
	}
	sch := prepared.Schema
	reg := prepared.Registry
	refs := prepared.Refs

	validators, err := CompileValidators(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("runtime build: compile validators: %w", err)
	}
	maxOccursLimit := cfg.MaxOccursLimit
	if maxOccursLimit == 0 {
		maxOccursLimit = defaultMaxOccursLimit
	}

	builder := &schemaBuilder{
		schema:     sch,
		registry:   reg,
		refs:       refs,
		validators: validators,
		limits:     cfg.Limits,
		builder:    runtime.NewBuilder(),
		typeIDs:    make(map[schema.TypeID]runtime.TypeID),
		elemIDs:    make(map[schema.ElemID]runtime.ElemID),
		attrIDs:    make(map[schema.AttrID]runtime.AttrID),
		builtinIDs: make(map[types.TypeName]runtime.TypeID),
		complexIDs: make(map[runtime.TypeID]uint32),
		maxOccurs:  maxOccursLimit,
	}
	rt, err := builder.build()
	if err != nil {
		return nil, err
	}
	sch.Phase = parser.PhaseRuntimeReady
	return rt, nil
}

type schemaBuilder struct {
	err             error
	attrIDs         map[schema.AttrID]runtime.AttrID
	elemIDs         map[schema.ElemID]runtime.ElemID
	validators      *CompiledValidators
	registry        *schema.Registry
	typeIDs         map[schema.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	builtinIDs      map[types.TypeName]runtime.TypeID
	refs            *schema.ResolvedReferences
	anyElementRules map[*types.AnyElement]runtime.WildcardID
	rt              *runtime.Schema
	paths           []runtime.PathProgram
	wildcards       []runtime.WildcardRule
	wildcardNS      []runtime.NamespaceID
	notations       []runtime.SymbolID
	maxOccurs       uint32
	anyTypeComplex  uint32
	limits          models.Limits
}

const defaultMaxOccursLimit = 1_000_000

func (b *schemaBuilder) build() (*runtime.Schema, error) {
	if err := b.initSymbols(); err != nil {
		return nil, err
	}
	if b.err != nil {
		return nil, b.err
	}
	rt, err := b.builder.Build()
	if err != nil {
		return nil, err
	}
	b.rt = rt
	b.rt.RootPolicy = runtime.RootStrict
	b.rt.Validators = b.validators.Validators
	b.rt.Facets = b.validators.Facets
	b.rt.Patterns = b.validators.Patterns
	b.rt.Enums = b.validators.Enums
	b.rt.Values = b.validators.Values
	b.rt.Notations = b.notations
	b.wildcards = make([]runtime.WildcardRule, 1)

	b.initIDs()
	if err := b.buildTypes(); err != nil {
		return nil, err
	}
	if err := b.buildAncestors(); err != nil {
		return nil, err
	}
	if err := b.buildAttributes(); err != nil {
		return nil, err
	}
	if err := b.buildElements(); err != nil {
		return nil, err
	}
	if err := b.buildModels(); err != nil {
		return nil, err
	}
	if err := b.buildIdentityConstraints(); err != nil {
		return nil, err
	}

	b.rt.Wildcards = b.wildcards
	b.rt.WildcardNS = b.wildcardNS
	b.rt.Paths = b.paths

	b.rt.BuildHash = computeBuildHash(b.rt)

	return b.rt, nil
}

func (b *schemaBuilder) initSymbols() error {
	if b.builder == nil {
		return fmt.Errorf("runtime build: symbol builder missing")
	}
	xsdNS := types.XSDNamespace
	for _, name := range builtinTypeNames() {
		_ = b.internQName(types.QName{Namespace: xsdNS, Local: string(name)})
	}

	for _, entry := range b.registry.TypeOrder {
		if entry.QName.IsZero() {
			continue
		}
		_ = b.internQName(entry.QName)
	}
	for _, entry := range b.registry.ElementOrder {
		_ = b.internQName(entry.QName)
	}
	for _, entry := range b.registry.AttributeOrder {
		_ = b.internQName(entry.QName)
	}
	for _, entry := range b.registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		attrs, wildcard, err := collectAttributeUses(b.schema, ct)
		if err != nil {
			return err
		}
		for _, attr := range attrs {
			if attr == nil {
				continue
			}
			_ = b.internQName(effectiveAttributeQName(b.schema, attr))
		}
		if wildcard != nil {
			b.internNamespaceConstraint(wildcard.Namespace, wildcard.NamespaceList, wildcard.TargetNamespace)
		}
		particle := typegraph.EffectiveContentParticle(b.schema, ct)
		if particle != nil {
			b.internWildcardNamespaces(particle)
		}
	}
	for _, entry := range b.registry.ElementOrder {
		decl := entry.Decl
		if decl == nil {
			continue
		}
		for _, constraint := range decl.Constraints {
			qname := types.QName{Namespace: constraint.TargetNamespace, Local: constraint.Name}
			_ = b.internQName(qname)
		}
	}
	for _, qname := range schema.SortedQNames(b.schema.NotationDecls) {
		if qname.IsZero() {
			continue
		}
		if id := b.internQName(qname); id != 0 {
			b.notations = append(b.notations, id)
		}
	}
	if len(b.notations) > 1 {
		slices.Sort(b.notations)
		b.notations = slices.Compact(b.notations)
	}
	return nil
}

func (b *schemaBuilder) initIDs() {
	builtin := builtinTypeNames()
	totalTypes := len(builtin) + len(b.registry.TypeOrder)
	b.rt.Types = make([]runtime.Type, totalTypes+1)

	complexCount := 0
	for _, entry := range b.registry.TypeOrder {
		if _, ok := types.AsComplexType(entry.Type); ok {
			complexCount++
		}
	}
	complexCount++
	b.rt.ComplexTypes = make([]runtime.ComplexType, complexCount+1)
	b.anyTypeComplex = 1
	b.rt.Elements = make([]runtime.Element, len(b.registry.ElementOrder)+1)
	b.rt.Attributes = make([]runtime.Attribute, len(b.registry.AttributeOrder)+1)

	nextType := runtime.TypeID(1)
	for _, name := range builtin {
		b.builtinIDs[name] = nextType
		nextType++
	}
	for _, entry := range b.registry.TypeOrder {
		b.typeIDs[entry.ID] = nextType
		nextType++
	}

	nextElem := runtime.ElemID(1)
	for _, entry := range b.registry.ElementOrder {
		b.elemIDs[entry.ID] = nextElem
		nextElem++
	}

	nextAttr := runtime.AttrID(1)
	for _, entry := range b.registry.AttributeOrder {
		b.attrIDs[entry.ID] = nextAttr
		nextAttr++
	}

	b.rt.GlobalTypes = make([]runtime.TypeID, b.rt.Symbols.Count()+1)
	b.rt.GlobalElements = make([]runtime.ElemID, b.rt.Symbols.Count()+1)
	b.rt.GlobalAttributes = make([]runtime.AttrID, b.rt.Symbols.Count()+1)

	b.rt.Models = runtime.ModelsBundle{
		DFA: make([]runtime.DFAModel, 1),
		NFA: make([]runtime.NFAModel, 1),
		All: make([]runtime.AllModel, 1),
	}
	b.rt.AttrIndex = runtime.ComplexAttrIndex{
		Uses:       make([]runtime.AttrUse, 0),
		HashTables: nil,
	}
	b.paths = make([]runtime.PathProgram, 1)
	b.rt.ICs = make([]runtime.IdentityConstraint, 1)
}

func (b *schemaBuilder) buildAncestors() error {
	if b.rt == nil {
		return fmt.Errorf("runtime build: schema missing for ancestors")
	}
	typeCount := len(b.rt.Types)
	ids := make([]runtime.TypeID, 0, typeCount)
	masks := make([]runtime.DerivationMethod, 0, typeCount)

	for id := runtime.TypeID(1); int(id) < typeCount; id++ {
		typ := b.rt.Types[id]
		offset := uint32(len(ids))

		var err error
		ids, masks, err = b.appendAncestors(id, typ, ids, masks)
		if err != nil {
			return err
		}

		typ.AncOff = offset
		typ.AncLen = uint32(len(ids)) - offset
		typ.AncMaskOff = typ.AncOff
		b.rt.Types[id] = typ
	}

	b.rt.Ancestors = runtime.TypeAncestors{IDs: ids, Masks: masks}
	return nil
}

func (b *schemaBuilder) appendAncestors(id runtime.TypeID, typ runtime.Type, ids []runtime.TypeID, masks []runtime.DerivationMethod) ([]runtime.TypeID, []runtime.DerivationMethod, error) {
	if b == nil {
		return ids, masks, fmt.Errorf("runtime build: schema builder missing")
	}
	if id == 0 || b.rt == nil {
		return ids, masks, fmt.Errorf("runtime build: invalid type ID for ancestors")
	}
	typeCount := len(b.rt.Types)
	cumulative := runtime.DerivationMethod(0)
	base := typ.Base
	current := typ
	visited := make(map[runtime.TypeID]bool)

	for base != 0 {
		if current.Derivation == runtime.DerNone {
			return ids, masks, fmt.Errorf("runtime build: type %d missing derivation method", id)
		}
		if int(base) >= typeCount {
			return ids, masks, fmt.Errorf("runtime build: ancestor type %d out of range", base)
		}
		if visited[base] {
			return ids, masks, fmt.Errorf("runtime build: type derivation cycle at %d", base)
		}
		visited[base] = true

		cumulative |= current.Derivation
		ids = append(ids, base)
		masks = append(masks, cumulative)

		current = b.rt.Types[base]
		base = current.Base
	}

	return ids, masks, nil
}

func (b *schemaBuilder) buildTypes() error {
	xsdNS := types.XSDNamespace
	nextComplex := uint32(1)
	for _, name := range builtinTypeNames() {
		id := b.builtinIDs[name]
		sym := b.internQName(types.QName{Namespace: xsdNS, Local: string(name)})
		typ := runtime.Type{Name: sym}
		if builtin := types.GetBuiltin(name); builtin != nil {
			base := builtin.BaseType()
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: builtin type %s base %s not found", name, base.Name())
				}
				typ.Base = baseID
				typ.Derivation = runtime.DerRestriction
			}
		}
		if name == types.TypeNameAnyType {
			typ.Kind = runtime.TypeComplex
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.anyTypeComplex = nextComplex
			nextComplex++
		} else {
			typ.Kind = runtime.TypeBuiltin
			typ.Validator = b.validatorForBuiltin(name)
		}
		b.rt.Types[id] = typ
		b.rt.GlobalTypes[sym] = id
		if name == types.TypeNameAnyType {
			b.rt.Builtin.AnyType = id
		}
		if name == types.TypeNameAnySimpleType {
			b.rt.Builtin.AnySimpleType = id
		}
	}

	for _, entry := range b.registry.TypeOrder {
		id := b.typeIDs[entry.ID]
		var sym runtime.SymbolID
		if !entry.QName.IsZero() {
			sym = b.internQName(entry.QName)
		} else if entry.Global {
			return fmt.Errorf("runtime build: global type %d missing name", entry.ID)
		}
		typ := runtime.Type{Name: sym}
		switch t := entry.Type.(type) {
		case *types.SimpleType:
			typ.Kind = runtime.TypeSimple
			if vid, ok := b.validators.TypeValidators[entry.ID]; ok {
				typ.Validator = vid
			} else if vid, ok := b.validators.ValidatorForType(t); ok {
				typ.Validator = vid
			}
			base, method := b.baseForSimpleType(t)
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", entry.QName, base.Name())
				}
				typ.Base = baseID
				typ.Derivation = method
			}
			typ.Final = toRuntimeDerivationSet(t.Final)
		case *types.ComplexType:
			typ.Kind = runtime.TypeComplex
			if t.Abstract {
				typ.Flags |= runtime.TypeAbstract
			}
			base := t.BaseType()
			if base != nil {
				baseID, ok := b.runtimeTypeID(base)
				if !ok {
					return fmt.Errorf("runtime build: type %s base %s not found", entry.QName, base.Name())
				}
				typ.Base = baseID
			}
			method := t.DerivationMethod
			if method == 0 {
				method = types.DerivationRestriction
			}
			typ.Derivation = toRuntimeDerivation(method)
			typ.Final = toRuntimeDerivationSet(t.Final)
			typ.Block = toRuntimeDerivationSet(t.Block)
			typ.Complex = runtime.ComplexTypeRef{ID: nextComplex}
			b.complexIDs[id] = nextComplex
			nextComplex++
		default:
			return fmt.Errorf("runtime build: unsupported type %T", entry.Type)
		}

		b.rt.Types[id] = typ
		if entry.Global {
			b.rt.GlobalTypes[sym] = id
		}
	}
	return nil
}

func (b *schemaBuilder) buildAttributes() error {
	for _, entry := range b.registry.AttributeOrder {
		id := b.attrIDs[entry.ID]
		sym := b.internQName(entry.QName)
		attr := runtime.Attribute{Name: sym}
		if decl := entry.Decl; decl != nil {
			if decl.Type == nil {
				return fmt.Errorf("runtime build: attribute %s missing type", entry.QName)
			}
			vid, ok := b.validators.ValidatorForType(decl.Type)
			if !ok || vid == 0 {
				return fmt.Errorf("runtime build: attribute %s missing validator", entry.QName)
			}
			attr.Validator = vid
		}
		if def, ok := b.validators.AttributeDefaults[entry.ID]; ok {
			attr.Default = def
			if key, ok := b.validators.AttributeDefaultKeys[entry.ID]; ok {
				attr.DefaultKey = key
			}
			if member, ok := b.validators.AttributeDefaultMembers[entry.ID]; ok {
				attr.DefaultMember = member
			}
		}
		if fixed, ok := b.validators.AttributeFixed[entry.ID]; ok {
			attr.Fixed = fixed
			if key, ok := b.validators.AttributeFixedKeys[entry.ID]; ok {
				attr.FixedKey = key
			}
			if member, ok := b.validators.AttributeFixedMembers[entry.ID]; ok {
				attr.FixedMember = member
			}
		}
		b.rt.Attributes[id] = attr
		if entry.Global {
			b.rt.GlobalAttributes[sym] = id
		}
	}

	for _, entry := range b.registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		typeID := b.typeIDs[entry.ID]
		complexID := b.complexIDs[typeID]
		if complexID == 0 {
			continue
		}

		uses, wildcard, err := b.collectAttrUses(ct)
		if err != nil {
			return err
		}
		if wildcard != nil {
			b.rt.ComplexTypes[complexID].AnyAttr = b.addWildcardAnyAttribute(wildcard)
		}
		if len(uses) == 0 {
			continue
		}
		off := uint32(len(b.rt.AttrIndex.Uses))
		var mode runtime.AttrIndexMode
		hashTable := uint32(0)
		switch {
		case len(uses) <= attrIndexLinearLimit:
			mode = runtime.AttrIndexSmallLinear
		case len(uses) <= attrIndexBinaryLimit:
			slices.SortFunc(uses, func(a, b runtime.AttrUse) int {
				return cmp.Compare(a.Name, b.Name)
			})
			mode = runtime.AttrIndexSortedBinary
		default:
			mode = runtime.AttrIndexHash
			table := buildAttrHashTable(uses, off)
			hashTable = uint32(len(b.rt.AttrIndex.HashTables))
			b.rt.AttrIndex.HashTables = append(b.rt.AttrIndex.HashTables, table)
		}
		b.rt.AttrIndex.Uses = append(b.rt.AttrIndex.Uses, uses...)
		ref := runtime.AttrIndexRef{
			Off:       off,
			Len:       uint32(len(uses)),
			Mode:      mode,
			HashTable: hashTable,
		}
		b.rt.ComplexTypes[complexID].Attrs = ref
	}
	return nil
}

func (b *schemaBuilder) buildElements() error {
	for _, entry := range b.registry.ElementOrder {
		id := b.elemIDs[entry.ID]
		decl := entry.Decl
		if decl == nil {
			return fmt.Errorf("runtime build: element %s is nil", entry.QName)
		}
		sym := b.internQName(entry.QName)
		elem := runtime.Element{Name: sym}
		if decl.Type == nil {
			return fmt.Errorf("runtime build: element %s missing type", entry.QName)
		}
		typeID, ok := b.runtimeTypeID(decl.Type)
		if !ok {
			return fmt.Errorf("runtime build: element %s missing type ID", entry.QName)
		}
		elem.Type = typeID
		if !decl.SubstitutionGroup.IsZero() {
			if head := b.schema.ElementDecls[decl.SubstitutionGroup]; head != nil {
				if headID, ok := b.runtimeElemID(head); ok {
					elem.SubstHead = headID
				}
			}
		}
		if decl.Nillable {
			elem.Flags |= runtime.ElemNillable
		}
		if decl.Abstract {
			elem.Flags |= runtime.ElemAbstract
		}
		elem.Block = toRuntimeElemBlock(decl.Block)
		elem.Final = toRuntimeDerivationSet(decl.Final)

		if def, ok := b.validators.ElementDefaults[entry.ID]; ok {
			elem.Default = def
			if key, ok := b.validators.ElementDefaultKeys[entry.ID]; ok {
				elem.DefaultKey = key
			}
			if member, ok := b.validators.ElementDefaultMembers[entry.ID]; ok {
				elem.DefaultMember = member
			}
		}
		if fixed, ok := b.validators.ElementFixed[entry.ID]; ok {
			elem.Fixed = fixed
			if key, ok := b.validators.ElementFixedKeys[entry.ID]; ok {
				elem.FixedKey = key
			}
			if member, ok := b.validators.ElementFixedMembers[entry.ID]; ok {
				elem.FixedMember = member
			}
		}

		b.rt.Elements[id] = elem
		if entry.Global {
			b.rt.GlobalElements[sym] = id
		}
	}
	return nil
}

func (b *schemaBuilder) buildModels() error {
	if err := b.buildAnyTypeModel(); err != nil {
		return err
	}
	for _, entry := range b.registry.TypeOrder {
		ct, ok := types.AsComplexType(entry.Type)
		if !ok || ct == nil {
			continue
		}
		typeID := b.typeIDs[entry.ID]
		complexID := b.complexIDs[typeID]
		if complexID == 0 {
			continue
		}

		content := ct.Content()
		model := &b.rt.ComplexTypes[complexID]
		model.Mixed = ct.EffectiveMixed()
		switch content.(type) {
		case *types.SimpleContent:
			model.Content = runtime.ContentSimple
			var textType types.Type
			if b.validators != nil && b.validators.SimpleContentTypes != nil {
				textType = b.validators.SimpleContentTypes[ct]
			}
			if textType == nil {
				var err error
				textType, err = b.simpleContentTextType(ct)
				if err != nil {
					return err
				}
			}
			if textType == nil {
				return fmt.Errorf("runtime build: complex type %s simpleContent base missing", entry.QName)
			}
			vid, ok := b.validators.ValidatorForType(textType)
			if !ok || vid == 0 {
				return fmt.Errorf("runtime build: complex type %s missing validator", entry.QName)
			}
			model.TextValidator = vid
		case *types.EmptyContent:
			model.Content = runtime.ContentEmpty
		default:
			particle := typegraph.EffectiveContentParticle(b.schema, ct)
			if particle == nil {
				model.Content = runtime.ContentEmpty
				break
			}
			ref, kind, err := b.compileParticleModel(particle)
			if err != nil {
				return err
			}
			model.Content = kind
			model.Model = ref
		}

	}
	return nil
}

func (b *schemaBuilder) buildAnyTypeModel() error {
	if b.anyTypeComplex == 0 || int(b.anyTypeComplex) >= len(b.rt.ComplexTypes) {
		return nil
	}
	model := &b.rt.ComplexTypes[b.anyTypeComplex]
	model.Mixed = true

	anyElem := &types.AnyElement{
		Namespace:       types.NSCAny,
		ProcessContents: types.Lax,
		MinOccurs:       types.OccursFromInt(0),
		MaxOccurs:       types.OccursUnbounded,
	}
	ref, kind, err := b.compileParticleModel(anyElem)
	if err != nil {
		return err
	}
	model.Content = kind
	model.Model = ref

	anyAttr := &types.AnyAttribute{
		Namespace:       types.NSCAny,
		ProcessContents: types.Lax,
		TargetNamespace: types.NamespaceEmpty,
	}
	model.AnyAttr = b.addWildcardAnyAttribute(anyAttr)
	return nil
}

func (b *schemaBuilder) compileParticleModel(particle types.Particle) (runtime.ModelRef, runtime.ContentKind, error) {
	if particle == nil {
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
	resolved, err := schemaops.ExpandGroupRefs(particle, b.groupRefExpansionOptions())
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	particle = resolved
	if isEmptyChoice(particle) {
		return b.addRejectAllModel(), runtime.ContentElementOnly, nil
	}
	err = b.validateOccursLimit(particle)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	if group, ok := particle.(*types.ModelGroup); ok && group.Kind == types.AllGroup {
		ref, addErr := b.addAllModel(group)
		if addErr != nil {
			return runtime.ModelRef{}, 0, addErr
		}
		return ref, runtime.ContentAll, nil
	}

	glu, err := models.BuildGlushkov(particle)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	glu, err = models.ExpandSubstitution(glu, b.resolveSubstitutionHead, b.substitutionMembers)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	matchers, err := b.buildMatchers(glu)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	compiled, err := models.Compile(glu, matchers, b.limits)
	if err != nil {
		return runtime.ModelRef{}, 0, err
	}
	switch compiled.Kind {
	case runtime.ModelDFA:
		id := uint32(len(b.rt.Models.DFA))
		b.rt.Models.DFA = append(b.rt.Models.DFA, compiled.DFA)
		return runtime.ModelRef{Kind: runtime.ModelDFA, ID: id}, runtime.ContentElementOnly, nil
	case runtime.ModelNFA:
		id := uint32(len(b.rt.Models.NFA))
		b.rt.Models.NFA = append(b.rt.Models.NFA, compiled.NFA)
		return runtime.ModelRef{Kind: runtime.ModelNFA, ID: id}, runtime.ContentElementOnly, nil
	default:
		return runtime.ModelRef{Kind: runtime.ModelNone}, runtime.ContentEmpty, nil
	}
}

func (b *schemaBuilder) groupRefExpansionOptions() schemaops.ExpandGroupRefsOptions {
	return schemaops.ExpandGroupRefsOptions{
		Lookup: func(ref *types.GroupRef) *types.ModelGroup {
			if ref == nil {
				return nil
			}
			if b != nil && b.refs != nil {
				if group := b.refs.GroupRefs[ref]; group != nil {
					return group
				}
			}
			if b == nil || b.schema == nil {
				return nil
			}
			return b.schema.Groups[ref.RefQName]
		},
		MissingError: func(ref types.QName) error {
			return fmt.Errorf("group ref %s not resolved", ref)
		},
		CycleError: func(ref types.QName) error {
			return fmt.Errorf("group ref cycle detected: %s", ref)
		},
		AllGroupMode: schemaops.AllGroupKeep,
		LeafClone:    schemaops.LeafReuse,
	}
}

func isEmptyChoice(particle types.Particle) bool {
	group, ok := particle.(*types.ModelGroup)
	if !ok || group == nil || group.Kind != types.Choice {
		return false
	}
	for _, child := range group.Particles {
		if child == nil {
			continue
		}
		if child.MaxOcc().IsZero() {
			continue
		}
		return false
	}
	return true
}

func (b *schemaBuilder) validateOccursLimit(particle types.Particle) error {
	if particle == nil || b.maxOccurs == 0 {
		return nil
	}
	if err := b.checkOccursValue("minOccurs", particle.MinOcc()); err != nil {
		return err
	}
	if err := b.checkOccursValue("maxOccurs", particle.MaxOcc()); err != nil {
		return err
	}
	if group, ok := particle.(*types.ModelGroup); ok {
		for _, child := range group.Particles {
			if err := b.validateOccursLimit(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *schemaBuilder) checkOccursValue(attr string, occ types.Occurs) error {
	if b == nil || b.maxOccurs == 0 {
		return nil
	}
	if occ.IsUnbounded() {
		return nil
	}
	if occ.IsOverflow() {
		return fmt.Errorf("%w: %s value %s exceeds uint32", types.ErrOccursOverflow, attr, occ.String())
	}
	if occ.GreaterThanInt(int(b.maxOccurs)) {
		return fmt.Errorf("%w: %s value %s exceeds limit %d", types.ErrOccursTooLarge, attr, occ.String(), b.maxOccurs)
	}
	return nil
}

func (b *schemaBuilder) addAllModel(group *types.ModelGroup) (runtime.ModelRef, error) {
	if group == nil {
		return runtime.ModelRef{Kind: runtime.ModelNone}, nil
	}
	minOccurs, ok := group.MinOccurs.Int()
	if !ok {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group minOccurs too large")
	}
	if group.MaxOccurs.IsUnbounded() {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs unbounded")
	}
	if maxOccurs, ok := group.MaxOccurs.Int(); !ok || maxOccurs > 1 {
		return runtime.ModelRef{}, fmt.Errorf("runtime build: all group maxOccurs must be <= 1")
	}

	model := runtime.AllModel{
		MinOccurs: uint32(minOccurs),
		Mixed:     false,
	}
	for _, particle := range group.Particles {
		elem, ok := particle.(*types.ElementDecl)
		if !ok || elem == nil {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group member must be element")
		}
		elemID, ok := b.runtimeElemID(elem)
		if !ok {
			return runtime.ModelRef{}, fmt.Errorf("runtime build: all group element %s missing ID", elem.Name)
		}
		minOccurs := elem.MinOcc()
		optional := minOccurs.IsZero()
		member := runtime.AllMember{
			Elem:     elemID,
			Optional: optional,
		}
		if elem.IsReference {
			member.AllowsSubst = true
			head := elem
			if resolved := b.resolveSubstitutionHead(elem); resolved != nil {
				head = resolved
			}
			list, err := models.ExpandSubstitutionMembers(head, b.substitutionMembers)
			if err != nil {
				return runtime.ModelRef{}, err
			}
			if len(list) > 0 {
				member.SubstOff = uint32(len(b.rt.Models.AllSubst))
				for _, decl := range list {
					if decl == nil {
						continue
					}
					memberID, ok := b.runtimeElemID(decl)
					if !ok {
						return runtime.ModelRef{}, fmt.Errorf("runtime build: all group substitution element %s missing ID", decl.Name)
					}
					b.rt.Models.AllSubst = append(b.rt.Models.AllSubst, memberID)
				}
				member.SubstLen = uint32(len(b.rt.Models.AllSubst)) - member.SubstOff
			}
		}
		model.Members = append(model.Members, member)
	}

	id := uint32(len(b.rt.Models.All))
	b.rt.Models.All = append(b.rt.Models.All, model)
	return runtime.ModelRef{Kind: runtime.ModelAll, ID: id}, nil
}

func (b *schemaBuilder) addRejectAllModel() runtime.ModelRef {
	id := uint32(len(b.rt.Models.NFA))
	b.rt.Models.NFA = append(b.rt.Models.NFA, runtime.NFAModel{
		Nullable:  false,
		Start:     runtime.BitsetRef{},
		Accept:    runtime.BitsetRef{},
		FollowOff: 0,
		FollowLen: 0,
	})
	return runtime.ModelRef{Kind: runtime.ModelNFA, ID: id}
}

func (b *schemaBuilder) buildMatchers(glu *models.Glushkov) ([]runtime.PosMatcher, error) {
	if glu == nil {
		return nil, fmt.Errorf("runtime build: glushkov model missing")
	}
	matchers := make([]runtime.PosMatcher, len(glu.Positions))
	for i, pos := range glu.Positions {
		switch pos.Kind {
		case models.PositionElement:
			if pos.Element == nil {
				return nil, fmt.Errorf("runtime build: position %d missing element", i)
			}
			elemID, ok := b.runtimeElemID(pos.Element)
			if !ok {
				return nil, fmt.Errorf("runtime build: element %s missing ID", pos.Element.Name)
			}
			sym := b.internQName(pos.Element.Name)
			matchers[i] = runtime.PosMatcher{
				Kind: runtime.PosExact,
				Sym:  sym,
				Elem: elemID,
			}
		case models.PositionWildcard:
			if pos.Wildcard == nil {
				return nil, fmt.Errorf("runtime build: position %d missing wildcard", i)
			}
			rule := b.addWildcardAnyElement(pos.Wildcard)
			matchers[i] = runtime.PosMatcher{
				Kind: runtime.PosWildcard,
				Rule: rule,
			}
		default:
			return nil, fmt.Errorf("runtime build: unknown position kind %d", pos.Kind)
		}
	}
	return matchers, nil
}

func (b *schemaBuilder) collectAttrUses(ct *types.ComplexType) ([]runtime.AttrUse, *types.AnyAttribute, error) {
	if ct == nil {
		return nil, nil, nil
	}
	attrs, wildcard, err := collectAttributeUses(b.schema, ct)
	if err != nil {
		return nil, nil, err
	}
	if len(attrs) == 0 {
		return nil, wildcard, nil
	}
	out := make([]runtime.AttrUse, 0, len(attrs))
	for _, decl := range attrs {
		if decl == nil {
			continue
		}
		target := decl
		if decl.IsReference {
			target = b.resolveAttributeDecl(decl)
			if target == nil {
				return nil, nil, fmt.Errorf("runtime build: attribute ref %s not found", decl.Name)
			}
		}
		sym := b.internQName(effectiveAttributeQName(b.schema, decl))
		use := runtime.AttrUse{
			Name: sym,
			Use:  toRuntimeAttrUse(decl.Use),
		}
		if target.Type == nil {
			return nil, nil, fmt.Errorf("runtime build: attribute %s missing type", target.Name)
		}
		vid, ok := b.validators.ValidatorForType(target.Type)
		if !ok || vid == 0 {
			return nil, nil, fmt.Errorf("runtime build: attribute %s missing validator", target.Name)
		}
		use.Validator = vid
		if decl.HasDefault {
			if def, ok := b.validators.AttrUseDefaults[decl]; ok {
				use.Default = def
				if key, ok := b.validators.AttrUseDefaultKeys[decl]; ok {
					use.DefaultKey = key
				}
				if member, ok := b.validators.AttrUseDefaultMembers[decl]; ok {
					use.DefaultMember = member
				}
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s default missing", decl.Name)
			}
		}
		if decl.HasFixed {
			if fixed, ok := b.validators.AttrUseFixed[decl]; ok {
				use.Fixed = fixed
				if key, ok := b.validators.AttrUseFixedKeys[decl]; ok {
					use.FixedKey = key
				}
				if member, ok := b.validators.AttrUseFixedMembers[decl]; ok {
					use.FixedMember = member
				}
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s fixed missing", decl.Name)
			}
		}
		if !use.Default.Present && !use.Fixed.Present {
			if attrID, ok := b.schemaAttrID(target); ok {
				if def, ok := b.validators.AttributeDefaults[attrID]; ok {
					use.Default = def
					if key, ok := b.validators.AttributeDefaultKeys[attrID]; ok {
						use.DefaultKey = key
					}
					if member, ok := b.validators.AttributeDefaultMembers[attrID]; ok {
						use.DefaultMember = member
					}
				}
				if fixed, ok := b.validators.AttributeFixed[attrID]; ok {
					use.Fixed = fixed
					if key, ok := b.validators.AttributeFixedKeys[attrID]; ok {
						use.FixedKey = key
					}
					if member, ok := b.validators.AttributeFixedMembers[attrID]; ok {
						use.FixedMember = member
					}
				}
			}
		}
		out = append(out, use)
	}
	return out, wildcard, nil
}

const (
	attrIndexLinearLimit = 8
	attrIndexBinaryLimit = 64
)

func buildAttrHashTable(uses []runtime.AttrUse, off uint32) runtime.AttrHashTable {
	size := max(runtime.NextPow2(len(uses)*2), 1)
	table := runtime.AttrHashTable{
		Hash: make([]uint64, size),
		Slot: make([]uint32, size),
	}
	mask := uint64(size - 1)
	for i := range uses {
		use := &uses[i]
		h := uint64(use.Name)
		if h == 0 {
			h = 1
		}
		slot := int(h & mask)
		for {
			if table.Slot[slot] == 0 {
				table.Hash[slot] = h
				table.Slot[slot] = off + uint32(i) + 1
				break
			}
			slot = (slot + 1) & int(mask)
		}
	}
	return table
}

func (b *schemaBuilder) resolveAttributeDecl(decl *types.AttributeDecl) *types.AttributeDecl {
	if decl == nil {
		return nil
	}
	if !decl.IsReference {
		return decl
	}
	return b.schema.AttributeDecls[decl.Name]
}

func (b *schemaBuilder) schemaAttrID(decl *types.AttributeDecl) (schema.AttrID, bool) {
	if decl == nil {
		return 0, false
	}
	if decl.IsReference {
		if id, ok := b.refs.AttributeRefs[decl]; ok {
			return id, true
		}
		return 0, false
	}
	if id, ok := b.registry.Attributes[decl.Name]; ok {
		return id, true
	}
	if id, ok := b.registry.LocalAttributes[decl]; ok {
		return id, true
	}
	return 0, false
}

func (b *schemaBuilder) addWildcardAnyElement(anyElem *types.AnyElement) runtime.WildcardID {
	if anyElem == nil {
		return 0
	}
	if b.anyElementRules == nil {
		b.anyElementRules = make(map[*types.AnyElement]runtime.WildcardID)
	}
	if id, ok := b.anyElementRules[anyElem]; ok {
		return id
	}
	id := b.addWildcard(anyElem.Namespace, anyElem.NamespaceList, anyElem.TargetNamespace, anyElem.ProcessContents)
	b.anyElementRules[anyElem] = id
	return id
}

func (b *schemaBuilder) addWildcardAnyAttribute(anyAttr *types.AnyAttribute) runtime.WildcardID {
	if anyAttr == nil {
		return 0
	}
	return b.addWildcard(anyAttr.Namespace, anyAttr.NamespaceList, anyAttr.TargetNamespace, anyAttr.ProcessContents)
}

func (b *schemaBuilder) addWildcard(constraint types.NamespaceConstraint, list []types.NamespaceURI, target types.NamespaceURI, pc types.ProcessContents) runtime.WildcardID {
	rule := runtime.WildcardRule{
		PC:       toRuntimeProcessContents(pc),
		TargetNS: b.internNamespace(target),
	}

	off := len(b.wildcardNS)
	switch constraint {
	case types.NSCAny:
		rule.NS.Kind = runtime.NSAny
	case types.NSCOther:
		rule.NS.Kind = runtime.NSOther
		rule.NS.HasTarget = true
	case types.NSCTargetNamespace:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasTarget = true
	case types.NSCLocal:
		rule.NS.Kind = runtime.NSEnumeration
		rule.NS.HasLocal = true
	case types.NSCNotAbsent:
		rule.NS.Kind = runtime.NSNotAbsent
	case types.NSCList:
		rule.NS.Kind = runtime.NSEnumeration
		for _, ns := range list {
			if ns == types.NamespaceTargetPlaceholder {
				rule.NS.HasTarget = true
				continue
			}
			if ns.IsEmpty() {
				rule.NS.HasLocal = true
				continue
			}
			b.wildcardNS = append(b.wildcardNS, b.internNamespace(ns))
		}
	default:
		rule.NS.Kind = runtime.NSAny
	}

	if rule.NS.Kind == runtime.NSEnumeration {
		ln := len(b.wildcardNS) - off
		if ln > 0 {
			rule.NS.Off = uint32(off)
			rule.NS.Len = uint32(ln)
		}
	}

	b.wildcards = append(b.wildcards, rule)
	return runtime.WildcardID(len(b.wildcards) - 1)
}

func (b *schemaBuilder) substitutionMembers(head *types.ElementDecl) []*types.ElementDecl {
	if head == nil {
		return nil
	}
	queue := []types.QName{head.Name}
	seen := make(map[types.QName]bool)
	seen[head.Name] = true
	out := make([]*types.ElementDecl, 0)

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, memberName := range b.schema.SubstitutionGroups[name] {
			if seen[memberName] {
				continue
			}
			seen[memberName] = true
			decl := b.schema.ElementDecls[memberName]
			if decl == nil {
				continue
			}
			out = append(out, decl)
			queue = append(queue, memberName)
		}
	}
	return out
}

func (b *schemaBuilder) resolveSubstitutionHead(decl *types.ElementDecl) *types.ElementDecl {
	if decl == nil || !decl.IsReference || b == nil || b.schema == nil {
		return decl
	}
	if head := b.schema.ElementDecls[decl.Name]; head != nil {
		return head
	}
	return decl
}

func (b *schemaBuilder) runtimeTypeID(typ types.Type) (runtime.TypeID, bool) {
	if typ == nil {
		return 0, false
	}
	if bt, ok := types.AsBuiltinType(typ); ok {
		return b.builtinIDs[types.TypeName(bt.Name().Local)], true
	}
	if st, ok := types.AsSimpleType(typ); ok && st.IsBuiltin() {
		if builtin := types.GetBuiltin(types.TypeName(st.Name().Local)); builtin != nil {
			return b.builtinIDs[types.TypeName(builtin.Name().Local)], true
		}
	}
	if !typ.Name().IsZero() {
		if id, ok := b.registry.Types[typ.Name()]; ok {
			return b.typeIDs[id], true
		}
	}
	if id, ok := b.registry.AnonymousTypes[typ]; ok {
		return b.typeIDs[id], true
	}
	return 0, false
}

func (b *schemaBuilder) runtimeElemID(decl *types.ElementDecl) (runtime.ElemID, bool) {
	if decl == nil {
		return 0, false
	}
	if decl.IsReference {
		if id, ok := b.refs.ElementRefs[decl]; ok {
			return b.elemIDs[id], true
		}
		return 0, false
	}
	if id, ok := b.registry.LocalElements[decl]; ok {
		return b.elemIDs[id], true
	}
	if id, ok := b.registry.Elements[decl.Name]; ok {
		return b.elemIDs[id], true
	}
	return 0, false
}

func (b *schemaBuilder) resolveTypeQName(qname types.QName) types.Type {
	if qname.IsZero() {
		return nil
	}
	if qname.Namespace == types.XSDNamespace {
		return types.GetBuiltin(types.TypeName(qname.Local))
	}
	return b.schema.TypeDefs[qname]
}

func (b *schemaBuilder) simpleContentTextType(ct *types.ComplexType) (types.Type, error) {
	res := newTypeResolver(b.schema)
	return schemaops.ResolveSimpleContentTextType(ct, schemaops.SimpleContentTextTypeOptions{
		ResolveQName: res.resolveQName,
	})
}

func (b *schemaBuilder) baseForSimpleType(st *types.SimpleType) (types.Type, runtime.DerivationMethod) {
	if st == nil {
		return nil, runtime.DerNone
	}
	if st.List != nil {
		return types.GetBuiltin(types.TypeNameAnySimpleType), runtime.DerList
	}
	if st.Union != nil {
		return types.GetBuiltin(types.TypeNameAnySimpleType), runtime.DerUnion
	}
	if st.Restriction != nil {
		if st.Restriction.SimpleType != nil {
			return st.Restriction.SimpleType, runtime.DerRestriction
		}
		if !st.Restriction.Base.IsZero() {
			return b.resolveTypeQName(st.Restriction.Base), runtime.DerRestriction
		}
	}
	if st.ResolvedBase != nil {
		return st.ResolvedBase, runtime.DerRestriction
	}
	return types.GetBuiltin(types.TypeNameAnySimpleType), runtime.DerRestriction
}

func (b *schemaBuilder) validatorForBuiltin(name types.TypeName) runtime.ValidatorID {
	if b.validators == nil {
		return 0
	}
	bt := types.GetBuiltin(name)
	if bt == nil {
		return 0
	}
	if id, ok := b.validators.ValidatorForType(bt); ok {
		return id
	}
	return 0
}

func builtinTypeNames() []types.TypeName {
	return []types.TypeName{
		types.TypeNameAnyType,
		types.TypeNameAnySimpleType,
		types.TypeNameString,
		types.TypeNameBoolean,
		types.TypeNameDecimal,
		types.TypeNameFloat,
		types.TypeNameDouble,
		types.TypeNameDuration,
		types.TypeNameDateTime,
		types.TypeNameTime,
		types.TypeNameDate,
		types.TypeNameGYearMonth,
		types.TypeNameGYear,
		types.TypeNameGMonthDay,
		types.TypeNameGDay,
		types.TypeNameGMonth,
		types.TypeNameHexBinary,
		types.TypeNameBase64Binary,
		types.TypeNameAnyURI,
		types.TypeNameQName,
		types.TypeNameNOTATION,
		types.TypeNameNormalizedString,
		types.TypeNameToken,
		types.TypeNameLanguage,
		types.TypeNameName,
		types.TypeNameNCName,
		types.TypeNameID,
		types.TypeNameIDREF,
		types.TypeNameIDREFS,
		types.TypeNameENTITY,
		types.TypeNameENTITIES,
		types.TypeNameNMTOKEN,
		types.TypeNameNMTOKENS,
		types.TypeNameInteger,
		types.TypeNameLong,
		types.TypeNameInt,
		types.TypeNameShort,
		types.TypeNameByte,
		types.TypeNameNonNegativeInteger,
		types.TypeNamePositiveInteger,
		types.TypeNameUnsignedLong,
		types.TypeNameUnsignedInt,
		types.TypeNameUnsignedShort,
		types.TypeNameUnsignedByte,
		types.TypeNameNonPositiveInteger,
		types.TypeNameNegativeInteger,
	}
}
