package schema

import (
	"math"
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

const dfaEndPos = -1

func checkedSchemaUint32(n int, msg string) (uint32, error) {
	if n < 0 || uint64(n) > math.MaxUint32 {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
	}
	return uint32(n), nil
}

type compiledGuardKind uint8

const (
	compiledGuardExitMin compiledGuardKind = iota
	compiledGuardLoopMax
)

type compiledGuard struct {
	Slot uint32
	N    uint32
	Kind compiledGuardKind
}

type compiledActionKind uint8

const (
	compiledActionInc compiledActionKind = iota
	compiledActionReset
)

type compiledAction struct {
	Slot uint32
	Kind compiledActionKind
}

type dfaEntry struct {
	Guards  []compiledGuard
	Actions []compiledAction
	Pos     int
}

type dfaNode struct {
	First    []dfaEntry
	Last     []dfaEntry
	Counters []uint32
	Nullable bool
}

type dfaBuilder struct {
	c         *contentModelCompiler
	follow    map[int][]dfaEntry
	followIDs []dfaStateID
	states    map[string]uint32
	positions []contentParticle
	rows      []dfaSourceRow
	queue     [][]dfaEntry
	limit     int
	counters  uint32
}

// dfaStateID caches the canonical state assigned to one normalized follow
// position. known distinguishes the valid state ID zero from an unpopulated
// entry.
type dfaStateID struct {
	id    uint32
	known bool
}

type contentModelCompiler struct {
	names                 *nameTable
	rt                    *schemaBuild
	work                  *workBudget
	analysis              *contentModelAnalysis
	maxContentModelStates int
}

func newContentModelCompiler(
	names *nameTable,
	rt *schemaBuild,
	maxContentModelStates int,
	work *workBudget,
	analysis *contentModelAnalysis,
) contentModelCompiler {
	return contentModelCompiler{
		names:                 names,
		rt:                    rt,
		work:                  work,
		analysis:              analysis,
		maxContentModelStates: maxContentModelStates,
	}
}

type dfaSourceRow struct {
	Edges  []dfaSourceEdge
	Accept []dfaAccept
}

type dfaSourceEdge struct {
	Guards   []compiledGuard
	Actions  []compiledAction
	Pos      int
	Particle contentParticle
	To       uint32
}

type dfaAccept struct {
	Guards []compiledGuard
}

// compileContentModels lowers each source graph, constructs its finite
// automaton, determinizes and indexes it, and charges every traversal,
// configuration, transition, and emitted index entry to one shared budget.
func compileContentModels(
	names *nameTable,
	rt *schemaBuild,
	count int,
	maxContentModelStates int,
	work *workBudget,
	analysis *contentModelAnalysis,
) ([]compiledModel, error) {
	cc := newContentModelCompiler(names, rt, maxContentModelStates, work, analysis)
	compiled := make([]compiledModel, count)
	for id := range count {
		if err := work.spend(1); err != nil {
			return nil, err
		}
		m, err := cc.compileContentModel(contentModelID(id))
		if err != nil {
			return nil, err
		}
		compiled[id] = m
	}
	if err := indexCompiledModelsRows(rt, compiled, work.spend); err != nil {
		return nil, contentRestrictionCompileError(err)
	}
	return compiled, nil
}

// checkContentModelsUPA validates direct unique-particle-attribution checks
// that can be proven before compiled DFA construction. Traversal is root-local:
// each root uses its own seen tag, so results and work charges are never
// shared across roots.
func checkContentModelsUPA(
	names *nameTable,
	rt *schemaBuild,
	count int,
	work *workBudget,
	analysis *contentModelAnalysis,
) error {
	cc := newContentModelCompiler(names, rt, 0, work, analysis)
	seen := make([]int, count)
	for id := range count {
		if err := cc.checkContentModelUPA(contentModelID(id), seen, id+1); err != nil {
			return err
		}
	}
	return nil
}

func (c *contentModelCompiler) checkContentModelUPA(id contentModelID, seen []int, tag int) error {
	if err := c.work.spend(1); err != nil {
		return err
	}
	model, ok := contentModelByID(c.rt.Models, id)
	if !ok {
		return xsderrors.InternalInvariant("UPA check references missing content model")
	}
	needsSplit, err := c.modelNeedsRuntimeSplitSeen(id, model, seen, tag)
	if err != nil {
		return err
	}
	wildcardOverlap, err := c.sequenceHasWildcardEquivalentOverlap(model)
	if err != nil {
		return err
	}
	if needsSplit || wildcardOverlap {
		return nil
	}
	return c.checkDirectUPA(model)
}

// elementDeclarationConsistencyChecker is the one charged, model-cached EDC
// check shared by compilation and publication. Same-named particles,
// including effective transitive substitution members, must share one named
// global type; generated internal names never make an anonymous type named.
// A single particle is visited once regardless of its occurrence range.
type elementDeclarationConsistencyChecker struct {
	rt     *schemaBuild
	work   contentModelWork
	states map[contentModelID]contentModelVisitState
	cache  map[contentModelID]map[QName]TypeID
}

type contentModelVisitState uint8

const (
	contentModelVisiting contentModelVisitState = iota + 1
	contentModelVisited
)

func newElementDeclarationConsistencyChecker(
	rt *schemaBuild,
	work contentModelWork,
) *elementDeclarationConsistencyChecker {
	return &elementDeclarationConsistencyChecker{
		rt:     rt,
		work:   work,
		states: make(map[contentModelID]contentModelVisitState),
		cache:  make(map[contentModelID]map[QName]TypeID),
	}
}

func (c *elementDeclarationConsistencyChecker) spend() error {
	if c.work == nil {
		return xsderrors.InternalInvariant("element declaration consistency work budget is missing")
	}
	return c.work(1)
}

func (c *elementDeclarationConsistencyChecker) checkModel(id contentModelID) error {
	_, err := c.collectModel(id)
	return err
}

func (c *elementDeclarationConsistencyChecker) collectModel(
	id contentModelID,
) (map[QName]TypeID, error) {
	if err := c.spend(); err != nil {
		return nil, err
	}
	switch c.states[id] {
	case contentModelVisiting:
		return nil, xsderrors.InternalInvariant("element declaration consistency check references cyclic content model")
	case contentModelVisited:
		return c.cache[id], nil
	}
	if !validContentModelID(id, len(c.rt.Models)) {
		return nil, xsderrors.InternalInvariant("element declaration consistency check references missing content model")
	}
	model := c.rt.Models[id]
	c.states[id] = contentModelVisiting
	types, err := c.collectParticles(model.Particles)
	if err != nil {
		delete(c.states, id)
		return nil, err
	}
	c.states[id] = contentModelVisited
	c.cache[id] = types
	return types, nil
}

func (c *elementDeclarationConsistencyChecker) collectParticles(
	particles []contentParticle,
) (map[QName]TypeID, error) {
	types := make(map[QName]TypeID)
	for _, p := range particles {
		if err := c.spend(); err != nil {
			return nil, err
		}
		if err := c.collectParticle(types, p); err != nil {
			return nil, err
		}
	}
	return types, nil
}

func (c *elementDeclarationConsistencyChecker) collectParticle(types map[QName]TypeID, particle contentParticle) error {
	switch particle.Kind {
	case particleElement:
		return c.collectElementParticle(types, particle.Element)
	case particleModel:
		return c.collectModelParticle(types, particle.Model)
	case particleWildcard:
		return nil
	default:
	}
	return nil
}

func (c *elementDeclarationConsistencyChecker) collectElementParticle(types map[QName]TypeID, element ElementID) error {
	if err := c.collectElementDeclaration(types, element); err != nil {
		return err
	}
	var err error
	c.rt.Substitutions.ForEachMember(element, func(member ElementID) bool {
		err = c.collectSubstitutionMember(types, element, member)
		return err == nil
	})
	return err
}

func (c *elementDeclarationConsistencyChecker) collectSubstitutionMember(types map[QName]TypeID, head, member ElementID) error {
	// EDC includes the actual substitution group, but every raw member scan
	// consumes work even when abstractness or blocking excludes that member.
	if err := c.spend(); err != nil {
		return err
	}
	name, ok := c.rt.ElementName(member)
	if !ok {
		return xsderrors.InternalInvariant("element declaration consistency references missing substitution member")
	}
	if accepted, ok := c.rt.Substitutions.MemberByName(head, name); !ok || accepted != member {
		return nil
	}
	return c.collectElementDeclaration(types, member)
}

func (c *elementDeclarationConsistencyChecker) collectElementDeclaration(types map[QName]TypeID, element ElementID) error {
	name, ok := c.rt.ElementName(element)
	if !ok {
		return xsderrors.InternalInvariant("element declaration consistency check references missing element name")
	}
	typ, ok := elementTypeByID(c.rt.Elements, element)
	if !ok {
		return xsderrors.InternalInvariant("element declaration consistency check references missing element type")
	}
	return c.addElementDeclarationType(types, name, typ)
}

func (c *elementDeclarationConsistencyChecker) collectModelParticle(types map[QName]TypeID, model contentModelID) error {
	nested, err := c.collectModel(model)
	if err != nil {
		return err
	}
	for name, typ := range nested {
		if err := c.spend(); err != nil {
			return err
		}
		if err := c.addElementDeclarationType(types, name, typ); err != nil {
			return err
		}
	}
	return nil
}

func (c *elementDeclarationConsistencyChecker) addElementDeclarationType(
	types map[QName]TypeID,
	name QName,
	typ TypeID,
) error {
	if previous, ok := types[name]; ok {
		if previous != typ {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations with the same name must have the same type")
		}
		named, err := c.hasNamedType(typ)
		if err != nil {
			return err
		}
		if !named {
			return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations with the same name must use a named top-level type")
		}
	}
	types[name] = typ
	return nil
}

func (c *elementDeclarationConsistencyChecker) hasNamedType(typ TypeID) (bool, error) {
	name, valid := typeNameByID(c.rt.SimpleTypes, c.rt.ComplexTypes, typ)
	if !valid {
		return false, xsderrors.InternalInvariant("element declaration consistency references invalid type")
	}
	var scope declarationScope
	if id, simple := typ.Simple(); simple {
		st := c.rt.SimpleTypes[id]
		if st.Missing {
			return true, nil
		}
		scope = st.Scope
	} else {
		id, _ := typ.Complex()
		scope = c.rt.ComplexTypes[id].Scope
	}
	bound, ok := c.rt.GlobalTypes[name]
	return scope == declarationScopeGlobal && ok && bound == typ, nil
}

func (c *contentModelCompiler) modelNeedsRuntimeSplitSeen(id contentModelID, model contentModelNode, seen []int, tag int) (bool, error) {
	if err := c.work.spend(1); err != nil {
		return false, err
	}
	if validUint32Index(uint32(id), len(seen)) {
		if seen[id] == tag {
			return false, nil
		}
		seen[id] = tag
	}
	if choiceNeedsRuntimeSplit(model, model.Occurs) {
		return true, nil
	}
	for _, p := range model.Particles {
		needsSplit, err := c.particleNeedsRuntimeSplit(p, seen, tag)
		if err != nil {
			return false, err
		}
		if needsSplit {
			return true, nil
		}
	}
	return false, nil
}

func (c *contentModelCompiler) particleNeedsRuntimeSplit(particle contentParticle, seen []int, tag int) (bool, error) {
	if err := c.work.spend(1); err != nil {
		return false, err
	}
	if particle.Kind != particleModel {
		return false, nil
	}
	model, ok := contentModelByID(c.rt.Models, particle.Model)
	if !ok {
		return false, nil
	}
	if choiceNeedsRuntimeSplit(model, particle.Occurs) {
		return true, nil
	}
	return c.modelNeedsRuntimeSplitSeen(particle.Model, model, seen, tag)
}

func choiceNeedsRuntimeSplit(model contentModelNode, occurs occurrence) bool {
	if model.Kind != modelChoice || occurs.IsExactlyOne() {
		return false
	}
	for _, p := range model.Particles {
		if !p.Occurs.IsExactlyOne() {
			return true
		}
	}
	return false
}

func (c *contentModelCompiler) checkDirectUPA(model contentModelNode) error {
	switch model.Kind {
	case modelChoice:
		return c.checkPairwiseUPA(model.Particles, "UPA violation: overlapping particles in choice")
	case modelAll:
		return c.checkPairwiseUPA(model.Particles, "UPA violation: overlapping particles in all")
	case modelSequence:
		return c.checkSequenceUPA(model)
	case modelEmpty, modelAny:
		return nil
	default:
	}
	return nil
}

func (c *contentModelCompiler) checkSequenceUPA(model contentModelNode) error {
	for i, p := range model.Particles {
		candidates, err := c.particleContinuationParticles(p)
		if err != nil {
			return err
		}
		if err := c.work.spend(len(candidates)); err != nil {
			return err
		}
		for _, candidate := range candidates {
			if err := c.checkSequenceContinuationUPA(model, candidate, i+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *contentModelCompiler) checkSequenceContinuationUPA(model contentModelNode, candidate contentParticle, start int) error {
	for j := start; j < len(model.Particles); j++ {
		if err := c.work.spend(1); err != nil {
			return err
		}
		stop, err := c.checkSequenceContinuation(candidate, model.Particles[j], model.Occurs)
		if err != nil {
			return err
		}
		if stop {
			break
		}
	}
	return nil
}

func (c *contentModelCompiler) checkSequenceContinuation(candidate, next contentParticle, modelOccurs occurrence) (bool, error) {
	name, overlap, err := c.particlesOverlap(candidate, next)
	if err != nil {
		return false, err
	}
	if !overlap {
		return next.Occurs.Min > 0, nil
	}
	if !modelOccurs.IsExactlyOne() && c.wildcardEquivalentOverlap(candidate, next) {
		return false, nil
	}
	return false, c.upaError("UPA violation: duplicate element in sequence", name)
}

func (c *contentModelCompiler) particleContinuationParticles(p contentParticle) ([]contentParticle, error) {
	if err := c.work.spend(1); err != nil {
		return nil, err
	}
	if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
		return nil, nil
	}
	switch p.Kind {
	case particleElement, particleWildcard:
		return c.leafContinuationParticles(p)
	case particleModel:
		return c.nestedModelContinuationParticles(p)
	}
	return nil, nil
}

func (c *contentModelCompiler) leafContinuationParticles(particle contentParticle) ([]contentParticle, error) {
	overlaps, err := c.particleCanOverlapFollowing(particle)
	if err != nil || !overlaps {
		return nil, err
	}
	return []contentParticle{particle}, nil
}

func (c *contentModelCompiler) nestedModelContinuationParticles(particle contentParticle) ([]contentParticle, error) {
	model, ok := contentModelByID(c.rt.Models, particle.Model)
	if !ok {
		return nil, nil
	}
	var out []contentParticle
	if particle.Occurs.Unbounded || particle.Occurs.Max > particle.Occurs.Min {
		start, err := c.modelStartParticles(model)
		if err != nil {
			return nil, err
		}
		if err := c.work.spend(len(start)); err != nil {
			return nil, err
		}
		out = append(out, start...)
	}
	continuations, err := c.modelContinuationParticles(model)
	if err != nil {
		return nil, err
	}
	if err := c.work.spend(len(continuations)); err != nil {
		return nil, err
	}
	return append(out, continuations...), nil
}

func (c *contentModelCompiler) modelContinuationParticles(model contentModelNode) ([]contentParticle, error) {
	if err := c.work.spend(1); err != nil {
		return nil, err
	}
	out, err := c.repeatedModelStartParticles(model)
	if err != nil {
		return nil, err
	}
	switch model.Kind {
	case modelSequence, modelChoice:
		return c.appendParticleContinuations(out, model.Particles)
	case modelEmpty, modelAny, modelAll:
		return out, nil
	default:
	}
	return out, nil
}

func (c *contentModelCompiler) repeatedModelStartParticles(model contentModelNode) ([]contentParticle, error) {
	if !model.Occurs.Unbounded && model.Occurs.Max <= model.Occurs.Min {
		return nil, nil
	}
	start, err := c.modelStartParticles(model)
	if err != nil {
		return nil, err
	}
	if err := c.work.spend(len(start)); err != nil {
		return nil, err
	}
	return start, nil
}

func (c *contentModelCompiler) appendParticleContinuations(out []contentParticle, particles []contentParticle) ([]contentParticle, error) {
	for _, particle := range particles {
		continuations, err := c.particleContinuationParticles(particle)
		if err != nil {
			return nil, err
		}
		if err := c.work.spend(len(continuations)); err != nil {
			return nil, err
		}
		out = append(out, continuations...)
	}
	return out, nil
}

func (c *contentModelCompiler) particleCanOverlapFollowing(p contentParticle) (bool, error) {
	r, err := c.analysis.ParticleCountRange(p)
	return r.Unbounded || r.Max > r.Min, err
}

func (c *contentModelCompiler) sequenceHasWildcardEquivalentOverlap(model contentModelNode) (bool, error) {
	if model.Kind != modelSequence {
		return false, nil
	}
	for i, p := range model.Particles {
		overlap, err := c.particleHasWildcardEquivalentContinuation(model, p, i+1)
		if err != nil {
			return false, err
		}
		if overlap {
			return true, nil
		}
	}
	return false, nil
}

func (c *contentModelCompiler) particleHasWildcardEquivalentContinuation(model contentModelNode, particle contentParticle, start int) (bool, error) {
	candidates, err := c.particleContinuationParticles(particle)
	if err != nil {
		return false, err
	}
	for _, candidate := range candidates {
		overlap, err := c.wildcardCandidateOverlaps(model.Particles, candidate, start)
		if err != nil || overlap {
			return overlap, err
		}
	}
	return false, nil
}

func (c *contentModelCompiler) wildcardCandidateOverlaps(particles []contentParticle, candidate contentParticle, start int) (bool, error) {
	for i := start; i < len(particles); i++ {
		if err := c.work.spend(1); err != nil {
			return false, err
		}
		if c.wildcardEquivalentOverlap(candidate, particles[i]) {
			return true, nil
		}
		if particles[i].Occurs.Min > 0 {
			break
		}
	}
	return false, nil
}

func (c *contentModelCompiler) wildcardEquivalentOverlap(a, b contentParticle) bool {
	if a.Kind != particleWildcard || b.Kind != particleWildcard {
		return false
	}
	wa, ok := wildcardByID(c.rt.Wildcards, a.Wildcard)
	if !ok {
		return false
	}
	wb, ok := wildcardByID(c.rt.Wildcards, b.Wildcard)
	if !ok {
		return false
	}
	return wildcardNamespaceEqual(wa, wb)
}

func (c *contentModelCompiler) modelStartParticles(model contentModelNode) ([]contentParticle, error) {
	if err := c.work.spend(1); err != nil {
		return nil, err
	}
	switch model.Kind {
	case modelAll, modelChoice:
		if err := c.work.spend(len(model.Particles)); err != nil {
			return nil, err
		}
		return slices.Clone(model.Particles), nil
	case modelSequence:
		return c.sequenceStartParticles(model.Particles)
	case modelEmpty, modelAny:
		return nil, nil
	default:
	}
	return nil, nil
}

func (c *contentModelCompiler) sequenceStartParticles(particles []contentParticle) ([]contentParticle, error) {
	var out []contentParticle
	for _, particle := range particles {
		if err := c.work.spend(1); err != nil {
			return nil, err
		}
		out = append(out, particle)
		emptiable, err := c.analysis.ParticleEmptiable(particle)
		if err != nil {
			return nil, err
		}
		if !emptiable {
			break
		}
	}
	return out, nil
}

func (c *contentModelCompiler) compileContentModel(id contentModelID) (compiledModel, error) {
	model, ok := contentModelByID(c.rt.Models, id)
	if !ok {
		return compiledModel{}, xsderrors.InternalInvariant("content model compiler references missing content model")
	}
	model = normalizeSingleParticleModel(model)
	switch model.Kind {
	case modelEmpty:
		return compiledModel{Kind: compiledModelEmpty, Empty: true}, nil
	case modelAny:
		return compiledModel{Kind: compiledModelAny, Empty: true}, nil
	case modelAll:
		return c.compileAllModel(model)
	case modelSequence, modelChoice:
		if m, ok, err := c.compileDirectModel(model); ok || err != nil {
			return m, err
		}
		b := &dfaBuilder{
			c:      c,
			follow: make(map[int][]dfaEntry),
			states: make(map[string]uint32),
			limit:  c.maxContentModelStates,
		}
		return b.compile(id)
	default:
		return compiledModel{}, xsderrors.InternalInvariant("content model has unknown kind")
	}
}

func normalizeSingleParticleModel(model contentModelNode) contentModelNode {
	if (model.Kind != modelSequence && model.Kind != modelChoice) ||
		len(model.Particles) != 1 {
		return model
	}
	particle := model.Particles[0]
	if !canFlattenSingleParticleModel(model.Occurs, particle.Occurs) {
		return model
	}
	particle.Occurs = multiplyOccurrence(particle.Occurs, model.Occurs)
	model.Kind = modelSequence
	model.Occurs = occurrence{Min: 1, Max: 1}
	model.Particles = []contentParticle{particle}
	return model
}

// executionParticle collapses representable singleton repetitions only in the
// execution projection; semantic models retain structural restriction guards.
func (c *contentModelCompiler) executionParticle(p contentParticle) (contentParticle, error) {
	for p.Kind == particleModel {
		child, flattened, err := c.executionSingleton(p)
		if err != nil || !flattened {
			return p, err
		}
		p = child
	}
	return p, nil
}

func (c *contentModelCompiler) executionSingleton(p contentParticle) (contentParticle, bool, error) {
	if err := c.work.spend(1); err != nil {
		return contentParticle{}, false, err
	}
	if !validContentModelID(p.Model, len(c.rt.Models)) {
		return contentParticle{}, false, xsderrors.InternalInvariant("execution particle references missing content model")
	}
	model := c.rt.Models[p.Model]
	if (model.Kind != modelSequence && model.Kind != modelChoice) || len(model.Particles) != 1 {
		return p, false, nil
	}
	child := model.Particles[0]
	if !canFlattenSingleParticleModel(model.Occurs, child.Occurs) {
		return p, false, nil
	}
	child.Occurs = multiplyOccurrence(child.Occurs, model.Occurs)
	if !canFlattenSingleParticleModel(p.Occurs, child.Occurs) {
		return p, false, nil
	}
	child.Occurs = multiplyOccurrence(child.Occurs, p.Occurs)
	return child, true, nil
}

func (c *contentModelCompiler) compileDirectModel(model contentModelNode) (compiledModel, bool, error) {
	if !model.Occurs.IsExactlyOne() {
		return compiledModel{}, false, nil
	}
	switch model.Kind {
	case modelSequence:
		return c.compileDirectSequenceModel(model)
	case modelChoice:
		return c.compileDirectChoiceModel(model)
	case modelEmpty, modelAny, modelAll:
		return compiledModel{}, false, nil
	default:
	}
	return compiledModel{}, false, nil
}

func (c *contentModelCompiler) compileDirectSequenceModel(model contentModelNode) (compiledModel, bool, error) {
	compilation := directSequenceCompilation{compiler: c, rows: []compiledModelRow{{}}, active: []uint32{0}}
	for _, particle := range model.Particles {
		supported, err := compilation.addParticle(particle)
		if err != nil || !supported {
			return compiledModel{}, supported, err
		}
	}
	for _, state := range compilation.active {
		compilation.rows[state].Accept = true
	}
	if err := c.checkCompiledRowsUPA(compilation.rows); err != nil {
		return compiledModel{}, false, err
	}
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  compilation.rows,
		Start: 0,
		Empty: compilation.rows[0].Accept,
	}, true, nil
}

type directSequenceCompilation struct {
	compiler *contentModelCompiler
	rows     []compiledModelRow
	active   []uint32
}

func (b *directSequenceCompilation) addParticle(particle contentParticle) (bool, error) {
	if err := b.compiler.work.spend(1); err != nil {
		return false, err
	}
	var err error
	particle, err = b.compiler.executionParticle(particle)
	if err != nil {
		return false, err
	}

	if particle.Kind != particleElement && particle.Kind != particleWildcard {
		return false, nil
	}
	if particle.Occurs.Max == 0 && !particle.Occurs.Unbounded {
		return true, nil
	}
	if particle.Occurs.IsExactlyOne() {
		return true, b.addExactParticle(particle)
	}
	return true, b.addRepeatedParticle(particle)
}

func (b *directSequenceCompilation) addExactParticle(particle contentParticle) error {
	edge := compiledModelEdge{Particle: singleParticle(particle)}
	to, err := b.appendRow(compiledModelRow{})
	if err != nil {
		return err
	}
	edge.To = to
	if err := b.appendActiveEdges(edge); err != nil {
		return err
	}
	b.active = []uint32{to}
	return nil
}

func (b *directSequenceCompilation) addRepeatedParticle(particle contentParticle) error {
	edge := compiledModelEdge{Particle: singleParticle(particle)}
	to, err := b.appendRow(compiledParticleRow(edge.Particle, particle.Occurs, compiledRowReject))
	if err != nil {
		return err
	}
	edge.To = to
	if err := b.appendActiveEdges(edge); err != nil {
		return err
	}
	if particle.Occurs.Unbounded || particle.Occurs.Max > 1 {
		b.rows[to].Edges = append(b.rows[to].Edges, edge)
	}
	next := []uint32{to}
	if particle.Occurs.Min == 0 {
		next = append(next, b.active...)
		slices.Sort(next)
		next = slices.Compact(next)
	}
	b.active = next
	return nil
}

func (b *directSequenceCompilation) appendRow(row compiledModelRow) (uint32, error) {
	to, err := checkedSchemaUint32(len(b.rows), "content model DFA state limit exceeded")
	if err != nil {
		return 0, err
	}
	b.rows, err = b.compiler.appendCompiledModelRow(b.rows, row)
	return to, err
}

func (b *directSequenceCompilation) appendActiveEdges(edge compiledModelEdge) error {
	for _, state := range b.active {
		if err := b.compiler.work.spend(1); err != nil {
			return err
		}
		b.rows[state].Edges = append(b.rows[state].Edges, edge)
	}
	return nil
}

func (c *contentModelCompiler) compileDirectChoiceModel(model contentModelNode) (compiledModel, bool, error) {
	compilation := directChoiceCompilation{compiler: c, rows: []compiledModelRow{{}}}
	for _, particle := range model.Particles {
		supported, err := compilation.addParticle(particle)
		if err != nil || !supported {
			return compiledModel{}, supported, err
		}
	}
	if err := c.checkCompiledRowsUPA(compilation.rows); err != nil {
		return compiledModel{}, false, err
	}
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  compilation.rows,
		Start: 0,
		Empty: compilation.rows[0].Accept,
	}, true, nil
}

type directChoiceCompilation struct {
	compiler *contentModelCompiler
	rows     []compiledModelRow
}

func (b *directChoiceCompilation) addParticle(particle contentParticle) (bool, error) {
	if err := b.compiler.work.spend(1); err != nil {
		return false, err
	}
	var err error
	particle, err = b.compiler.executionParticle(particle)
	if err != nil {
		return false, err
	}

	if particle.Kind != particleElement && particle.Kind != particleWildcard {
		return false, nil
	}
	if particle.Occurs.Max == 0 && !particle.Occurs.Unbounded {
		b.rows[0].Accept = true
		return true, nil
	}
	if particle.Occurs.Min == 0 {
		b.rows[0].Accept = true
	}
	to, err := checkedSchemaUint32(len(b.rows), "content model DFA state limit exceeded")
	if err != nil {
		return false, err
	}
	edge := compiledModelEdge{Particle: singleParticle(particle), To: to}
	return true, b.appendParticleRow(particle, edge)
}

func (b *directChoiceCompilation) appendParticleRow(particle contentParticle, edge compiledModelEdge) error {
	row := compiledParticleRow(edge.Particle, particle.Occurs, compiledRowAccept)
	if particle.Occurs.IsExactlyOne() {
		row = compiledModelRow{Accept: true}
	}
	var err error
	b.rows, err = b.compiler.appendCompiledModelRow(b.rows, row)
	if err != nil {
		return err
	}
	b.rows[0].Edges = append(b.rows[0].Edges, edge)
	if !particle.Occurs.IsExactlyOne() && (particle.Occurs.Unbounded || particle.Occurs.Max > 1) {
		b.rows[edge.To].Edges = append(b.rows[edge.To].Edges, edge)
	}
	return nil
}

func (c *contentModelCompiler) appendCompiledModelRow(rows []compiledModelRow, row compiledModelRow) ([]compiledModelRow, error) {
	if len(rows) >= c.maxContentModelStates {
		return nil, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded")
	}
	return append(rows, row), nil
}

type compiledRowAcceptance uint8

const (
	compiledRowReject compiledRowAcceptance = iota
	compiledRowAccept
)

func compiledParticleRow(p contentParticle, occurs occurrence, accept compiledRowAcceptance) compiledModelRow {
	row := compiledModelRow{Accept: accept == compiledRowAccept}
	if repeatNeedsCounter(occurs) {
		row.Counted = true
		row.CountParticle = p
		row.Min = occurs.Min
		row.Max = occurs.Max
		row.Unbounded = occurs.Unbounded
	}
	return row
}

func (c *contentModelCompiler) checkCompiledRowsUPA(rows []compiledModelRow) error {
	for state, row := range rows {
		needsCheck, err := c.compiledRowNeedsUPACheck(row)
		if err != nil {
			return err
		}
		if !needsCheck {
			continue
		}
		if err := c.checkCompiledRowUPA(uint32(state), row); err != nil {
			return err
		}
	}
	return nil
}

func (c *contentModelCompiler) checkCompiledRowUPA(state uint32, row compiledModelRow) error {
	for i, edge := range row.Edges {
		for j := i + 1; j < len(row.Edges); j++ {
			if err := c.checkCompiledEdgePairUPA(state, row, edge, row.Edges[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *contentModelCompiler) checkCompiledEdgePairUPA(state uint32, row compiledModelRow, a, b compiledModelEdge) error {
	if err := c.work.spend(1); err != nil {
		return err
	}
	name, overlap, err := c.particlesOverlap(a.Particle, b.Particle)
	if err != nil || !overlap {
		return err
	}
	if compiledCountingException(state, row, a, b) {
		return nil
	}
	return c.upaError("UPA violation: overlapping particles", name)
}

func (c *contentModelCompiler) compiledRowNeedsUPACheck(row compiledModelRow) (bool, error) {
	return c.particleSetNeedsUPACheck(len(row.Edges), func(index int) contentParticle {
		return row.Edges[index].Particle
	})
}

func (c *contentModelCompiler) particlesNeedUPACheck(particles []contentParticle) (bool, error) {
	return c.particleSetNeedsUPACheck(len(particles), func(index int) contentParticle {
		return particles[index]
	})
}

func (c *contentModelCompiler) particleSetNeedsUPACheck(
	count int,
	particleAt func(index int) contentParticle,
) (bool, error) {
	for i := range count {
		particle := particleAt(i)
		name, needsCheck := c.particleUPAName(particle)
		if needsCheck {
			return true, nil
		}
		for j := i + 1; j < count; j++ {
			needsCheck, err := c.particlePairNeedsUPACheck(name, particleAt(j))
			if err != nil || needsCheck {
				return needsCheck, err
			}
		}
	}
	return false, nil
}

func (c *contentModelCompiler) particleUPAName(particle contentParticle) (QName, bool) {
	if particle.Kind != particleElement || c.rt.Substitutions.HasMembers(particle.Element) {
		return QName{}, true
	}
	name, ok := c.rt.ElementName(particle.Element)
	return name, !ok
}

func (c *contentModelCompiler) particlePairNeedsUPACheck(name QName, next contentParticle) (bool, error) {
	if err := c.work.spend(1); err != nil {
		return false, err
	}
	nextName, needsCheck := c.particleUPAName(next)
	return needsCheck || nextName == name, nil
}

func singleParticle(p contentParticle) contentParticle {
	p.Occurs = occurrence{Min: 1, Max: 1}
	return p
}

func (c *contentModelCompiler) compileAllModel(model contentModelNode) (compiledModel, error) {
	if err := c.checkAllUPA(model); err != nil {
		return compiledModel{}, err
	}
	terms := make([]compiledAllTerm, 0, len(model.Particles))
	required := false
	for _, p := range model.Particles {
		if p.Kind == particleModel {
			return compiledModel{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all cannot contain model group particles")
		}
		if p.Occurs.Min > 0 {
			required = true
		}
		terms = append(terms, compiledAllTerm{
			Particle: p,
			Required: p.Occurs.Min > 0,
		})
	}
	allBitLen, err := checkedSchemaUint32((len(terms)+63)/64, "xs:all term limit exceeded")
	if err != nil {
		return compiledModel{}, err
	}
	return compiledModel{
		Kind:      compiledModelAll,
		All:       terms,
		AllBitLen: allBitLen,
		Empty:     model.Occurs.Min == 0 || !required,
	}, nil
}

func (c *contentModelCompiler) checkAllUPA(model contentModelNode) error {
	return c.checkPairwiseUPA(model.Particles, "UPA violation: overlapping particles in all")
}

func (c *contentModelCompiler) checkPairwiseUPA(particles []contentParticle, msg string) error {
	needsCheck, err := c.particlesNeedUPACheck(particles)
	if err != nil {
		return err
	}
	if !needsCheck {
		return nil
	}
	for i, p := range particles {
		if err := c.checkParticleAgainstFollowingUPA(p, particles[i+1:], msg); err != nil {
			return err
		}
	}
	return nil
}

func (c *contentModelCompiler) checkParticleAgainstFollowingUPA(particle contentParticle, following []contentParticle, msg string) error {
	for _, next := range following {
		if err := c.work.spend(1); err != nil {
			return err
		}
		name, overlap, err := c.particlesOverlap(particle, next)
		if err != nil {
			return err
		}
		if overlap {
			return c.upaError(msg, name)
		}
	}
	return nil
}

func (c *contentModelCompiler) particlesOverlap(a, b contentParticle) (QName, bool, error) {
	if c.analysis == nil {
		return QName{}, false, xsderrors.InternalInvariant("content model analysis is nil")
	}
	return c.analysis.Overlap(a, b)
}

func (c *contentModelCompiler) upaError(msg string, name QName) error {
	if c.names != nil && (name.Local != 0 || name.Namespace != 0) {
		msg += " " + c.names.Format(name)
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
}

func (b *dfaBuilder) compile(id contentModelID) (compiledModel, error) {
	root, err := b.modelNode(id)
	if err != nil {
		return compiledModel{}, err
	}
	start, err := b.rootStartEntries(root)
	if err != nil {
		return compiledModel{}, err
	}
	if followErr := b.appendRootEndFollows(root.Last); followErr != nil {
		return compiledModel{}, followErr
	}
	if normalizeErr := b.normalizeFollows(); normalizeErr != nil {
		return compiledModel{}, normalizeErr
	}
	startID, err := b.stateID(start)
	if err != nil {
		return compiledModel{}, err
	}
	if err := b.compileSourceRows(); err != nil {
		return compiledModel{}, err
	}
	if err := b.checkUPA(); err != nil {
		return compiledModel{}, err
	}
	return b.compileDeterministicModel(id, startID)
}

func (b *dfaBuilder) rootStartEntries(root dfaNode) ([]dfaEntry, error) {
	start := root.First
	if !root.Nullable {
		return start, nil
	}
	if err := b.c.work.spend(1); err != nil {
		return nil, err
	}
	return append(start, dfaEntry{Pos: dfaEndPos}), nil
}

func (b *dfaBuilder) appendRootEndFollows(last []dfaEntry) error {
	for _, tail := range last {
		if err := b.c.work.spend(len(tail.Guards) + len(tail.Actions) + 1); err != nil {
			return err
		}
		b.appendFollow(tail.Pos, dfaEntry{
			Pos: dfaEndPos, Guards: slices.Clone(tail.Guards), Actions: slices.Clone(tail.Actions),
		})
	}
	return nil
}

func (b *dfaBuilder) compileSourceRows() error {
	for len(b.queue) != 0 {
		entries := b.queue[0]
		b.queue[0] = nil
		b.queue = b.queue[1:]
		if err := b.c.work.spend(1); err != nil {
			return err
		}
		row, err := b.row(entries)
		if err != nil {
			return err
		}
		b.rows = append(b.rows, row)
	}
	return nil
}

func (b *dfaBuilder) row(entries []dfaEntry) (dfaSourceRow, error) {
	var row dfaSourceRow
	for _, e := range entries {
		if err := b.c.work.spend(len(e.Guards) + len(e.Actions) + 1); err != nil {
			return dfaSourceRow{}, err
		}
		if e.Pos == dfaEndPos {
			row.Accept = append(row.Accept, dfaAccept{
				Guards: slices.Clone(e.Guards),
			})
			continue
		}
		if e.Pos < 0 || e.Pos >= len(b.positions) {
			return dfaSourceRow{}, xsderrors.InternalInvariant("content model DFA references invalid position")
		}
		to, err := b.followStateID(e.Pos)
		if err != nil {
			return dfaSourceRow{}, err
		}
		row.Edges = append(row.Edges, dfaSourceEdge{
			Particle: b.positions[e.Pos],
			Guards:   slices.Clone(e.Guards),
			Actions:  slices.Clone(e.Actions),
			To:       to,
			Pos:      e.Pos,
		})
	}
	return row, nil
}

func (b *dfaBuilder) stateID(entries []dfaEntry) (uint32, error) {
	if err := b.spendDFAEntries(entries); err != nil {
		return 0, err
	}
	return b.internState(normalizeDFAEntries(entries))
}

// followStateID charges the follow set before consulting the position cache,
// so state discovery order and limit-failure order do not depend on caching.
func (b *dfaBuilder) followStateID(pos int) (uint32, error) {
	if err := b.spendDFAEntries(b.follow[pos]); err != nil {
		return 0, err
	}
	cache := &b.followIDs[pos]
	if cache.known {
		return cache.id, nil
	}
	id, err := b.internState(b.follow[pos])
	if err != nil {
		return 0, err
	}
	cache.id = id
	cache.known = true
	return id, nil
}

func (b *dfaBuilder) internState(entries []dfaEntry) (uint32, error) {
	key := dfaStateKey(entries)
	if id, ok := b.states[key]; ok {
		return id, nil
	}
	if len(b.states) >= b.limit {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded")
	}
	id, err := checkedSchemaUint32(len(b.states), "content model DFA state limit exceeded")
	if err != nil {
		return 0, err
	}
	b.states[key] = id
	b.queue = append(b.queue, entries)
	return id, nil
}

func (b *dfaBuilder) spendDFAEntries(entries []dfaEntry) error {
	if err := b.c.work.spend(len(entries) + 1); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := b.c.work.spend(len(entry.Guards) + len(entry.Actions)); err != nil {
			return err
		}
	}
	return nil
}

func (b *dfaBuilder) modelNode(id contentModelID) (dfaNode, error) {
	if err := b.c.work.spend(1); err != nil {
		return dfaNode{}, err
	}
	model, ok := contentModelByID(b.c.rt.Models, id)
	if !ok {
		return dfaNode{}, xsderrors.InternalInvariant("content model DFA references missing content model")
	}
	var (
		node dfaNode
		err  error
	)
	switch model.Kind {
	case modelEmpty:
		node.Nullable = true
	case modelSequence:
		node, err = b.sequenceNode(model.Particles)
		if err != nil {
			return dfaNode{}, err
		}
	case modelChoice:
		node, err = b.choiceNode(model.Particles)
		if err != nil {
			return dfaNode{}, err
		}
	case modelAll:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all cannot be nested in DFA content models")
	case modelAny:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported content model")
	default:
		err := xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported content model")
		return dfaNode{}, err
	}
	return b.repeat(node, model.Occurs)
}

func (b *dfaBuilder) sequenceNode(particles []contentParticle) (dfaNode, error) {
	node := dfaNode{Nullable: true}
	for _, particle := range particles {
		child, err := b.particleNode(particle)
		if err != nil {
			return dfaNode{}, err
		}
		if spendErr := b.c.work.spend(len(node.First) + len(node.Last) + len(child.First) + len(child.Last)); spendErr != nil {
			return dfaNode{}, spendErr
		}
		node, err = b.concat(node, child)
		if err != nil {
			return dfaNode{}, err
		}
	}
	return node, nil
}

func (b *dfaBuilder) choiceNode(particles []contentParticle) (dfaNode, error) {
	var node dfaNode
	for _, particle := range particles {
		child, err := b.particleNode(particle)
		if err != nil {
			return dfaNode{}, err
		}
		if err := b.c.work.spend(len(node.First) + len(node.Last) + len(child.First) + len(child.Last)); err != nil {
			return dfaNode{}, err
		}
		node = dfaChoice(node, child)
	}
	return node, nil
}

func (b *dfaBuilder) particleNode(p contentParticle) (dfaNode, error) {
	if err := b.c.work.spend(1); err != nil {
		return dfaNode{}, err
	}
	var node dfaNode
	switch p.Kind {
	case particleElement, particleWildcard:
		if err := b.c.work.spend(2); err != nil {
			return dfaNode{}, err
		}
		node = b.leaf(p)
	case particleModel:
		child, err := b.modelNode(p.Model)
		if err != nil {
			return dfaNode{}, err
		}
		node = child
	default:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported particle")
	}
	return b.repeat(node, p.Occurs)
}

func (b *dfaBuilder) leaf(p contentParticle) dfaNode {
	pos := len(b.positions)
	b.positions = append(b.positions, p)
	return dfaNode{
		First: []dfaEntry{{Pos: pos}},
		Last:  []dfaEntry{{Pos: pos}},
	}
}

func (b *dfaBuilder) concat(a, c dfaNode) (dfaNode, error) {
	for _, tail := range a.Last {
		for _, first := range c.First {
			if err := b.c.work.spend(
				len(tail.Guards) + len(tail.Actions) + len(first.Guards) + len(first.Actions) + 1,
			); err != nil {
				return dfaNode{}, err
			}
			b.appendFollow(tail.Pos, composeEntry(tail.Guards, tail.Actions, first))
		}
	}
	first := slices.Clone(a.First)
	if a.Nullable {
		first = append(first, c.First...)
	}
	last := slices.Clone(c.Last)
	if c.Nullable {
		last = append(last, a.Last...)
	}
	return dfaNode{
		First:    normalizeDFAEntries(first),
		Last:     normalizeDFAEntries(last),
		Counters: mergeCounters(a.Counters, c.Counters),
		Nullable: a.Nullable && c.Nullable,
	}, nil
}

func dfaChoice(a, c dfaNode) dfaNode {
	return dfaNode{
		First:    normalizeDFAEntries(append(slices.Clone(a.First), c.First...)),
		Last:     normalizeDFAEntries(append(slices.Clone(a.Last), c.Last...)),
		Counters: mergeCounters(a.Counters, c.Counters),
		Nullable: a.Nullable || c.Nullable,
	}
}

func (b *dfaBuilder) repeat(child dfaNode, occurs occurrence) (dfaNode, error) {
	if err := b.c.work.spend(1); err != nil {
		return dfaNode{}, err
	}
	if occurs.Max == 0 && !occurs.Unbounded {
		return dfaNode{Nullable: true}, nil
	}
	if occurs.IsExactlyOne() {
		return child, nil
	}
	return b.repeatMultiple(child, occurs)
}

func (b *dfaBuilder) repeatMultiple(child dfaNode, occurs occurrence) (dfaNode, error) {
	if err := b.c.work.spend(len(child.First) + len(child.Last) + len(child.Counters)); err != nil {
		return dfaNode{}, err
	}
	counter, err := b.repeatCounter(occurs)
	if err != nil {
		return dfaNode{}, err
	}
	last, err := b.repeatLastEntries(child, occurs, counter)
	if err != nil {
		return dfaNode{}, err
	}
	if occurs.Unbounded || occurs.Max > 1 {
		if err := b.addRepeatLoopFollows(child, occurs, counter); err != nil {
			return dfaNode{}, err
		}
	}
	return dfaNode{
		First:    child.First,
		Last:     normalizeDFAEntries(last),
		Counters: repeatCounterSet(child.Counters, counter),
		Nullable: occurs.Min == 0 || child.Nullable,
	}, nil
}

type dfaRepeatCounterKind uint8

const (
	dfaRepeatCounterInactive dfaRepeatCounterKind = iota
	dfaRepeatCounterActive
)

type dfaRepeatCounter struct {
	slot uint32
	kind dfaRepeatCounterKind
}

func (c dfaRepeatCounter) active() bool {
	switch c.kind {
	case dfaRepeatCounterInactive:
		return false
	case dfaRepeatCounterActive:
		return true
	default:
		panic("unknown DFA repeat counter kind")
	}
}

func (b *dfaBuilder) repeatCounter(occurs occurrence) (dfaRepeatCounter, error) {
	slot := -1
	if repeatNeedsCounter(occurs) {
		slot = int(b.newCounter())
	}
	if slot < 0 {
		return dfaRepeatCounter{slot: ^uint32(0), kind: dfaRepeatCounterInactive}, nil
	}
	self, err := checkedSchemaUint32(slot, "content model counter limit exceeded")
	return dfaRepeatCounter{slot: self, kind: dfaRepeatCounterActive}, err
}

func repeatCounterSet(child []uint32, counter dfaRepeatCounter) []uint32 {
	counters := slices.Clone(child)
	if counter.active() && !slices.Contains(counters, counter.slot) {
		counters = append(counters, counter.slot)
		slices.Sort(counters)
	}
	return counters
}

func (b *dfaBuilder) repeatLastEntries(child dfaNode, occurs occurrence, counter dfaRepeatCounter) ([]dfaEntry, error) {
	var exitGuards []compiledGuard
	var exitActions []compiledAction
	if counter.active() {
		if occurs.Min > 0 && !child.Nullable {
			exitGuards = append(exitGuards, compiledGuard{Slot: counter.slot, N: occurs.Min, Kind: compiledGuardExitMin})
		}
		exitActions = append(exitActions, compiledAction{Slot: counter.slot, Kind: compiledActionInc})
	}
	var last []dfaEntry
	for _, tail := range child.Last {
		if err := b.c.work.spend(len(tail.Guards) + len(exitGuards) + len(tail.Actions) + len(exitActions) + 1); err != nil {
			return nil, err
		}
		last = append(last, dfaEntry{
			Pos:     tail.Pos,
			Guards:  appendGuards(tail.Guards, exitGuards),
			Actions: appendActions(tail.Actions, exitActions),
		})
	}
	return last, nil
}

func (b *dfaBuilder) addRepeatLoopFollows(child dfaNode, occurs occurrence, counter dfaRepeatCounter) error {
	for _, tail := range child.Last {
		for _, first := range child.First {
			if err := b.addRepeatLoopFollow(tail, first, child.Counters, occurs, counter); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *dfaBuilder) addRepeatLoopFollow(tail, first dfaEntry, childCounters []uint32, occurs occurrence, counter dfaRepeatCounter) error {
	extraGuards := 0
	if counter.active() && !occurs.Unbounded {
		extraGuards = 1
	}
	extraActions := len(childCounters)
	if counter.active() {
		extraActions++
		if slices.Contains(childCounters, counter.slot) {
			extraActions--
		}
	}
	if err := b.c.work.spend(len(tail.Guards) + extraGuards + len(tail.Actions) + extraActions + len(first.Guards) + len(first.Actions) + 1); err != nil {
		return err
	}
	guards := slices.Clone(tail.Guards)
	if counter.active() && !occurs.Unbounded {
		guards = append(guards, compiledGuard{Slot: counter.slot, N: occurs.Max, Kind: compiledGuardLoopMax})
	}
	actions := slices.Clone(tail.Actions)
	if counter.active() {
		actions = append(actions, compiledAction{Slot: counter.slot, Kind: compiledActionInc})
	}
	actions = append(actions, resetActions(childCounters, counter.slot)...)
	b.appendFollow(tail.Pos, composeEntry(guards, actions, first))
	return nil
}

func repeatNeedsCounter(occurs occurrence) bool {
	if occurs.Min > 1 {
		return true
	}
	return !occurs.Unbounded && occurs.Max > 1
}

func (b *dfaBuilder) newCounter() uint32 {
	slot := b.counters
	b.counters++
	return slot
}

func (b *dfaBuilder) appendFollow(pos int, entry dfaEntry) {
	b.follow[pos] = append(b.follow[pos], entry)
}

func (b *dfaBuilder) normalizeFollows() error {
	if err := b.c.work.spend(len(b.follow)); err != nil {
		return err
	}
	for pos, entries := range b.follow {
		if err := b.spendDFAEntries(entries); err != nil {
			return err
		}
		b.follow[pos] = normalizeDFAEntries(entries)
	}
	b.followIDs = make([]dfaStateID, len(b.positions))
	return nil
}

func (b *dfaBuilder) checkUPA() error {
	for _, row := range b.rows {
		needsCheck, err := b.c.sourceRowNeedsUPACheck(row)
		if err != nil {
			return err
		}
		if !needsCheck {
			continue
		}
		if err := b.checkSourceRowUPA(row); err != nil {
			return err
		}
	}
	return nil
}

func (b *dfaBuilder) checkSourceRowUPA(row dfaSourceRow) error {
	for i, edge := range row.Edges {
		for j := i + 1; j < len(row.Edges); j++ {
			if err := b.checkSourceEdgePairUPA(edge, row.Edges[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *dfaBuilder) checkSourceEdgePairUPA(a, next dfaSourceEdge) error {
	if err := b.c.work.spend(1); err != nil {
		return err
	}
	if a.Pos == next.Pos {
		return nil
	}
	name, overlap, err := b.c.particlesOverlap(a.Particle, next.Particle)
	if err != nil || !overlap {
		return err
	}
	if err := b.c.work.spendProduct(len(a.Guards), len(next.Guards)); err != nil {
		return err
	}
	if err := b.c.work.spendProduct(len(next.Guards), len(a.Guards)); err != nil {
		return err
	}
	if countingException(a, next) {
		return nil
	}
	return b.c.upaError("UPA violation: overlapping particles", name)
}

func (c *contentModelCompiler) sourceRowNeedsUPACheck(row dfaSourceRow) (bool, error) {
	return c.particleSetNeedsUPACheck(len(row.Edges), func(index int) contentParticle {
		return row.Edges[index].Particle
	})
}

func canFlattenSingleParticleModel(modelOccurs, particleOccurs occurrence) bool {
	return occurrenceProductRepresentable(modelOccurs, particleOccurs) &&
		(modelOccurs.IsExactlyOne() ||
			particleOccurs.Min == 0 ||
			particleOccurs.IsExactlyOne() ||
			(particleOccurs.Unbounded && (modelOccurs.Min > 0 || particleOccurs.Min == 1)) ||
			(!modelOccurs.Unbounded && modelOccurs.Min == modelOccurs.Max))
}

func occurrenceProductRepresentable(a, b occurrence) bool {
	const maxUint32 = ^uint32(0)
	if a.Min != 0 && b.Min > maxUint32/a.Min {
		return false
	}
	if a.Unbounded || b.Unbounded {
		return true
	}
	return a.Max == 0 || b.Max <= maxUint32/a.Max
}
