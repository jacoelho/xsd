package compile

import (
	"math"
	"slices"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

const dfaEndPos = -1

func checkedUint32(n int, msg string) (uint32, error) {
	if n < 0 || uint64(n) > math.MaxUint32 {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
	}
	return uint32(n), nil
}

func saturatingUint32(n int) uint32 {
	if n < 0 || uint64(n) > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(n)
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
	states    map[string]uint32
	positions []runtime.Particle
	rows      []dfaSourceRow
	queue     [][]dfaEntry
	limits    []uint32
	limit     int
	counters  uint32
}

// ContentModelCompileRuntime supplies compiler-only substitution facts in addition to runtime model metadata.
type ContentModelCompileRuntime interface {
	runtime.CompiledModelRuntime
	HasSubstitutionMembers(id runtime.ElementID) bool
}

type contentModelCompiler struct {
	names                 *runtime.NameTable
	rt                    ContentModelCompileRuntime
	work                  *workBudget
	analysis              *runtime.ContentModelAnalysis
	maxContentModelStates int
}

func newContentModelCompiler(
	names *runtime.NameTable,
	rt ContentModelCompileRuntime,
	maxContentModelStates int,
	work *workBudget,
	analysis *runtime.ContentModelAnalysis,
) contentModelCompiler {
	return contentModelCompiler{
		names:                 names,
		rt:                    rt,
		work:                  work,
		analysis:              analysis,
		maxContentModelStates: maxContentModelStates,
	}
}

// ElementDeclarationRuntime supplies model and element metadata for compile-time
// model declaration consistency checks.
type ElementDeclarationRuntime interface {
	ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool)
	ElementName(id runtime.ElementID) (runtime.QName, bool)
	ElementType(id runtime.ElementID) (runtime.TypeID, bool)
}

type dfaSourceRow struct {
	Edges  []dfaSourceEdge
	Accept []dfaAccept
}

type dfaSourceEdge struct {
	Guards   []compiledGuard
	Actions  []compiledAction
	Pos      int
	Particle runtime.Particle
	To       uint32
}

type dfaAccept struct {
	Guards []compiledGuard
}

// CompileContentModels lowers each source graph, constructs its finite
// automaton, determinizes and indexes it, and charges every traversal,
// configuration, transition, and emitted index entry to one shared budget.
func CompileContentModels(
	names *runtime.NameTable,
	rt ContentModelCompileRuntime,
	count int,
	maxContentModelStates int,
	work *workBudget,
	analysis *runtime.ContentModelAnalysis,
) ([]runtime.CompiledModel, error) {
	cc := newContentModelCompiler(names, rt, maxContentModelStates, work, analysis)
	compiled := make([]runtime.CompiledModel, count)
	for id := range count {
		if err := work.spend(1); err != nil {
			return nil, err
		}
		m, err := cc.compileContentModel(runtime.ContentModelID(id))
		if err != nil {
			return nil, err
		}
		m.Source = runtime.ContentModelID(id)
		compiled[id] = m
	}
	if err := runtime.IndexCompiledModelsRows(rt, compiled, work.spend); err != nil {
		return nil, contentRestrictionCompileError(err)
	}
	return compiled, nil
}

// CheckContentModelsUPA validates direct unique-particle-attribution checks
// that can be proven before compiled DFA construction.
func CheckContentModelsUPA(
	names *runtime.NameTable,
	rt ContentModelCompileRuntime,
	count int,
	work *workBudget,
	analysis *runtime.ContentModelAnalysis,
) error {
	cc := newContentModelCompiler(names, rt, 0, work, analysis)
	seen := make([]bool, count)
	for id := range count {
		if err := cc.checkContentModelUPA(runtime.ContentModelID(id), seen); err != nil {
			return err
		}
	}
	return nil
}

func (c *contentModelCompiler) checkContentModelUPA(id runtime.ContentModelID, seen []bool) error {
	if err := c.work.spend(1); err != nil {
		return err
	}
	model, ok := c.rt.ContentModel(id)
	if !ok {
		return xsderrors.InternalInvariant("UPA check references missing content model")
	}
	clear(seen)
	needsSplit, err := c.modelNeedsRuntimeSplitSeen(id, model, seen)
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

type elementDeclarationConsistencyChecker struct {
	rt     ElementDeclarationRuntime
	work   *workBudget
	states map[runtime.ContentModelID]contentModelVisitState
	cache  map[runtime.ContentModelID]map[runtime.QName]runtime.TypeID
}

type contentModelVisitState uint8

const (
	contentModelVisiting contentModelVisitState = iota + 1
	contentModelVisited
)

func newElementDeclarationConsistencyChecker(
	rt ElementDeclarationRuntime,
	work *workBudget,
) *elementDeclarationConsistencyChecker {
	return &elementDeclarationConsistencyChecker{
		rt:     rt,
		work:   work,
		states: make(map[runtime.ContentModelID]contentModelVisitState),
		cache:  make(map[runtime.ContentModelID]map[runtime.QName]runtime.TypeID),
	}
}

func (c *elementDeclarationConsistencyChecker) spend(steps int) error {
	return c.work.spend(steps)
}

func (c *elementDeclarationConsistencyChecker) checkModel(id runtime.ContentModelID) error {
	_, err := c.collectModel(id)
	return err
}

func (c *elementDeclarationConsistencyChecker) collectModel(
	id runtime.ContentModelID,
) (map[runtime.QName]runtime.TypeID, error) {
	if err := c.spend(1); err != nil {
		return nil, err
	}
	switch c.states[id] {
	case contentModelVisiting:
		return nil, xsderrors.InternalInvariant("element declaration consistency check references cyclic content model")
	case contentModelVisited:
		return c.cache[id], nil
	}
	model, ok := c.rt.ContentModel(id)
	if !ok {
		return nil, xsderrors.InternalInvariant("element declaration consistency check references missing content model")
	}
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
	particles []runtime.Particle,
) (map[runtime.QName]runtime.TypeID, error) {
	types := make(map[runtime.QName]runtime.TypeID)
	for _, p := range particles {
		if err := c.spend(1); err != nil {
			return nil, err
		}
		if err := c.collectParticle(types, p); err != nil {
			return nil, err
		}
	}
	return types, nil
}

func (c *elementDeclarationConsistencyChecker) collectParticle(types map[runtime.QName]runtime.TypeID, particle runtime.Particle) error {
	switch particle.Kind {
	case runtime.ParticleElement:
		return c.collectElementParticle(types, particle.Element)
	case runtime.ParticleModel:
		return c.collectModelParticle(types, particle.Model)
	case runtime.ParticleWildcard:
		return nil
	default:
	}
	return nil
}

func (c *elementDeclarationConsistencyChecker) collectElementParticle(types map[runtime.QName]runtime.TypeID, element runtime.ElementID) error {
	name, ok := c.rt.ElementName(element)
	if !ok {
		return xsderrors.InternalInvariant("element declaration consistency check references missing element name")
	}
	typ, ok := c.rt.ElementType(element)
	if !ok {
		return xsderrors.InternalInvariant("element declaration consistency check references missing element type")
	}
	return addElementDeclarationType(types, name, typ)
}

func (c *elementDeclarationConsistencyChecker) collectModelParticle(types map[runtime.QName]runtime.TypeID, model runtime.ContentModelID) error {
	nested, err := c.collectModel(model)
	if err != nil {
		return err
	}
	for name, typ := range nested {
		if err := c.spend(1); err != nil {
			return err
		}
		if err := addElementDeclarationType(types, name, typ); err != nil {
			return err
		}
	}
	return nil
}

func addElementDeclarationType(
	types map[runtime.QName]runtime.TypeID,
	name runtime.QName,
	typ runtime.TypeID,
) error {
	if previous, ok := types[name]; ok && previous != typ {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations with the same name must have the same type")
	}
	types[name] = typ
	return nil
}

func (c *contentModelCompiler) modelNeedsRuntimeSplitSeen(id runtime.ContentModelID, model runtime.ContentModel, seen []bool) (bool, error) {
	if err := c.work.spend(1); err != nil {
		return false, err
	}
	if runtime.ValidUint32Index(uint32(id), len(seen)) {
		if seen[id] {
			return false, nil
		}
		seen[id] = true
	}
	if choiceNeedsRuntimeSplit(model, model.Occurs) {
		return true, nil
	}
	for _, p := range model.Particles {
		needsSplit, err := c.particleNeedsRuntimeSplit(p, seen)
		if err != nil {
			return false, err
		}
		if needsSplit {
			return true, nil
		}
	}
	return false, nil
}

func (c *contentModelCompiler) particleNeedsRuntimeSplit(particle runtime.Particle, seen []bool) (bool, error) {
	if err := c.work.spend(1); err != nil {
		return false, err
	}
	if particle.Kind != runtime.ParticleModel {
		return false, nil
	}
	model, ok := c.rt.ContentModel(particle.Model)
	if !ok {
		return false, nil
	}
	if choiceNeedsRuntimeSplit(model, particle.Occurs) {
		return true, nil
	}
	return c.modelNeedsRuntimeSplitSeen(particle.Model, model, seen)
}

func choiceNeedsRuntimeSplit(model runtime.ContentModel, occurs runtime.Occurrence) bool {
	if model.Kind != runtime.ModelChoice || occurs.IsExactlyOne() {
		return false
	}
	for _, p := range model.Particles {
		if !p.Occurs.IsExactlyOne() {
			return true
		}
	}
	return false
}

func (c *contentModelCompiler) checkDirectUPA(model runtime.ContentModel) error {
	switch model.Kind {
	case runtime.ModelChoice:
		return c.checkPairwiseUPA(model.Particles, "UPA violation: overlapping particles in choice")
	case runtime.ModelAll:
		return c.checkPairwiseUPA(model.Particles, "UPA violation: overlapping particles in all")
	case runtime.ModelSequence:
		return c.checkSequenceUPA(model)
	case runtime.ModelEmpty, runtime.ModelAny:
		return nil
	default:
	}
	return nil
}

func (c *contentModelCompiler) checkSequenceUPA(model runtime.ContentModel) error {
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

func (c *contentModelCompiler) checkSequenceContinuationUPA(model runtime.ContentModel, candidate runtime.Particle, start int) error {
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

func (c *contentModelCompiler) checkSequenceContinuation(candidate, next runtime.Particle, modelOccurs runtime.Occurrence) (bool, error) {
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

func (c *contentModelCompiler) particleContinuationParticles(p runtime.Particle) ([]runtime.Particle, error) {
	if err := c.work.spend(1); err != nil {
		return nil, err
	}
	if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
		return nil, nil
	}
	switch p.Kind {
	case runtime.ParticleElement, runtime.ParticleWildcard:
		return c.leafContinuationParticles(p)
	case runtime.ParticleModel:
		return c.nestedModelContinuationParticles(p)
	}
	return nil, nil
}

func (c *contentModelCompiler) leafContinuationParticles(particle runtime.Particle) ([]runtime.Particle, error) {
	overlaps, err := c.particleCanOverlapFollowing(particle)
	if err != nil || !overlaps {
		return nil, err
	}
	return []runtime.Particle{particle}, nil
}

func (c *contentModelCompiler) nestedModelContinuationParticles(particle runtime.Particle) ([]runtime.Particle, error) {
	model, ok := c.rt.ContentModel(particle.Model)
	if !ok {
		return nil, nil
	}
	var out []runtime.Particle
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

func (c *contentModelCompiler) modelContinuationParticles(model runtime.ContentModel) ([]runtime.Particle, error) {
	if err := c.work.spend(1); err != nil {
		return nil, err
	}
	out, err := c.repeatedModelStartParticles(model)
	if err != nil {
		return nil, err
	}
	switch model.Kind {
	case runtime.ModelSequence, runtime.ModelChoice:
		return c.appendParticleContinuations(out, model.Particles)
	case runtime.ModelEmpty, runtime.ModelAny, runtime.ModelAll:
		return out, nil
	default:
	}
	return out, nil
}

func (c *contentModelCompiler) repeatedModelStartParticles(model runtime.ContentModel) ([]runtime.Particle, error) {
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

func (c *contentModelCompiler) appendParticleContinuations(out []runtime.Particle, particles []runtime.Particle) ([]runtime.Particle, error) {
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

func (c *contentModelCompiler) particleCanOverlapFollowing(p runtime.Particle) (bool, error) {
	r, err := c.analysis.ParticleCountRange(p)
	return r.Unbounded || r.Max > r.Min, err
}

func (c *contentModelCompiler) sequenceHasWildcardEquivalentOverlap(model runtime.ContentModel) (bool, error) {
	if model.Kind != runtime.ModelSequence {
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

func (c *contentModelCompiler) particleHasWildcardEquivalentContinuation(model runtime.ContentModel, particle runtime.Particle, start int) (bool, error) {
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

func (c *contentModelCompiler) wildcardCandidateOverlaps(particles []runtime.Particle, candidate runtime.Particle, start int) (bool, error) {
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

func (c *contentModelCompiler) wildcardEquivalentOverlap(a, b runtime.Particle) bool {
	if a.Kind != runtime.ParticleWildcard || b.Kind != runtime.ParticleWildcard {
		return false
	}
	wa, ok := c.rt.Wildcard(a.Wildcard)
	if !ok {
		return false
	}
	wb, ok := c.rt.Wildcard(b.Wildcard)
	if !ok {
		return false
	}
	return runtime.WildcardNamespaceEqual(wa, wb)
}

func (c *contentModelCompiler) modelStartParticles(model runtime.ContentModel) ([]runtime.Particle, error) {
	if err := c.work.spend(1); err != nil {
		return nil, err
	}
	switch model.Kind {
	case runtime.ModelAll, runtime.ModelChoice:
		if err := c.work.spend(len(model.Particles)); err != nil {
			return nil, err
		}
		return slices.Clone(model.Particles), nil
	case runtime.ModelSequence:
		return c.sequenceStartParticles(model.Particles)
	case runtime.ModelEmpty, runtime.ModelAny:
		return nil, nil
	default:
	}
	return nil, nil
}

func (c *contentModelCompiler) sequenceStartParticles(particles []runtime.Particle) ([]runtime.Particle, error) {
	var out []runtime.Particle
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

func (c *contentModelCompiler) compileContentModel(id runtime.ContentModelID) (runtime.CompiledModel, error) {
	model, ok := c.rt.ContentModel(id)
	if !ok {
		return runtime.CompiledModel{}, xsderrors.InternalInvariant("content model compiler references missing content model")
	}
	model = normalizeSingleParticleModel(model)
	switch model.Kind {
	case runtime.ModelEmpty:
		return runtime.CompiledModel{Kind: runtime.CompiledModelEmpty, Mixed: model.Mixed, Empty: true}, nil
	case runtime.ModelAny:
		return runtime.CompiledModel{Kind: runtime.CompiledModelAny, Mixed: model.Mixed, Empty: true}, nil
	case runtime.ModelAll:
		return c.compileAllModel(model)
	case runtime.ModelSequence, runtime.ModelChoice:
		limits := model.ChoiceLimits
		if m, ok, err := c.compileDirectModel(model, limits); ok || err != nil {
			return m, err
		}
		b := &dfaBuilder{
			c:      c,
			follow: make(map[int][]dfaEntry),
			states: make(map[string]uint32),
			limits: limits,
			limit:  c.maxContentModelStates,
		}
		return b.compile(id)
	default:
		return runtime.CompiledModel{}, xsderrors.InternalInvariant("content model has unknown kind")
	}
}

func normalizeSingleParticleModel(model runtime.ContentModel) runtime.ContentModel {
	if (model.Kind != runtime.ModelSequence && model.Kind != runtime.ModelChoice) ||
		len(model.Particles) != 1 || len(model.ChoiceLimits) != 0 {
		return model
	}
	particle := model.Particles[0]
	if !canFlattenSingleParticleModel(model.Occurs, particle.Occurs) {
		return model
	}
	particle.Occurs = runtime.MultiplyOccurrence(particle.Occurs, model.Occurs)
	model.Kind = runtime.ModelSequence
	model.Occurs = runtime.Occurrence{Min: 1, Max: 1}
	model.Particles = []runtime.Particle{particle}
	return model
}

func (c *contentModelCompiler) compileDirectModel(model runtime.ContentModel, limits []uint32) (runtime.CompiledModel, bool, error) {
	if !model.Occurs.IsExactlyOne() {
		return runtime.CompiledModel{}, false, nil
	}
	switch model.Kind {
	case runtime.ModelSequence:
		return c.compileDirectSequenceModel(model, limits)
	case runtime.ModelChoice:
		return c.compileDirectChoiceModel(model)
	case runtime.ModelEmpty, runtime.ModelAny, runtime.ModelAll:
		return runtime.CompiledModel{}, false, nil
	default:
	}
	return runtime.CompiledModel{}, false, nil
}

func (c *contentModelCompiler) compileDirectSequenceModel(model runtime.ContentModel, limits []uint32) (runtime.CompiledModel, bool, error) {
	compilation := directSequenceCompilation{compiler: c, rows: []runtime.CompiledModelRow{{}}, active: []uint32{0}}
	for i, particle := range model.Particles {
		supported, err := compilation.addParticle(particle, i, limits)
		if err != nil || !supported {
			return runtime.CompiledModel{}, supported, err
		}
	}
	for _, state := range compilation.active {
		compilation.rows[state].Accept = true
	}
	if err := c.checkCompiledRowsUPA(compilation.rows); err != nil {
		return runtime.CompiledModel{}, false, err
	}
	return runtime.CompiledModel{
		Kind:  runtime.CompiledModelDFA,
		Rows:  compilation.rows,
		Start: 0,
		Mixed: model.Mixed,
		Empty: compilation.rows[0].Accept,
	}, true, nil
}

type directSequenceCompilation struct {
	compiler *contentModelCompiler
	rows     []runtime.CompiledModelRow
	active   []uint32
}

func (b *directSequenceCompilation) addParticle(particle runtime.Particle, index int, limits []uint32) (bool, error) {
	if err := b.compiler.work.spend(1); err != nil {
		return false, err
	}
	particle = applyRepeatedChoiceLimit(particle, index, limits)
	if particle.Kind != runtime.ParticleElement && particle.Kind != runtime.ParticleWildcard {
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

func (b *directSequenceCompilation) addExactParticle(particle runtime.Particle) error {
	edge := runtime.CompiledModelEdge{Particle: singleParticle(particle)}
	to, err := b.appendRow(runtime.CompiledModelRow{})
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

func (b *directSequenceCompilation) addRepeatedParticle(particle runtime.Particle) error {
	edge := runtime.CompiledModelEdge{Particle: singleParticle(particle)}
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

func (b *directSequenceCompilation) appendRow(row runtime.CompiledModelRow) (uint32, error) {
	to, err := checkedUint32(len(b.rows), "content model DFA state limit exceeded")
	if err != nil {
		return 0, err
	}
	b.rows, err = b.compiler.appendCompiledModelRow(b.rows, row)
	return to, err
}

func (b *directSequenceCompilation) appendActiveEdges(edge runtime.CompiledModelEdge) error {
	for _, state := range b.active {
		if err := b.compiler.work.spend(1); err != nil {
			return err
		}
		b.rows[state].Edges = append(b.rows[state].Edges, edge)
	}
	return nil
}

func (c *contentModelCompiler) compileDirectChoiceModel(model runtime.ContentModel) (runtime.CompiledModel, bool, error) {
	compilation := directChoiceCompilation{compiler: c, rows: []runtime.CompiledModelRow{{}}}
	for _, particle := range model.Particles {
		supported, err := compilation.addParticle(particle)
		if err != nil || !supported {
			return runtime.CompiledModel{}, supported, err
		}
	}
	if err := c.checkCompiledRowsUPA(compilation.rows); err != nil {
		return runtime.CompiledModel{}, false, err
	}
	return runtime.CompiledModel{
		Kind:  runtime.CompiledModelDFA,
		Rows:  compilation.rows,
		Start: 0,
		Mixed: model.Mixed,
		Empty: compilation.rows[0].Accept,
	}, true, nil
}

type directChoiceCompilation struct {
	compiler *contentModelCompiler
	rows     []runtime.CompiledModelRow
}

func (b *directChoiceCompilation) addParticle(particle runtime.Particle) (bool, error) {
	if err := b.compiler.work.spend(1); err != nil {
		return false, err
	}
	if particle.Kind != runtime.ParticleElement && particle.Kind != runtime.ParticleWildcard {
		return false, nil
	}
	if particle.Occurs.Max == 0 && !particle.Occurs.Unbounded {
		b.rows[0].Accept = true
		return true, nil
	}
	if particle.Occurs.Min == 0 {
		b.rows[0].Accept = true
	}
	to, err := checkedUint32(len(b.rows), "content model DFA state limit exceeded")
	if err != nil {
		return false, err
	}
	edge := runtime.CompiledModelEdge{Particle: singleParticle(particle), To: to}
	return true, b.appendParticleRow(particle, edge)
}

func (b *directChoiceCompilation) appendParticleRow(particle runtime.Particle, edge runtime.CompiledModelEdge) error {
	row := compiledParticleRow(edge.Particle, particle.Occurs, compiledRowAccept)
	if particle.Occurs.IsExactlyOne() {
		row = runtime.CompiledModelRow{Accept: true}
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

func (c *contentModelCompiler) appendCompiledModelRow(rows []runtime.CompiledModelRow, row runtime.CompiledModelRow) ([]runtime.CompiledModelRow, error) {
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

func compiledParticleRow(p runtime.Particle, occurs runtime.Occurrence, accept compiledRowAcceptance) runtime.CompiledModelRow {
	row := runtime.CompiledModelRow{Accept: accept == compiledRowAccept}
	if repeatNeedsCounter(occurs) {
		row.Counted = true
		row.CountParticle = p
		row.Min = occurs.Min
		row.Max = occurs.Max
		row.Unbounded = occurs.Unbounded
	}
	return row
}

func (c *contentModelCompiler) checkCompiledRowsUPA(rows []runtime.CompiledModelRow) error {
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

func (c *contentModelCompiler) checkCompiledRowUPA(state uint32, row runtime.CompiledModelRow) error {
	for i, edge := range row.Edges {
		for j := i + 1; j < len(row.Edges); j++ {
			if err := c.checkCompiledEdgePairUPA(state, row, edge, row.Edges[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *contentModelCompiler) checkCompiledEdgePairUPA(state uint32, row runtime.CompiledModelRow, a, b runtime.CompiledModelEdge) error {
	if err := c.work.spend(1); err != nil {
		return err
	}
	name, overlap, err := c.particlesOverlap(a.Particle, b.Particle)
	if err != nil || !overlap {
		return err
	}
	if runtime.CompiledCountingException(state, row, a, b) {
		return nil
	}
	return c.upaError("UPA violation: overlapping particles", name)
}

func (c *contentModelCompiler) compiledRowNeedsUPACheck(row runtime.CompiledModelRow) (bool, error) {
	return c.particleSetNeedsUPACheck(len(row.Edges), func(index int) runtime.Particle {
		return row.Edges[index].Particle
	})
}

func (c *contentModelCompiler) particlesNeedUPACheck(particles []runtime.Particle) (bool, error) {
	return c.particleSetNeedsUPACheck(len(particles), func(index int) runtime.Particle {
		return particles[index]
	})
}

func (c *contentModelCompiler) particleSetNeedsUPACheck(
	count int,
	particleAt func(index int) runtime.Particle,
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

func (c *contentModelCompiler) particleUPAName(particle runtime.Particle) (runtime.QName, bool) {
	if particle.Kind != runtime.ParticleElement || c.rt.HasSubstitutionMembers(particle.Element) {
		return runtime.QName{}, true
	}
	name, ok := c.rt.ElementName(particle.Element)
	return name, !ok
}

func (c *contentModelCompiler) particlePairNeedsUPACheck(name runtime.QName, next runtime.Particle) (bool, error) {
	if err := c.work.spend(1); err != nil {
		return false, err
	}
	nextName, needsCheck := c.particleUPAName(next)
	return needsCheck || nextName == name, nil
}

func singleParticle(p runtime.Particle) runtime.Particle {
	p.Occurs = runtime.Occurrence{Min: 1, Max: 1}
	return p
}

func applyRepeatedChoiceLimit(p runtime.Particle, index int, limits []uint32) runtime.Particle {
	if !slices.Contains(limits, saturatingUint32(index)) || p.Occurs.Min > 1 {
		return p
	}
	if p.Occurs.Unbounded || p.Occurs.Max > 1 {
		p.Occurs.Unbounded = false
		p.Occurs.Max = 1
	}
	return p
}

func (c *contentModelCompiler) compileAllModel(model runtime.ContentModel) (runtime.CompiledModel, error) {
	if err := c.checkAllUPA(model); err != nil {
		return runtime.CompiledModel{}, err
	}
	terms := make([]runtime.CompiledAllTerm, 0, len(model.Particles))
	required := false
	for _, p := range model.Particles {
		if p.Kind == runtime.ParticleModel {
			return runtime.CompiledModel{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all cannot contain model group particles")
		}
		if p.Occurs.Min > 0 {
			required = true
		}
		terms = append(terms, runtime.CompiledAllTerm{
			Particle: p,
			Required: p.Occurs.Min > 0,
		})
	}
	allBitLen, err := checkedUint32((len(terms)+63)/64, "xs:all term limit exceeded")
	if err != nil {
		return runtime.CompiledModel{}, err
	}
	return runtime.CompiledModel{
		Kind:      runtime.CompiledModelAll,
		All:       terms,
		AllBitLen: allBitLen,
		Mixed:     model.Mixed,
		Empty:     model.Occurs.Min == 0 || !required,
	}, nil
}

func (c *contentModelCompiler) checkAllUPA(model runtime.ContentModel) error {
	return c.checkPairwiseUPA(model.Particles, "UPA violation: overlapping particles in all")
}

func (c *contentModelCompiler) checkPairwiseUPA(particles []runtime.Particle, msg string) error {
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

func (c *contentModelCompiler) checkParticleAgainstFollowingUPA(particle runtime.Particle, following []runtime.Particle, msg string) error {
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

func (c *contentModelCompiler) particlesOverlap(a, b runtime.Particle) (runtime.QName, bool, error) {
	if c.analysis == nil {
		return runtime.QName{}, false, xsderrors.InternalInvariant("content model analysis is nil")
	}
	return c.analysis.Overlap(a, b)
}

func (c *contentModelCompiler) upaError(msg string, name runtime.QName) error {
	if c.names != nil && (name.Local != 0 || name.Namespace != 0) {
		msg += " " + c.names.Format(name)
	}
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, msg)
}

func (b *dfaBuilder) compile(id runtime.ContentModelID) (runtime.CompiledModel, error) {
	root, err := b.modelNode(id, choiceLimitRoot)
	if err != nil {
		return runtime.CompiledModel{}, err
	}
	start, err := b.rootStartEntries(root)
	if err != nil {
		return runtime.CompiledModel{}, err
	}
	if followErr := b.appendRootEndFollows(root.Last); followErr != nil {
		return runtime.CompiledModel{}, followErr
	}
	if normalizeErr := b.normalizeFollows(); normalizeErr != nil {
		return runtime.CompiledModel{}, normalizeErr
	}
	startID, err := b.stateID(start)
	if err != nil {
		return runtime.CompiledModel{}, err
	}
	if err := b.compileSourceRows(); err != nil {
		return runtime.CompiledModel{}, err
	}
	if err := b.checkUPA(); err != nil {
		return runtime.CompiledModel{}, err
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
		to, err := b.stateID(b.follow[e.Pos])
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
	entries = normalizeDFAEntries(entries)
	key := dfaStateKey(entries)
	if id, ok := b.states[key]; ok {
		return id, nil
	}
	if len(b.states) >= b.limit {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded")
	}
	id, err := checkedUint32(len(b.states), "content model DFA state limit exceeded")
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

type choiceLimitScope uint8

const (
	choiceLimitRoot choiceLimitScope = iota
	choiceLimitNested
)

func (b *dfaBuilder) modelNode(id runtime.ContentModelID, scope choiceLimitScope) (dfaNode, error) {
	if err := b.c.work.spend(1); err != nil {
		return dfaNode{}, err
	}
	model, ok := b.c.rt.ContentModel(id)
	if !ok {
		return dfaNode{}, xsderrors.InternalInvariant("content model DFA references missing content model")
	}
	var (
		node dfaNode
		err  error
	)
	switch model.Kind {
	case runtime.ModelEmpty:
		node.Nullable = true
	case runtime.ModelSequence:
		node, err = b.sequenceNode(model.Particles, scope)
		if err != nil {
			return dfaNode{}, err
		}
	case runtime.ModelChoice:
		node, err = b.choiceNode(model.Particles, scope)
		if err != nil {
			return dfaNode{}, err
		}
	case runtime.ModelAll:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all cannot be nested in DFA content models")
	case runtime.ModelAny:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported content model")
	default:
		err := xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported content model")
		return dfaNode{}, err
	}
	return b.repeat(node, model.Occurs, -1)
}

func (b *dfaBuilder) sequenceNode(particles []runtime.Particle, scope choiceLimitScope) (dfaNode, error) {
	node := dfaNode{Nullable: true}
	for i, particle := range particles {
		child, err := b.particleNode(particle, i, scope)
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

func (b *dfaBuilder) choiceNode(particles []runtime.Particle, scope choiceLimitScope) (dfaNode, error) {
	var node dfaNode
	for i, particle := range particles {
		child, err := b.particleNode(particle, i, scope)
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

func (b *dfaBuilder) particleNode(p runtime.Particle, index int, scope choiceLimitScope) (dfaNode, error) {
	if err := b.c.work.spend(1); err != nil {
		return dfaNode{}, err
	}
	var node dfaNode
	if scope == choiceLimitRoot {
		p = applyRepeatedChoiceLimit(p, index, b.limits)
	}
	switch p.Kind {
	case runtime.ParticleElement, runtime.ParticleWildcard:
		if err := b.c.work.spend(2); err != nil {
			return dfaNode{}, err
		}
		node = b.leaf(p)
	case runtime.ParticleModel:
		child, err := b.modelNode(p.Model, choiceLimitNested)
		if err != nil {
			return dfaNode{}, err
		}
		node = child
	default:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported particle")
	}
	slot := -1
	return b.repeat(node, p.Occurs, slot)
}

func (b *dfaBuilder) leaf(p runtime.Particle) dfaNode {
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

func (b *dfaBuilder) repeat(child dfaNode, occurs runtime.Occurrence, slot int) (dfaNode, error) {
	if err := b.c.work.spend(1); err != nil {
		return dfaNode{}, err
	}
	if occurs.Max == 0 && !occurs.Unbounded {
		return dfaNode{Nullable: true}, nil
	}
	if occurs.IsExactlyOne() {
		return b.repeatExactlyOnce(child, slot)
	}
	return b.repeatMultiple(child, occurs, slot)
}

func (b *dfaBuilder) repeatMultiple(child dfaNode, occurs runtime.Occurrence, slot int) (dfaNode, error) {
	if err := b.c.work.spend(len(child.First) + len(child.Last) + len(child.Counters)); err != nil {
		return dfaNode{}, err
	}
	counter, err := b.repeatCounter(occurs, slot)
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

func (b *dfaBuilder) repeatExactlyOnce(child dfaNode, slot int) (dfaNode, error) {
	if slot < 0 {
		return child, nil
	}
	slotID, err := checkedUint32(slot, "content model counter limit exceeded")
	if err != nil {
		return dfaNode{}, err
	}
	return b.countNode(child, slotID)
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

func (b *dfaBuilder) repeatCounter(occurs runtime.Occurrence, slot int) (dfaRepeatCounter, error) {
	if slot < 0 && repeatNeedsCounter(occurs) {
		slot = int(b.newCounter())
	}
	if slot < 0 {
		return dfaRepeatCounter{slot: ^uint32(0), kind: dfaRepeatCounterInactive}, nil
	}
	self, err := checkedUint32(slot, "content model counter limit exceeded")
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

func (b *dfaBuilder) repeatLastEntries(child dfaNode, occurs runtime.Occurrence, counter dfaRepeatCounter) ([]dfaEntry, error) {
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

func (b *dfaBuilder) addRepeatLoopFollows(child dfaNode, occurs runtime.Occurrence, counter dfaRepeatCounter) error {
	for _, tail := range child.Last {
		for _, first := range child.First {
			if err := b.addRepeatLoopFollow(tail, first, child.Counters, occurs, counter); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *dfaBuilder) addRepeatLoopFollow(tail, first dfaEntry, childCounters []uint32, occurs runtime.Occurrence, counter dfaRepeatCounter) error {
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

func repeatNeedsCounter(occurs runtime.Occurrence) bool {
	if occurs.Min > 1 {
		return true
	}
	return !occurs.Unbounded && occurs.Max > 1
}

func (b *dfaBuilder) countNode(child dfaNode, slot uint32) (dfaNode, error) {
	if err := b.c.work.spend(len(child.Counters)); err != nil {
		return dfaNode{}, err
	}
	action := []compiledAction{{Slot: slot, Kind: compiledActionInc}}
	last := make([]dfaEntry, 0, len(child.Last))
	for _, tail := range child.Last {
		if err := b.c.work.spend(len(tail.Guards) + len(tail.Actions) + len(action) + 1); err != nil {
			return dfaNode{}, err
		}
		last = append(last, dfaEntry{
			Pos:     tail.Pos,
			Guards:  slices.Clone(tail.Guards),
			Actions: appendActions(tail.Actions, action),
		})
	}
	counters := slices.Clone(child.Counters)
	if !slices.Contains(counters, slot) {
		counters = append(counters, slot)
		slices.Sort(counters)
	}
	return dfaNode{
		First:    child.First,
		Last:     normalizeDFAEntries(last),
		Counters: counters,
		Nullable: child.Nullable,
	}, nil
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
	return c.particleSetNeedsUPACheck(len(row.Edges), func(index int) runtime.Particle {
		return row.Edges[index].Particle
	})
}
