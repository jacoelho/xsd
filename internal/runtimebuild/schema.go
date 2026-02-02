package runtimebuild

import (
	"fmt"
	"sort"

	"github.com/jacoelho/xsd/internal/models"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xpath"
)

// BuildConfig configures runtime schema compilation.
type BuildConfig struct {
	Limits         models.Limits
	MaxOccursLimit uint32
}

// BuildSchema compiles a parsed schema into a runtime schema model.
func BuildSchema(sch *parser.Schema, cfg BuildConfig) (*runtime.Schema, error) {
	if sch == nil {
		return nil, fmt.Errorf("runtime build: schema is nil")
	}
	reg, err := schema.AssignIDs(sch)
	if err != nil {
		return nil, fmt.Errorf("runtime build: assign IDs: %w", err)
	}
	refs, err := schema.ResolveReferences(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("runtime build: resolve references: %w", err)
	}
	err = schema.DetectCycles(sch)
	if err != nil {
		return nil, fmt.Errorf("runtime build: detect cycles: %w", err)
	}
	if !sch.UPAValidated {
		err = schema.ValidateUPA(sch, reg)
		if err != nil {
			return nil, fmt.Errorf("runtime build: validate UPA: %w", err)
		}
	}
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
	return builder.build()
}

type schemaBuilder struct {
	complexIDs      map[runtime.TypeID]uint32
	rt              *runtime.Schema
	refs            *schema.ResolvedReferences
	validators      *CompiledValidators
	anyElementRules map[*types.AnyElement]runtime.WildcardID
	typeIDs         map[schema.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	registry        *schema.Registry
	attrIDs         map[schema.AttrID]runtime.AttrID
	elemIDs         map[schema.ElemID]runtime.ElemID
	builtinIDs      map[types.TypeName]runtime.TypeID
	wildcardNS      []runtime.NamespaceID
	paths           []runtime.PathProgram
	wildcards       []runtime.WildcardRule
	maxOccurs       uint32
	anyTypeComplex  uint32
	limits          models.Limits
}

const defaultMaxOccursLimit = 1_000_000

func (b *schemaBuilder) build() (*runtime.Schema, error) {
	if err := b.initSymbols(); err != nil {
		return nil, err
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
		particle := schemacheck.EffectiveContentParticle(b.schema, ct)
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
	return nil
}

func (b *schemaBuilder) internNamespaceConstraint(constraint types.NamespaceConstraint, list []types.NamespaceURI, target types.NamespaceURI) {
	if b == nil {
		return
	}
	switch constraint {
	case types.NSCTargetNamespace, types.NSCOther:
		_ = b.internNamespace(target)
	case types.NSCList:
		for _, ns := range list {
			if ns == types.NamespaceTargetPlaceholder {
				_ = b.internNamespace(target)
				continue
			}
			if ns.IsEmpty() {
				continue
			}
			_ = b.internNamespace(ns)
		}
	}
}

func (b *schemaBuilder) internWildcardNamespaces(particle types.Particle) {
	if particle == nil || b == nil {
		return
	}
	visited := make(map[*types.ModelGroup]bool)
	b.internWildcardNamespacesInParticle(particle, visited)
}

func (b *schemaBuilder) internWildcardNamespacesInParticle(particle types.Particle, visited map[*types.ModelGroup]bool) {
	if particle == nil {
		return
	}
	switch typed := particle.(type) {
	case *types.AnyElement:
		b.internNamespaceConstraint(typed.Namespace, typed.NamespaceList, typed.TargetNamespace)
	case *types.ModelGroup:
		if visited[typed] {
			return
		}
		visited[typed] = true
		for _, child := range typed.Particles {
			b.internWildcardNamespacesInParticle(child, visited)
		}
	case *types.GroupRef:
		if b.schema == nil {
			return
		}
		group := b.schema.Groups[typed.RefQName]
		if group == nil {
			return
		}
		b.internWildcardNamespacesInParticle(group, visited)
	}
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
			if vid, ok := b.validators.ValidatorForType(decl.Type); ok {
				attr.Validator = vid
			}
		}
		if def, ok := b.validators.AttributeDefaults[entry.ID]; ok {
			attr.Default = def
		}
		if fixed, ok := b.validators.AttributeFixed[entry.ID]; ok {
			attr.Fixed = fixed
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
			sort.Slice(uses, func(i, j int) bool { return uses[i].Name < uses[j].Name })
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
		if typeID, ok := b.runtimeTypeID(decl.Type); ok {
			elem.Type = typeID
		}
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
		}
		if fixed, ok := b.validators.ElementFixed[entry.ID]; ok {
			elem.Fixed = fixed
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
		switch typed := content.(type) {
		case *types.SimpleContent:
			model.Content = runtime.ContentSimple
			base := b.resolveTypeQName(typed.BaseTypeQName())
			if base == nil {
				base = types.GetBuiltin(types.TypeNameAnySimpleType)
			}
			if vid, ok := b.validators.ValidatorForType(base); ok {
				model.TextValidator = vid
			}
		case *types.EmptyContent:
			model.Content = runtime.ContentEmpty
		default:
			particle := schemacheck.EffectiveContentParticle(b.schema, ct)
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
	resolved, err := b.resolveGroupRefs(particle, make(map[types.QName]bool))
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

func (b *schemaBuilder) resolveGroupRefs(particle types.Particle, stack map[types.QName]bool) (types.Particle, error) {
	switch typed := particle.(type) {
	case *types.GroupRef:
		if typed == nil {
			return nil, nil
		}
		if stack[typed.RefQName] {
			return nil, fmt.Errorf("group ref cycle detected: %s", typed.RefQName)
		}
		stack[typed.RefQName] = true
		defer delete(stack, typed.RefQName)

		group := b.refs.GroupRefs[typed]
		if group == nil && b.schema != nil {
			group = b.schema.Groups[typed.RefQName]
		}
		if group == nil {
			return nil, fmt.Errorf("group ref %s not resolved", typed.RefQName)
		}

		clone := *group
		clone.MinOccurs = typed.MinOccurs
		clone.MaxOccurs = typed.MaxOccurs
		if len(group.Particles) > 0 {
			clone.Particles = make([]types.Particle, len(group.Particles))
			for i, child := range group.Particles {
				resolved, err := b.resolveGroupRefs(child, stack)
				if err != nil {
					return nil, err
				}
				clone.Particles[i] = resolved
			}
		}
		return &clone, nil
	case *types.ModelGroup:
		if typed == nil {
			return nil, nil
		}
		if len(typed.Particles) == 0 {
			return typed, nil
		}
		updated := false
		particles := make([]types.Particle, len(typed.Particles))
		for i, child := range typed.Particles {
			resolved, err := b.resolveGroupRefs(child, stack)
			if err != nil {
				return nil, err
			}
			if resolved != child {
				updated = true
			}
			particles[i] = resolved
		}
		if !updated {
			return typed, nil
		}
		clone := *typed
		clone.Particles = particles
		return &clone, nil
	default:
		return particle, nil
	}
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

func (b *schemaBuilder) buildIdentityConstraints() error {
	b.rt.ICSelectors = nil
	b.rt.ICFields = nil
	b.rt.ElemICs = nil

	icByElem := make(map[runtime.ElemID]map[types.QName]runtime.ICID)
	type keyrefPending struct {
		name types.QName
		elem runtime.ElemID
		id   runtime.ICID
	}
	var pending []keyrefPending

	for _, entry := range b.registry.ElementOrder {
		decl := entry.Decl
		if decl == nil || len(decl.Constraints) == 0 {
			continue
		}
		elemID := b.elemIDs[entry.ID]
		elem := b.rt.Elements[elemID]
		off := uint32(len(b.rt.ElemICs))

		for _, constraint := range decl.Constraints {
			icID := runtime.ICID(len(b.rt.ICs))
			selectorOff := uint32(len(b.rt.ICSelectors))
			selectorPrograms, err := xpath.CompilePrograms(constraint.Selector.XPath, constraint.NamespaceContext, xpath.AttributesDisallowed, b.rt)
			if err != nil {
				return fmt.Errorf("runtime build: selector %s: %w", constraint.Name, err)
			}
			for _, program := range selectorPrograms {
				pathID := b.addPath(program)
				b.rt.ICSelectors = append(b.rt.ICSelectors, pathID)
			}
			selectorLen := uint32(len(b.rt.ICSelectors)) - selectorOff

			fieldOff := uint32(len(b.rt.ICFields))
			for fieldIdx, field := range constraint.Fields {
				fieldPrograms, err := xpath.CompilePrograms(field.XPath, constraint.NamespaceContext, xpath.AttributesAllowed, b.rt)
				if err != nil {
					return fmt.Errorf("runtime build: field %d %s: %w", fieldIdx+1, constraint.Name, err)
				}
				for _, program := range fieldPrograms {
					pathID := b.addPath(program)
					b.rt.ICFields = append(b.rt.ICFields, pathID)
				}
				if fieldIdx < len(constraint.Fields)-1 {
					b.rt.ICFields = append(b.rt.ICFields, 0)
				}
			}
			fieldLen := uint32(len(b.rt.ICFields)) - fieldOff

			category := runtime.ICUnique
			switch constraint.Type {
			case types.UniqueConstraint:
				category = runtime.ICUnique
			case types.KeyConstraint:
				category = runtime.ICKey
			case types.KeyRefConstraint:
				category = runtime.ICKeyRef
			}

			name := types.QName{Namespace: constraint.TargetNamespace, Local: constraint.Name}
			nameSym := b.internQName(name)
			ic := runtime.IdentityConstraint{
				Name:        nameSym,
				Category:    category,
				SelectorOff: selectorOff,
				SelectorLen: selectorLen,
				FieldOff:    fieldOff,
				FieldLen:    fieldLen,
			}
			b.rt.ICs = append(b.rt.ICs, ic)
			b.rt.ElemICs = append(b.rt.ElemICs, icID)
			scope := icByElem[elemID]
			if scope == nil {
				scope = make(map[types.QName]runtime.ICID)
				icByElem[elemID] = scope
			}
			scope[name] = icID

			if constraint.Type == types.KeyRefConstraint {
				pending = append(pending, keyrefPending{
					elem: elemID,
					id:   icID,
					name: constraint.ReferQName,
				})
			}
		}

		elem.ICOff = off
		elem.ICLen = uint32(len(b.rt.ElemICs)) - off
		b.rt.Elements[elemID] = elem
	}

	for _, ref := range pending {
		scope := icByElem[ref.elem]
		target, ok := scope[ref.name]
		if !ok {
			return fmt.Errorf("runtime build: keyref %s refers to missing key", ref.name)
		}
		if int(ref.id) >= len(b.rt.ICs) {
			return fmt.Errorf("runtime build: keyref constraint %d out of range", ref.id)
		}
		ic := b.rt.ICs[ref.id]
		ic.Referenced = target
		b.rt.ICs[ref.id] = ic
	}

	return nil
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
				continue
			}
		}
		sym := b.internQName(effectiveAttributeQName(b.schema, decl))
		use := runtime.AttrUse{
			Name: sym,
			Use:  toRuntimeAttrUse(decl.Use),
		}
		if vid, ok := b.validators.ValidatorForType(target.Type); ok {
			use.Validator = vid
		}
		if decl.HasDefault {
			if def, ok := b.validators.AttrUseDefaults[decl]; ok {
				use.Default = def
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s default missing", decl.Name)
			}
		}
		if decl.HasFixed {
			if fixed, ok := b.validators.AttrUseFixed[decl]; ok {
				use.Fixed = fixed
			} else {
				return nil, nil, fmt.Errorf("runtime build: attribute use %s fixed missing", decl.Name)
			}
		}
		if !use.Default.Present && !use.Fixed.Present {
			if attrID, ok := b.schemaAttrID(target); ok {
				if def, ok := b.validators.AttributeDefaults[attrID]; ok {
					use.Default = def
				}
				if fixed, ok := b.validators.AttributeFixed[attrID]; ok {
					use.Fixed = fixed
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
	for i, use := range uses {
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

func (b *schemaBuilder) addPath(program runtime.PathProgram) runtime.PathID {
	b.paths = append(b.paths, program)
	return runtime.PathID(len(b.paths) - 1)
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

func (b *schemaBuilder) internNamespace(ns types.NamespaceURI) runtime.NamespaceID {
	if b == nil {
		return 0
	}
	if b.rt != nil {
		if ns.IsEmpty() {
			return b.rt.PredefNS.Empty
		}
		return b.rt.Namespaces.Lookup([]byte(ns))
	}
	if b.builder == nil {
		return 0
	}
	if ns.IsEmpty() {
		return b.builder.InternNamespace(nil)
	}
	return b.builder.InternNamespace([]byte(ns))
}

func (b *schemaBuilder) internQName(qname types.QName) runtime.SymbolID {
	if b == nil {
		return 0
	}
	nsID := b.internNamespace(qname.Namespace)
	if nsID == 0 {
		return 0
	}
	if b.rt != nil {
		return b.rt.Symbols.Lookup(nsID, []byte(qname.Local))
	}
	if b.builder == nil {
		return 0
	}
	return b.builder.InternSymbol(nsID, []byte(qname.Local))
}

func toRuntimeAttrUse(use types.AttributeUse) runtime.AttrUseKind {
	switch use {
	case types.Required:
		return runtime.AttrRequired
	case types.Prohibited:
		return runtime.AttrProhibited
	default:
		return runtime.AttrOptional
	}
}

func toRuntimeElemBlock(block types.DerivationSet) runtime.ElemBlock {
	var out runtime.ElemBlock
	if block.Has(types.DerivationSubstitution) {
		out |= runtime.ElemBlockSubstitution
	}
	if block.Has(types.DerivationExtension) {
		out |= runtime.ElemBlockExtension
	}
	if block.Has(types.DerivationRestriction) {
		out |= runtime.ElemBlockRestriction
	}
	return out
}

func toRuntimeDerivation(mask types.DerivationMethod) runtime.DerivationMethod {
	var out runtime.DerivationMethod
	if mask&types.DerivationExtension != 0 {
		out |= runtime.DerExtension
	}
	if mask&types.DerivationRestriction != 0 {
		out |= runtime.DerRestriction
	}
	if mask&types.DerivationList != 0 {
		out |= runtime.DerList
	}
	if mask&types.DerivationUnion != 0 {
		out |= runtime.DerUnion
	}
	return out
}

func toRuntimeDerivationSet(set types.DerivationSet) runtime.DerivationMethod {
	var out runtime.DerivationMethod
	if set.Has(types.DerivationExtension) {
		out |= runtime.DerExtension
	}
	if set.Has(types.DerivationRestriction) {
		out |= runtime.DerRestriction
	}
	if set.Has(types.DerivationList) {
		out |= runtime.DerList
	}
	if set.Has(types.DerivationUnion) {
		out |= runtime.DerUnion
	}
	return out
}

func toRuntimeProcessContents(pc types.ProcessContents) runtime.ProcessContents {
	switch pc {
	case types.Lax:
		return runtime.PCLax
	case types.Skip:
		return runtime.PCSkip
	default:
		return runtime.PCStrict
	}
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
