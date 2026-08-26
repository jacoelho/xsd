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
		if err := work.spend(1); err != nil {
			return err
		}
		modelID := runtime.ContentModelID(id)
		model, ok := cc.rt.ContentModel(modelID)
		if !ok {
			return xsderrors.InternalInvariant("UPA check references missing content model")
		}
		clear(seen)
		needsSplit, err := cc.modelNeedsRuntimeSplitSeen(modelID, model, seen)
		if err != nil {
			return err
		}
		wildcardOverlap, err := cc.sequenceHasWildcardEquivalentOverlap(model)
		if err != nil {
			return err
		}
		if needsSplit || wildcardOverlap {
			continue
		}
		if err := cc.checkDirectUPA(model); err != nil {
			return err
		}
	}
	return nil
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
		switch p.Kind {
		case runtime.ParticleElement:
			name, ok := c.rt.ElementName(p.Element)
			if !ok {
				return nil, xsderrors.InternalInvariant("element declaration consistency check references missing element name")
			}
			typ, ok := c.rt.ElementType(p.Element)
			if !ok {
				return nil, xsderrors.InternalInvariant("element declaration consistency check references missing element type")
			}
			if err := addElementDeclarationType(types, name, typ); err != nil {
				return nil, err
			}
		case runtime.ParticleModel:
			nested, err := c.collectModel(p.Model)
			if err != nil {
				return nil, err
			}
			for name, typ := range nested {
				if err := c.spend(1); err != nil {
					return nil, err
				}
				if err := addElementDeclarationType(types, name, typ); err != nil {
					return nil, err
				}
			}
		case runtime.ParticleWildcard:
		}
	}
	return types, nil
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
	if c.choiceNeedsRuntimeSplit(model, model.Occurs) {
		return true, nil
	}
	for _, p := range model.Particles {
		if err := c.work.spend(1); err != nil {
			return false, err
		}
		if p.Kind != runtime.ParticleModel {
			continue
		}
		child, ok := c.rt.ContentModel(p.Model)
		if !ok {
			continue
		}
		if c.choiceNeedsRuntimeSplit(child, p.Occurs) {
			return true, nil
		}
		needsSplit, err := c.modelNeedsRuntimeSplitSeen(p.Model, child, seen)
		if err != nil {
			return false, err
		}
		if needsSplit {
			return true, nil
		}
	}
	return false, nil
}

func (c *contentModelCompiler) choiceNeedsRuntimeSplit(model runtime.ContentModel, occurs runtime.Occurrence) bool {
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
	default:
		return nil
	}
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
		next := model.Particles[j]
		name, ok, overlapErr := c.particlesOverlap(candidate, next)
		if overlapErr != nil {
			return overlapErr
		}
		if !ok {
			if next.Occurs.Min > 0 {
				break
			}
			continue
		}
		if !model.Occurs.IsExactlyOne() && c.wildcardEquivalentOverlap(candidate, next) {
			continue
		}
		return c.upaError("UPA violation: duplicate element in sequence", name)
	}
	return nil
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
		overlaps, err := c.particleCanOverlapFollowing(p)
		if err != nil {
			return nil, err
		}
		if overlaps {
			return []runtime.Particle{p}, nil
		}
	case runtime.ParticleModel:
		model, ok := c.rt.ContentModel(p.Model)
		if !ok {
			return nil, nil
		}
		var out []runtime.Particle
		if p.Occurs.Unbounded || p.Occurs.Max > p.Occurs.Min {
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
		out = append(out, continuations...)
		return out, nil
	}
	return nil, nil
}

func (c *contentModelCompiler) modelContinuationParticles(model runtime.ContentModel) ([]runtime.Particle, error) {
	if err := c.work.spend(1); err != nil {
		return nil, err
	}
	var out []runtime.Particle
	if model.Occurs.Unbounded || model.Occurs.Max > model.Occurs.Min {
		start, err := c.modelStartParticles(model)
		if err != nil {
			return nil, err
		}
		if err := c.work.spend(len(start)); err != nil {
			return nil, err
		}
		out = append(out, start...)
	}
	switch model.Kind {
	case runtime.ModelSequence, runtime.ModelChoice:
		for _, p := range model.Particles {
			continuations, err := c.particleContinuationParticles(p)
			if err != nil {
				return nil, err
			}
			if err := c.work.spend(len(continuations)); err != nil {
				return nil, err
			}
			out = append(out, continuations...)
		}
	default:
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
		candidates, err := c.particleContinuationParticles(p)
		if err != nil {
			return false, err
		}
		for _, candidate := range candidates {
			for j := i + 1; j < len(model.Particles); j++ {
				if err := c.work.spend(1); err != nil {
					return false, err
				}
				if c.wildcardEquivalentOverlap(candidate, model.Particles[j]) {
					return true, nil
				}
				if model.Particles[j].Occurs.Min > 0 {
					break
				}
			}
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
	var out []runtime.Particle
	switch model.Kind {
	case runtime.ModelAll, runtime.ModelChoice:
		if err := c.work.spend(len(model.Particles)); err != nil {
			return nil, err
		}
		out = append(out, model.Particles...)
	case runtime.ModelSequence:
		for _, p := range model.Particles {
			if err := c.work.spend(1); err != nil {
				return nil, err
			}
			out = append(out, p)
			emptiable, err := c.analysis.ParticleEmptiable(p)
			if err != nil {
				return nil, err
			}
			if !emptiable {
				break
			}
		}
	default:
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
	default:
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
	default:
		return runtime.CompiledModel{}, false, nil
	}
}

func (c *contentModelCompiler) compileDirectSequenceModel(model runtime.ContentModel, limits []uint32) (runtime.CompiledModel, bool, error) {
	rows := []runtime.CompiledModelRow{{}}
	active := []uint32{0}
	for i, p := range model.Particles {
		if err := c.work.spend(1); err != nil {
			return runtime.CompiledModel{}, false, err
		}
		p = applyRepeatedChoiceLimit(p, i, limits)
		if p.Kind != runtime.ParticleElement && p.Kind != runtime.ParticleWildcard {
			return runtime.CompiledModel{}, false, nil
		}
		if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
			continue
		}
		edge := runtime.CompiledModelEdge{Particle: singleParticle(p)}
		if p.Occurs.IsExactlyOne() {
			to, err := checkedUint32(len(rows), "content model DFA state limit exceeded")
			if err != nil {
				return runtime.CompiledModel{}, false, err
			}
			rows, err = c.appendCompiledModelRow(rows, runtime.CompiledModelRow{})
			if err != nil {
				return runtime.CompiledModel{}, false, err
			}
			edge.To = to
			for _, state := range active {
				if err := c.work.spend(1); err != nil {
					return runtime.CompiledModel{}, false, err
				}
				rows[state].Edges = append(rows[state].Edges, edge)
			}
			active = []uint32{to}
			continue
		}
		to, err := checkedUint32(len(rows), "content model DFA state limit exceeded")
		if err != nil {
			return runtime.CompiledModel{}, false, err
		}
		rows, err = c.appendCompiledModelRow(rows, compiledParticleRow(edge.Particle, p.Occurs, compiledRowReject))
		if err != nil {
			return runtime.CompiledModel{}, false, err
		}
		edge.To = to
		for _, state := range active {
			if err := c.work.spend(1); err != nil {
				return runtime.CompiledModel{}, false, err
			}
			rows[state].Edges = append(rows[state].Edges, edge)
		}
		if p.Occurs.Unbounded || p.Occurs.Max > 1 {
			rows[to].Edges = append(rows[to].Edges, edge)
		}
		next := []uint32{to}
		if p.Occurs.Min == 0 {
			next = append(next, active...)
			slices.Sort(next)
			next = slices.Compact(next)
		}
		active = next
	}
	for _, state := range active {
		rows[state].Accept = true
	}
	if err := c.checkCompiledRowsUPA(rows); err != nil {
		return runtime.CompiledModel{}, false, err
	}
	return runtime.CompiledModel{
		Kind:  runtime.CompiledModelDFA,
		Rows:  rows,
		Start: 0,
		Mixed: model.Mixed,
		Empty: rows[0].Accept,
	}, true, nil
}

func (c *contentModelCompiler) compileDirectChoiceModel(model runtime.ContentModel) (runtime.CompiledModel, bool, error) {
	rows := []runtime.CompiledModelRow{{}}
	for _, p := range model.Particles {
		if err := c.work.spend(1); err != nil {
			return runtime.CompiledModel{}, false, err
		}
		if p.Kind != runtime.ParticleElement && p.Kind != runtime.ParticleWildcard {
			return runtime.CompiledModel{}, false, nil
		}
		if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
			rows[0].Accept = true
			continue
		}
		if p.Occurs.Min == 0 {
			rows[0].Accept = true
		}
		to, err := checkedUint32(len(rows), "content model DFA state limit exceeded")
		if err != nil {
			return runtime.CompiledModel{}, false, err
		}
		edge := runtime.CompiledModelEdge{Particle: singleParticle(p), To: to}
		if p.Occurs.IsExactlyOne() {
			rows, err = c.appendCompiledModelRow(rows, runtime.CompiledModelRow{Accept: true})
			if err != nil {
				return runtime.CompiledModel{}, false, err
			}
			rows[0].Edges = append(rows[0].Edges, edge)
			continue
		}
		rows, err = c.appendCompiledModelRow(rows, compiledParticleRow(edge.Particle, p.Occurs, compiledRowAccept))
		if err != nil {
			return runtime.CompiledModel{}, false, err
		}
		rows[0].Edges = append(rows[0].Edges, edge)
		if p.Occurs.Unbounded || p.Occurs.Max > 1 {
			rows[edge.To].Edges = append(rows[edge.To].Edges, edge)
		}
	}
	if err := c.checkCompiledRowsUPA(rows); err != nil {
		return runtime.CompiledModel{}, false, err
	}
	return runtime.CompiledModel{
		Kind:  runtime.CompiledModelDFA,
		Rows:  rows,
		Start: 0,
		Mixed: model.Mixed,
		Empty: rows[0].Accept,
	}, true, nil
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
		for i, a := range row.Edges {
			for j := i + 1; j < len(row.Edges); j++ {
				if err := c.work.spend(1); err != nil {
					return err
				}
				next := row.Edges[j]
				name, ok, overlapErr := c.particlesOverlap(a.Particle, next.Particle)
				if overlapErr != nil {
					return overlapErr
				}
				if !ok {
					continue
				}
				if runtime.CompiledCountingException(uint32(state), row, a, next) {
					continue
				}
				return c.upaError("UPA violation: overlapping particles", name)
			}
		}
	}
	return nil
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
		if particle.Kind != runtime.ParticleElement || c.rt.HasSubstitutionMembers(particle.Element) {
			return true, nil
		}
		name, ok := c.rt.ElementName(particle.Element)
		if !ok {
			return true, nil
		}
		for j := i + 1; j < count; j++ {
			if err := c.work.spend(1); err != nil {
				return false, err
			}
			next := particleAt(j)
			if next.Kind != runtime.ParticleElement || c.rt.HasSubstitutionMembers(next.Element) {
				return true, nil
			}
			nextName, ok := c.rt.ElementName(next.Element)
			if !ok || nextName == name {
				return true, nil
			}
		}
	}
	return false, nil
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
		for j := i + 1; j < len(particles); j++ {
			if err := c.work.spend(1); err != nil {
				return err
			}
			name, ok, overlapErr := c.particlesOverlap(p, particles[j])
			if overlapErr != nil {
				return overlapErr
			}
			if ok {
				return c.upaError(msg, name)
			}
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
	start := root.First
	if root.Nullable {
		if spendErr := b.c.work.spend(1); spendErr != nil {
			return runtime.CompiledModel{}, spendErr
		}
		start = append(start, dfaEntry{Pos: dfaEndPos})
	}
	for _, tail := range root.Last {
		if spendErr := b.c.work.spend(len(tail.Guards) + len(tail.Actions) + 1); spendErr != nil {
			return runtime.CompiledModel{}, spendErr
		}
		b.appendFollow(tail.Pos, dfaEntry{
			Pos:     dfaEndPos,
			Guards:  slices.Clone(tail.Guards),
			Actions: slices.Clone(tail.Actions),
		})
	}
	if normalizeErr := b.normalizeFollows(); normalizeErr != nil {
		return runtime.CompiledModel{}, normalizeErr
	}
	startID, err := b.stateID(start)
	if err != nil {
		return runtime.CompiledModel{}, err
	}
	for len(b.queue) != 0 {
		entries := b.queue[0]
		b.queue[0] = nil
		b.queue = b.queue[1:]
		if err := b.c.work.spend(1); err != nil {
			return runtime.CompiledModel{}, err
		}
		row, err := b.row(entries)
		if err != nil {
			return runtime.CompiledModel{}, err
		}
		b.rows = append(b.rows, row)
	}
	if err := b.checkUPA(); err != nil {
		return runtime.CompiledModel{}, err
	}
	return b.compileDeterministicModel(id, startID)
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
	var node dfaNode
	switch model.Kind {
	case runtime.ModelEmpty:
		node.Nullable = true
	case runtime.ModelSequence:
		node = dfaNode{Nullable: true}
		for i, p := range model.Particles {
			child, err := b.particleNode(p, i, scope)
			if err != nil {
				return dfaNode{}, err
			}
			if spendErr := b.c.work.spend(len(node.First) + len(node.Last) + len(child.First) + len(child.Last)); spendErr != nil {
				return dfaNode{}, spendErr
			}
			var concatErr error
			node, concatErr = b.concat(node, child)
			if concatErr != nil {
				return dfaNode{}, concatErr
			}
		}
	case runtime.ModelChoice:
		for i, p := range model.Particles {
			child, err := b.particleNode(p, i, scope)
			if err != nil {
				return dfaNode{}, err
			}
			if err := b.c.work.spend(len(node.First) + len(node.Last) + len(child.First) + len(child.Last)); err != nil {
				return dfaNode{}, err
			}
			node = b.choice(node, child)
		}
	case runtime.ModelAll:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "xs:all cannot be nested in DFA content models")
	default:
		return dfaNode{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported content model")
	}
	return b.repeat(node, model.Occurs, -1)
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

func (b *dfaBuilder) choice(a, c dfaNode) dfaNode {
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
		if slot < 0 {
			return child, nil
		}
		slotID, err := checkedUint32(slot, "content model counter limit exceeded")
		if err != nil {
			return dfaNode{}, err
		}
		return b.countNode(child, slotID)
	}
	if err := b.c.work.spend(len(child.First) + len(child.Last) + len(child.Counters)); err != nil {
		return dfaNode{}, err
	}
	if slot < 0 && repeatNeedsCounter(occurs) {
		slot = int(b.newCounter())
	}
	loop := occurs.Unbounded || occurs.Max > 1
	self := ^uint32(0)
	counted := slot >= 0
	if counted {
		slotID, err := checkedUint32(slot, "content model counter limit exceeded")
		if err != nil {
			return dfaNode{}, err
		}
		self = slotID
	}
	last, err := b.repeatLastEntries(child, occurs, self, counted)
	if err != nil {
		return dfaNode{}, err
	}
	if loop {
		if err := b.addRepeatLoopFollows(child, occurs, self, counted); err != nil {
			return dfaNode{}, err
		}
	}
	counters := slices.Clone(child.Counters)
	if counted && !slices.Contains(counters, self) {
		counters = append(counters, self)
		slices.Sort(counters)
	}
	return dfaNode{
		First:    child.First,
		Last:     normalizeDFAEntries(last),
		Counters: counters,
		Nullable: occurs.Min == 0 || child.Nullable,
	}, nil
}

func (b *dfaBuilder) repeatLastEntries(child dfaNode, occurs runtime.Occurrence, self uint32, counted bool) ([]dfaEntry, error) {
	var exitGuards []compiledGuard
	var exitActions []compiledAction
	if counted {
		if occurs.Min > 0 && !child.Nullable {
			exitGuards = append(exitGuards, compiledGuard{Slot: self, N: occurs.Min, Kind: compiledGuardExitMin})
		}
		exitActions = append(exitActions, compiledAction{Slot: self, Kind: compiledActionInc})
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

func (b *dfaBuilder) addRepeatLoopFollows(child dfaNode, occurs runtime.Occurrence, self uint32, counted bool) error {
	for _, tail := range child.Last {
		for _, first := range child.First {
			extraGuards := 0
			if counted && !occurs.Unbounded {
				extraGuards = 1
			}
			extraActions := len(child.Counters)
			if counted {
				extraActions++
				if slices.Contains(child.Counters, self) {
					extraActions--
				}
			}
			if err := b.c.work.spend(
				len(tail.Guards) + extraGuards + len(tail.Actions) + extraActions +
					len(first.Guards) + len(first.Actions) + 1,
			); err != nil {
				return err
			}
			guards := slices.Clone(tail.Guards)
			if counted && !occurs.Unbounded {
				guards = append(guards, compiledGuard{Slot: self, N: occurs.Max, Kind: compiledGuardLoopMax})
			}
			actions := slices.Clone(tail.Actions)
			if counted {
				actions = append(actions, compiledAction{Slot: self, Kind: compiledActionInc})
			}
			actions = append(actions, resetActions(child.Counters, self)...)
			b.appendFollow(tail.Pos, composeEntry(guards, actions, first))
		}
	}
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
		for i, a := range row.Edges {
			for j := i + 1; j < len(row.Edges); j++ {
				if err := b.c.work.spend(1); err != nil {
					return err
				}
				next := row.Edges[j]
				if a.Pos == next.Pos {
					continue
				}
				name, ok, overlapErr := b.c.particlesOverlap(a.Particle, next.Particle)
				if overlapErr != nil {
					return overlapErr
				}
				if !ok {
					continue
				}
				if err := b.c.work.spendProduct(len(a.Guards), len(next.Guards)); err != nil {
					return err
				}
				if err := b.c.work.spendProduct(len(next.Guards), len(a.Guards)); err != nil {
					return err
				}
				if countingException(a, next) {
					continue
				}
				return b.c.upaError("UPA violation: overlapping particles", name)
			}
		}
	}
	return nil
}

func (c *contentModelCompiler) sourceRowNeedsUPACheck(row dfaSourceRow) (bool, error) {
	return c.particleSetNeedsUPACheck(len(row.Edges), func(index int) runtime.Particle {
		return row.Edges[index].Particle
	})
}
