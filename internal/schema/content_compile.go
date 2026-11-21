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

// dfaNode is the position-automaton summary of one particle. First holds
// ascending positions: only a tail carries guards and actions, which the
// follow entries composed from it apply to every first position. First, Last
// and Counters are normalized and may be shared with the nodes it was built
// from, so they are never modified in place.
type dfaNode struct {
	First    []int
	Last     []dfaEntry
	Counters []uint32
	Nullable bool
}

type dfaBuilder struct {
	c         *contentModelCompiler
	states    map[string]uint32
	positions []contentParticle
	rows      []dfaSourceRow
	// queue holds the keys of the states whose rows are not built yet.
	queue      []string
	rowEntries []dfaStateEntry
	key        []byte
	follow     followTable
	limit      int
	counters   uint32
}

// dfaStateID caches an ID assigned on first use. known distinguishes the
// valid ID zero from an unpopulated entry.
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

// compileRootContentModel lowers the source graph of one model in the table,
// constructs its finite automaton and determinizes it, charging every
// traversal, configuration and transition to the compiler's budget.
// indexCompiledModelsRows indexes the table once every model is compiled.
func (c *contentModelCompiler) compileRootContentModel(id contentModelID) (compiledModel, error) {
	if err := c.work.spend(1); err != nil {
		return compiledModel{}, err
	}
	return c.compileContentModel(id)
}

// contentModelUPACheck validates the direct unique-particle-attribution checks
// that can be proven before compiled DFA construction, one model of a table at
// a time. Verdicts are root-local: each root uses its own seen tag, so a
// root's verdict and the work of its own traversal are never shared across
// roots. The analysis facts it reads, such as emptiability and continuation
// sets, are computed once per model and charged to the root that first reads
// them.
type contentModelUPACheck struct {
	compiler contentModelCompiler
	seen     []uint64
}

func newContentModelUPACheck(
	names *nameTable,
	rt *schemaBuild,
	count int,
	work *workBudget,
	analysis *contentModelAnalysis,
) *contentModelUPACheck {
	return &contentModelUPACheck{
		compiler: newContentModelCompiler(names, rt, 0, work, analysis),
		seen:     make([]uint64, count),
	}
}

func (u *contentModelUPACheck) checkModel(id contentModelID) error {
	return u.compiler.checkContentModelUPA(id, u.seen, uint64(id)+1)
}

func (c *contentModelCompiler) checkContentModelUPA(id contentModelID, seen []uint64, tag uint64) error {
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
// check shared by compilation and publication. Distinct same-named element
// declarations, including effective transitive substitution members, must
// share one named global type; generated internal names never make an
// anonymous type named. One declaration reached through several particles,
// groups or substitution heads counts once (I-026 in
// docs/spec/INTERPRETATIONS.md). A single particle is visited once regardless
// of its occurrence range. Declarations are collected and merged in particle
// order, so work charging and the reported conflict do not depend on map
// iteration.
type elementDeclarationConsistencyChecker struct {
	rt   *schemaBuild
	work contentModelWork
	// models holds the collection state of each content model of rt, indexed
	// by contentModelID.
	models []collectedContentModel
	// scratch holds one empty name index per content model on the collection
	// stack, and depth counts the ones in use. A model empties its index by
	// deleting the names it added, so reusing an index costs a model its own
	// size, however many names an earlier model at that depth had.
	scratch []map[QName]ElementID
	depth   int
}

// namedElementDeclaration is the first element declaration EDC saw for a name
// in one content model.
type namedElementDeclaration struct {
	name    QName
	element ElementID
}

// elementDeclarationSet collects the first declaration EDC saw for each name
// in one content model: ordered lists them in particle order and first indexes
// them by name.
type elementDeclarationSet struct {
	first   map[QName]ElementID
	ordered []namedElementDeclaration
}

// collectedContentModel is the EDC collection state of one content model and,
// once visited, its declarations.
type collectedContentModel struct {
	declarations []namedElementDeclaration
	state        contentModelVisitState
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
		models: make([]collectedContentModel, len(rt.Models)),
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
) ([]namedElementDeclaration, error) {
	if err := c.spend(); err != nil {
		return nil, err
	}
	if !validContentModelID(id, len(c.models)) {
		return nil, xsderrors.InternalInvariant("element declaration consistency check references missing content model")
	}
	collected := &c.models[id]
	switch collected.state {
	case contentModelVisiting:
		return nil, xsderrors.InternalInvariant("element declaration consistency check references cyclic content model")
	case contentModelVisited:
		return collected.declarations, nil
	}
	collected.state = contentModelVisiting
	declarations, err := c.collectParticles(c.rt.Models[id].Particles)
	if err != nil {
		collected.state = 0
		return nil, err
	}
	*collected = collectedContentModel{state: contentModelVisited, declarations: declarations}
	return declarations, nil
}

func (c *elementDeclarationConsistencyChecker) collectParticles(
	particles []contentParticle,
) ([]namedElementDeclaration, error) {
	hint := c.particleDeclarationsHint(particles)
	if c.depth == len(c.scratch) {
		c.scratch = append(c.scratch, make(map[QName]ElementID, hint))
	}
	declarations := elementDeclarationSet{
		first:   c.scratch[c.depth],
		ordered: make([]namedElementDeclaration, 0, hint),
	}
	c.depth++
	defer func() {
		c.depth--
		for _, declaration := range declarations.ordered {
			delete(declarations.first, declaration.name)
		}
	}()
	for _, p := range particles {
		if err := c.spend(); err != nil {
			return nil, err
		}
		if err := c.collectParticle(&declarations, p); err != nil {
			return nil, err
		}
	}
	return declarations.ordered, nil
}

// particleDeclarationsHint sizes a model's declaration set from its element
// particles and the cached declarations of nested models already visited. It
// reads only the cache, so work charging and error order are those of the
// collection itself.
func (c *elementDeclarationConsistencyChecker) particleDeclarationsHint(particles []contentParticle) int {
	n := 0
	for _, p := range particles {
		if p.Kind == particleElement {
			n++
		} else if p.Kind == particleModel && validContentModelID(p.Model, len(c.models)) {
			n += len(c.models[p.Model].declarations)
		}
	}
	return n
}

func (c *elementDeclarationConsistencyChecker) collectParticle(declarations *elementDeclarationSet, particle contentParticle) error {
	switch particle.Kind {
	case particleElement:
		return c.collectElementParticle(declarations, particle.Element)
	case particleModel:
		return c.collectModelParticle(declarations, particle.Model)
	case particleWildcard:
		return nil
	default:
	}
	return nil
}

// collectElementParticle collects element and the members of its actual
// substitution group in expanded-name order (substitutionTable.ForEachMember),
// so the first conflict found, and the work spent before it, do not depend on
// the order names were interned.
func (c *elementDeclarationConsistencyChecker) collectElementParticle(declarations *elementDeclarationSet, element ElementID) error {
	if err := c.collectElementDeclaration(declarations, element); err != nil {
		return err
	}
	var err error
	c.rt.Substitutions.ForEachMember(element, func(member ElementID) bool {
		err = c.collectSubstitutionMember(declarations, element, member)
		return err == nil
	})
	return err
}

func (c *elementDeclarationConsistencyChecker) collectSubstitutionMember(declarations *elementDeclarationSet, head, member ElementID) error {
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
	return c.collectElementDeclaration(declarations, member)
}

func (c *elementDeclarationConsistencyChecker) collectElementDeclaration(declarations *elementDeclarationSet, element ElementID) error {
	name, ok := c.rt.ElementName(element)
	if !ok {
		return xsderrors.InternalInvariant("element declaration consistency check references missing element name")
	}
	return c.addElementDeclaration(declarations, namedElementDeclaration{name: name, element: element})
}

func (c *elementDeclarationConsistencyChecker) collectModelParticle(declarations *elementDeclarationSet, model contentModelID) error {
	nested, err := c.collectModel(model)
	if err != nil {
		return err
	}
	for _, declaration := range nested {
		if err := c.spend(); err != nil {
			return err
		}
		if err := c.addElementDeclaration(declarations, declaration); err != nil {
			return err
		}
	}
	return nil
}

func (c *elementDeclarationConsistencyChecker) addElementDeclaration(
	declarations *elementDeclarationSet,
	declaration namedElementDeclaration,
) error {
	previous, ok := declarations.first[declaration.name]
	if !ok {
		declarations.first[declaration.name] = declaration.element
		declarations.ordered = append(declarations.ordered, declaration)
		return nil
	}
	if previous == declaration.element {
		return nil
	}
	previousType, previousOK := elementTypeByID(c.rt.Elements, previous)
	typ, ok := elementTypeByID(c.rt.Elements, declaration.element)
	if !previousOK || !ok {
		return xsderrors.InternalInvariant("element declaration consistency check references missing element type")
	}
	if previousType != typ {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations with the same name must have the same type")
	}
	named, err := c.hasNamedType(typ)
	if err != nil {
		return err
	}
	if !named {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations with the same name must use a named top-level type")
	}
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

func (c *contentModelCompiler) modelNeedsRuntimeSplitSeen(id contentModelID, model contentModelNode, seen []uint64, tag uint64) (bool, error) {
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

func (c *contentModelCompiler) particleNeedsRuntimeSplit(particle contentParticle, seen []uint64, tag uint64) (bool, error) {
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
	case modelEmpty:
		return nil
	default:
	}
	return nil
}

func (c *contentModelCompiler) checkSequenceUPA(model contentModelNode) error {
	for i := range model.Particles {
		sets, err := c.analysis.ParticleContinuations(model.Particles, i)
		if err != nil {
			return err
		}
		for _, candidates := range sets {
			if err := c.checkCandidatesUPA(model, candidates, i+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *contentModelCompiler) checkCandidatesUPA(model contentModelNode, candidates []contentParticle, start int) error {
	if err := c.work.spend(len(candidates)); err != nil {
		return err
	}
	for _, candidate := range candidates {
		if err := c.checkSequenceContinuationUPA(model, candidate, start); err != nil {
			return err
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
		emptiable, err := c.analysis.ParticleEmptiable(next)
		return !emptiable, err
	}
	if !modelOccurs.IsExactlyOne() && c.wildcardEquivalentOverlap(candidate, next) {
		return false, nil
	}
	return false, c.upaError("UPA violation: duplicate element in sequence", name)
}

func (c *contentModelCompiler) sequenceHasWildcardEquivalentOverlap(model contentModelNode) (bool, error) {
	if model.Kind != modelSequence {
		return false, nil
	}
	for i := range model.Particles {
		sets, err := c.analysis.ParticleContinuations(model.Particles, i)
		if err != nil {
			return false, err
		}
		for _, candidates := range sets {
			overlap, err := c.candidatesHaveWildcardEquivalentOverlap(model.Particles, candidates, i+1)
			if err != nil || overlap {
				return overlap, err
			}
		}
	}
	return false, nil
}

func (c *contentModelCompiler) candidatesHaveWildcardEquivalentOverlap(particles, candidates []contentParticle, start int) (bool, error) {
	for _, candidate := range candidates {
		overlap, err := c.wildcardCandidateOverlaps(particles, candidate, start)
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
		emptiable, err := c.analysis.ParticleEmptiable(particles[i])
		if err != nil || !emptiable {
			return false, err
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

func (c *contentModelCompiler) compileContentModel(id contentModelID) (compiledModel, error) {
	model, ok := contentModelByID(c.rt.Models, id)
	if !ok {
		return compiledModel{}, xsderrors.InternalInvariant("content model compiler references missing content model")
	}
	model, err := c.executionModel(model)
	if err != nil {
		return compiledModel{}, err
	}
	if c.laxAnyContent(model) {
		return compiledModel{Kind: compiledModelAny, Empty: true}, nil
	}
	switch model.Kind {
	case modelEmpty:
		return compiledModel{Kind: compiledModelEmpty, Empty: true}, nil
	case modelAll:
		return c.compileAllModel(model)
	case modelSequence, modelChoice:
		if m, ok, err := c.compileDirectModel(model); ok || err != nil {
			return m, err
		}
		b := &dfaBuilder{
			c:      c,
			follow: newFollowTable(),
			states: make(map[string]uint32),
			limit:  c.maxContentModelStates,
		}
		return b.compile(model)
	default:
		return compiledModel{}, xsderrors.InternalInvariant("content model has unknown kind")
	}
}

// laxAnyContent reports whether model, an execution model, holds only the
// particle of xs:anyType's content, which compiledModelAny executes without a
// DFA. The decision reads the content, never the owning type: xs:anyType and
// every declaration of the same content share it.
func (c *contentModelCompiler) laxAnyContent(model contentModelNode) bool {
	return model.Kind == modelSequence && len(model.Particles) == 1 &&
		builtinAnyTypeContentParticleShape(c.rt.Wildcards, model.Particles[0])
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

// executionModel is the root that direct compilation and automaton
// construction compile for a declared model. The declared model keeps its
// groups for Particle Valid (Restriction) clause 2.2, and a shared group is
// never copied into the models that refer to it: execution reads the
// language-preserving splice of same-compositor 1..1 groups through
// executionParticles. A model whose splice is one particle becomes a model of
// that particle as executionParticle reduces it.
func (c *contentModelCompiler) executionModel(model contentModelNode) (contentModelNode, error) {
	if model.Kind != modelSequence && model.Kind != modelChoice {
		return model, nil
	}
	var first contentParticle
	count := 0
	err := c.executionParticles(model, func(p contentParticle) (bool, error) {
		first = p
		count++
		return count < 2, nil
	})
	if err != nil || count != 1 {
		return model, err
	}
	if first, err = c.executionParticle(first); err != nil {
		return model, err
	}
	single := contentModelNode{Kind: model.Kind, Occurs: model.Occurs, Mixed: model.Mixed, Particles: []contentParticle{first}}
	return normalizeSingleParticleModel(single), nil
}

func (c *contentModelCompiler) executionParticles(model contentModelNode, yield func(contentParticle) (bool, error)) error {
	_, err := c.yieldExecutionParticles(model, yield)
	return err
}

func (c *contentModelCompiler) yieldExecutionParticles(model contentModelNode, yield func(contentParticle) (bool, error)) (bool, error) {
	for _, p := range model.Particles {
		next, err := c.yieldExecutionParticle(model.Kind, p, yield)
		if err != nil || !next {
			return false, err
		}
	}
	return true, nil
}

func (c *contentModelCompiler) yieldExecutionParticle(parent modelKind, p contentParticle, yield func(contentParticle) (bool, error)) (bool, error) {
	if err := c.work.spend(1); err != nil {
		return false, err
	}
	child, ok, err := c.executionGroup(parent, p)
	if err != nil {
		return false, err
	}
	if ok {
		return c.yieldExecutionParticles(child, yield)
	}
	return yield(p)
}

// executionGroup returns the group p contributes to a parent of kind parent
// when its particles replace p there.
func (c *contentModelCompiler) executionGroup(parent modelKind, p contentParticle) (contentModelNode, bool, error) {
	if p.Kind != particleModel || !p.Occurs.IsExactlyOne() {
		return contentModelNode{}, false, nil
	}
	child, ok := contentModelByID(c.rt.Models, p.Model)
	if !ok {
		return contentModelNode{}, false, xsderrors.InternalInvariant("execution particle references missing content model")
	}
	return child, child.Occurs.IsExactlyOne() && child.Kind == parent, nil
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
	case modelEmpty, modelAll:
		return compiledModel{}, false, nil
	default:
	}
	return compiledModel{}, false, nil
}

func (c *contentModelCompiler) compileDirectSequenceModel(model contentModelNode) (compiledModel, bool, error) {
	compilation := directSequenceCompilation{compiler: c, rows: []compiledModelRow{{}}, spans: []directSequenceSpan{{}}}
	supported := true
	err := c.executionParticles(model, func(particle contentParticle) (bool, error) {
		var err error
		supported, err = compilation.addParticle(particle)
		return supported, err
	})
	if err != nil || !supported {
		return compiledModel{}, supported, err
	}
	rows := compilation.table()
	if err := c.checkCompiledRowsUPA(rows); err != nil {
		return compiledModel{}, false, err
	}
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  rows,
		Start: 0,
		Empty: rows[0].Accept,
	}, true, nil
}

// directSequenceCompilation compiles a sequence of element and wildcard
// particles. Each particle adds a row and one edge into it, which every
// active row takes: the rows from active, the first that no required
// particle has passed since it was added, through the last. A row's edges
// are therefore one run of edges, which table shares between the rows.
type directSequenceCompilation struct {
	compiler *contentModelCompiler
	rows     []compiledModelRow
	spans    []directSequenceSpan
	edges    []compiledModelEdge
	active   uint32
}

// directSequenceSpan is a row's run of edges. The run of an active row
// extends to the last edge.
type directSequenceSpan struct {
	start, end int
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
	if particle.Occurs.IsAbsent() {
		return true, nil
	}
	row := compiledModelRow{}
	if !particle.Occurs.IsExactlyOne() {
		row = compiledParticleRow(singleParticle(particle), particle.Occurs, compiledRowReject)
	}
	return true, b.addEdge(particle, row)
}

// addEdge adds row and the edge of particle into it, which every active row
// takes. row also takes it when particle repeats; the earlier rows stay
// active when particle is optional.
func (b *directSequenceCompilation) addEdge(particle contentParticle, row compiledModelRow) error {
	to, err := b.appendRow(row)
	if err != nil {
		return err
	}
	for range to - b.active {
		if err := b.compiler.work.spend(1); err != nil {
			return err
		}
	}
	edge := len(b.edges)
	b.edges = append(b.edges, compiledModelEdge{Particle: singleParticle(particle), To: to})
	start := edge + 1
	if particle.Occurs.Unbounded || particle.Occurs.Max > 1 {
		start = edge
	}
	b.spans = append(b.spans, directSequenceSpan{start: start})
	if particle.Occurs.Min > 0 {
		for state := b.active; state < to; state++ {
			b.spans[state].end = edge + 1
		}
		b.active = to
	}
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

// table returns the rows with the active rows accepting. Each row's Edges is
// its run of edges, shared and without spare capacity, or nil.
func (b *directSequenceCompilation) table() []compiledModelRow {
	for state := range b.rows {
		span := b.spans[state]
		if state >= int(b.active) {
			span.end = len(b.edges)
			b.rows[state].Accept = true
		}
		if span.start < span.end {
			b.rows[state].Edges = b.edges[span.start:span.end:span.end]
		}
	}
	return b.rows
}

func (c *contentModelCompiler) compileDirectChoiceModel(model contentModelNode) (compiledModel, bool, error) {
	compilation := directChoiceCompilation{compiler: c, rows: []compiledModelRow{{}}}
	supported := true
	err := c.executionParticles(model, func(particle contentParticle) (bool, error) {
		var err error
		supported, err = compilation.addParticle(particle)
		return supported, err
	})
	if err != nil || !supported {
		return compiledModel{}, supported, err
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
	if particle.Occurs.IsAbsent() {
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

func (b *dfaBuilder) compile(model contentModelNode) (compiledModel, error) {
	if err := b.c.work.spend(1); err != nil {
		return compiledModel{}, err
	}
	root, err := b.groupNode(model)
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
	return b.compileDeterministicModel(startID)
}

func (b *dfaBuilder) rootStartEntries(root dfaNode) ([]dfaStateEntry, error) {
	start := make([]dfaStateEntry, 0, len(root.First)+1)
	for _, pos := range root.First {
		start = append(start, dfaStateEntry{Pos: pos})
	}
	if !root.Nullable {
		return start, nil
	}
	if err := b.c.work.spend(1); err != nil {
		return nil, err
	}
	return append(start, dfaStateEntry{Pos: dfaEndPos}), nil
}

func (b *dfaBuilder) appendRootEndFollows(last []dfaEntry) error {
	if len(last) == 0 {
		return nil
	}
	end, err := b.follow.rootEndSet()
	if err != nil {
		return err
	}
	for _, tail := range last {
		if err := b.c.work.spend(len(tail.Guards) + len(tail.Actions) + 1); err != nil {
			return err
		}
		if err := b.follow.add(tail.Pos, tail.Guards, tail.Actions, end); err != nil {
			return err
		}
	}
	return nil
}

func (b *dfaBuilder) compileSourceRows() error {
	for len(b.queue) != 0 {
		key := b.queue[0]
		b.queue[0] = ""
		b.queue = b.queue[1:]
		if err := b.c.work.spend(1); err != nil {
			return err
		}
		b.rowEntries = decodeDFAStateKey(b.rowEntries[:0], key)
		row, err := b.row(b.rowEntries)
		if err != nil {
			return err
		}
		b.rows = appendDoubling(b.rows, row)
	}
	return nil
}

// row builds the source row of entries, which must not be the follow
// table's storage: resolving each edge's target materializes into it.
func (b *dfaBuilder) row(entries []dfaStateEntry) (dfaSourceRow, error) {
	row := newDFASourceRow(entries)
	for _, e := range entries {
		p := b.follow.prefixes[e.Prefix]
		if err := b.c.work.spend(p.len() + 1); err != nil {
			return dfaSourceRow{}, err
		}
		if e.Pos == dfaEndPos {
			row.Accept = append(row.Accept, dfaAccept{Guards: p.guards})
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
			Guards:   p.guards,
			Actions:  p.actions,
			To:       to,
			Pos:      e.Pos,
		})
	}
	return row, nil
}

func newDFASourceRow(entries []dfaStateEntry) dfaSourceRow {
	edges := 0
	for _, e := range entries {
		if e.Pos != dfaEndPos {
			edges++
		}
	}
	if edges == 0 {
		return dfaSourceRow{}
	}
	return dfaSourceRow{Edges: make([]dfaSourceEdge, 0, edges)}
}

func (b *dfaBuilder) stateID(entries []dfaStateEntry) (uint32, error) {
	if err := b.spendDFAEntries(entries); err != nil {
		return 0, err
	}
	return b.internState(b.follow.normalize(slices.Clone(entries)))
}

// followStateID charges the follow set before consulting the class cache,
// so state discovery order and limit-failure order do not depend on caching.
func (b *dfaBuilder) followStateID(pos int) (uint32, error) {
	class, err := b.follow.class(pos)
	if err != nil {
		return 0, err
	}
	if spendErr := b.c.work.spend(class.charge); spendErr != nil {
		return 0, spendErr
	}
	if class.state.known {
		return class.state.id, nil
	}
	id, err := b.internState(b.follow.classEntries(pos))
	if err != nil {
		return 0, err
	}
	class.state = dfaStateID{id: id, known: true}
	return id, nil
}

// internState returns the state of entries, which must be normalized,
// admitting it if it is new. A queued state keeps only its key.
func (b *dfaBuilder) internState(entries []dfaStateEntry) (uint32, error) {
	b.key = appendDFAStateKey(b.key[:0], entries)
	if id, ok := b.states[string(b.key)]; ok {
		return id, nil
	}
	if len(b.states) >= b.limit {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded")
	}
	id, err := checkedSchemaUint32(len(b.states), "content model DFA state limit exceeded")
	if err != nil {
		return 0, err
	}
	key := string(b.key)
	b.states[key] = id
	b.queue = append(b.queue, key)
	return id, nil
}

func (b *dfaBuilder) spendDFAEntries(entries []dfaStateEntry) error {
	if err := b.c.work.spend(len(entries) + 1); err != nil {
		return err
	}
	for _, entry := range entries {
		if err := b.c.work.spend(b.follow.prefixes[entry.Prefix].len()); err != nil {
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
	return b.groupNode(model)
}

func (b *dfaBuilder) groupNode(model contentModelNode) (dfaNode, error) {
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
	default:
		err := xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "unsupported content model")
		return dfaNode{}, err
	}
	return b.repeat(node, model.Occurs)
}

// sequenceNode and choiceNode charge each child the lengths of the
// accumulated and child entry sets. Leaves take positions in particle order,
// so a child's entries follow every earlier sibling's and the union of
// sibling sets is their concatenation.
func (b *dfaBuilder) sequenceNode(particles []contentParticle) (dfaNode, error) {
	node := dfaNode{Nullable: true}
	var first dfaSetUnion[int]
	var last dfaSetUnion[dfaEntry]
	for _, particle := range particles {
		child, err := b.particleNode(particle)
		if err != nil {
			return dfaNode{}, err
		}
		if spendErr := b.c.work.spend(len(node.First) + len(node.Last) + len(child.First) + len(child.Last)); spendErr != nil {
			return dfaNode{}, spendErr
		}
		if err := b.addFollows(node.Last, child.First); err != nil {
			return dfaNode{}, err
		}
		node = concatNode(node, child, &first, &last)
	}
	return node, nil
}

// concatNode returns the summary of node followed by child; first and last
// accumulate node's entry sets.
func concatNode(node, child dfaNode, first *dfaSetUnion[int], last *dfaSetUnion[dfaEntry]) dfaNode {
	if node.Nullable {
		node.First = first.add(child.First)
	}
	if child.Nullable {
		node.Last = last.add(child.Last)
	} else {
		node.Last = last.reset(child.Last)
	}
	node.Counters = mergeCounters(node.Counters, child.Counters)
	node.Nullable = node.Nullable && child.Nullable
	return node
}

func (b *dfaBuilder) choiceNode(particles []contentParticle) (dfaNode, error) {
	var node dfaNode
	var first dfaSetUnion[int]
	var last dfaSetUnion[dfaEntry]
	for _, particle := range particles {
		child, err := b.particleNode(particle)
		if err != nil {
			return dfaNode{}, err
		}
		if err := b.c.work.spend(len(node.First) + len(node.Last) + len(child.First) + len(child.Last)); err != nil {
			return dfaNode{}, err
		}
		node.First = first.add(child.First)
		node.Last = last.add(child.Last)
		node.Counters = mergeCounters(node.Counters, child.Counters)
		node.Nullable = node.Nullable || child.Nullable
	}
	return node, nil
}

// dfaSetUnion accumulates the union of normalized position or entry sets,
// each holding only positions above those of the sets before it. It shares
// the first set until a second joins it, then appends to a buffer it owns, so
// a shared set is never modified. The sets it returns have no spare capacity,
// so appending to one copies it instead of writing into the buffer.
type dfaSetUnion[E any] struct {
	elems []E
	owned bool
}

func (u *dfaSetUnion[E]) add(elems []E) []E {
	switch {
	case len(elems) == 0:
	case len(u.elems) == 0:
		u.elems = elems
		u.owned = false
	case !u.owned:
		u.elems = append(make([]E, 0, len(u.elems)+len(elems)), u.elems...)
		u.elems = append(u.elems, elems...)
		u.owned = true
	default:
		u.elems = append(u.elems, elems...)
	}
	return slices.Clip(u.elems)
}

func (u *dfaSetUnion[E]) reset(elems []E) []E {
	u.elems = elems
	u.owned = false
	return elems
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
	b.positions = appendDoubling(b.positions, p)
	b.follow.addPosition()
	sets := &dfaLeafSets{last: [1]dfaEntry{{Pos: pos}}, first: [1]int{pos}}
	return dfaNode{First: sets.first[:], Last: sets.last[:]}
}

type dfaLeafSets struct {
	last  [1]dfaEntry
	first [1]int
}

// addFollows makes every tail followed by every position of first.
func (b *dfaBuilder) addFollows(tails []dfaEntry, first []int) error {
	if len(tails) == 0 || len(first) == 0 {
		return nil
	}
	set, err := b.follow.addSet(first)
	if err != nil {
		return err
	}
	for _, tail := range tails {
		if err := b.spendFollowPairs(len(tail.Guards)+len(tail.Actions), set); err != nil {
			return err
		}
		if err := b.follow.add(tail.Pos, tail.Guards, tail.Actions, set); err != nil {
			return err
		}
	}
	return nil
}

// spendFollowPairs charges, for each position of set, one step plus prefix,
// the number of guards and actions the tail adds to that follow entry.
func (b *dfaBuilder) spendFollowPairs(prefix int, set uint32) error {
	return b.c.work.spendProduct(b.follow.setLen(set), prefix+1)
}

func (b *dfaBuilder) repeat(child dfaNode, occurs occurrence) (dfaNode, error) {
	if err := b.c.work.spend(1); err != nil {
		return dfaNode{}, err
	}
	if occurs.IsAbsent() {
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
	if len(child.Last) == 0 || len(child.First) == 0 {
		return nil
	}
	set, err := b.follow.addSet(child.First)
	if err != nil {
		return err
	}
	for _, tail := range child.Last {
		if err := b.addRepeatLoopFollow(tail, set, child.Counters, occurs, counter); err != nil {
			return err
		}
	}
	return nil
}

func (b *dfaBuilder) addRepeatLoopFollow(tail dfaEntry, set uint32, childCounters []uint32, occurs occurrence, counter dfaRepeatCounter) error {
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
	if err := b.spendFollowPairs(len(tail.Guards)+extraGuards+len(tail.Actions)+extraActions, set); err != nil {
		return err
	}
	guards := tail.Guards
	if counter.active() && !occurs.Unbounded {
		guards = appendGuards(guards, []compiledGuard{{Slot: counter.slot, N: occurs.Max, Kind: compiledGuardLoopMax}})
	}
	actions := slices.Clone(tail.Actions)
	if counter.active() {
		actions = append(actions, compiledAction{Slot: counter.slot, Kind: compiledActionInc})
	}
	actions = append(actions, resetActions(childCounters, counter.slot)...)
	return b.follow.add(tail.Pos, guards, actions, set)
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

// normalizeFollows charges normalizing every follow set; followStateID
// materializes each set when a row first reaches it.
func (b *dfaBuilder) normalizeFollows() error {
	return b.follow.chargeNormalization(b.c.work.spend)
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
