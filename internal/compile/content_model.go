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

type contentModelCompiler struct {
	names                 *runtime.NameTable
	rt                    runtime.CompiledModelRuntime
	substitutions         map[runtime.ElementID][]runtime.ElementID
	models                []runtime.ContentModel
	wildcards             []runtime.Wildcard
	maxContentModelStates int
	directBuildReads      bool
}

func newContentModelCompiler(names *runtime.NameTable, rt runtime.CompiledModelRuntime, maxContentModelStates int) contentModelCompiler {
	c := contentModelCompiler{names: names, rt: rt, maxContentModelStates: maxContentModelStates}
	if build, ok := rt.(*runtime.SchemaBuild); ok {
		c.models = build.Models
		c.wildcards = build.Wildcards
		c.substitutions = build.Substitutions
		c.directBuildReads = true
	}
	return c
}

func (c *contentModelCompiler) ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	return c.contentModel(id)
}

func (c *contentModelCompiler) ElementName(id runtime.ElementID) (runtime.QName, bool) {
	return c.rt.ElementName(id)
}

func (c *contentModelCompiler) Wildcard(id runtime.WildcardID) (runtime.Wildcard, bool) {
	if c.wildcards != nil {
		if !runtime.ValidWildcardID(id, len(c.wildcards)) {
			return runtime.Wildcard{}, false
		}
		return c.wildcards[id], true
	}
	return c.rt.Wildcard(id)
}

func (c *contentModelCompiler) ForEachSubstitutionMember(id runtime.ElementID, fn func(runtime.ElementID) bool) {
	c.rt.ForEachSubstitutionMember(id, fn)
}

func (c *contentModelCompiler) SubstitutionMemberByName(id runtime.ElementID, name runtime.QName) (runtime.ElementID, bool) {
	return c.rt.SubstitutionMemberByName(id, name)
}

func (c *contentModelCompiler) contentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	if c.models != nil {
		if !runtime.ValidContentModelID(id, len(c.models)) {
			return runtime.ContentModel{}, false
		}
		return c.models[id], true
	}
	return c.rt.ContentModel(id)
}

func (c *contentModelCompiler) AnyTypeID() runtime.ComplexTypeID {
	if rt, ok := c.rt.(runtime.TypeDerivationRuntime); ok {
		return rt.AnyTypeID()
	}
	return runtime.NoComplexType
}

func (c *contentModelCompiler) ComplexTypeCount() int {
	if rt, ok := c.rt.(runtime.TypeDerivationRuntime); ok {
		return rt.ComplexTypeCount()
	}
	return 0
}

func (c *contentModelCompiler) SimpleTypeDerivation(id runtime.SimpleTypeID) (runtime.SimpleTypeDerivation, bool) {
	if rt, ok := c.rt.(runtime.TypeDerivationRuntime); ok {
		return rt.SimpleTypeDerivation(id)
	}
	return runtime.SimpleTypeDerivation{}, false
}

func (c *contentModelCompiler) ComplexTypeDerivation(id runtime.ComplexTypeID) (runtime.ComplexTypeDerivation, bool) {
	if rt, ok := c.rt.(runtime.TypeDerivationRuntime); ok {
		return rt.ComplexTypeDerivation(id)
	}
	return runtime.ComplexTypeDerivation{}, false
}

func (c *contentModelCompiler) ElementRestriction(id runtime.ElementID) (runtime.ParticleRestrictionElement, bool) {
	if rt, ok := c.rt.(runtime.ParticleRestrictionRuntime); ok {
		return rt.ElementRestriction(id)
	}
	return runtime.ParticleRestrictionElement{}, false
}

// ElementDeclarationRuntime supplies model and element metadata for compile-time
// model declaration consistency checks.
type ElementDeclarationRuntime interface {
	ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool)
	ElementName(id runtime.ElementID) (runtime.QName, bool)
	ElementType(id runtime.ElementID) (runtime.TypeID, bool)
}

type elementDeclarationModelRuntime struct {
	rt     ElementDeclarationRuntime
	models []runtime.ContentModel
}

func (r elementDeclarationModelRuntime) ContentModel(id runtime.ContentModelID) (runtime.ContentModel, bool) {
	if r.models != nil {
		if !runtime.ValidContentModelID(id, len(r.models)) {
			return runtime.ContentModel{}, false
		}
		return r.models[id], true
	}
	return r.rt.ContentModel(id)
}

func (r elementDeclarationModelRuntime) ElementName(id runtime.ElementID) (runtime.QName, bool) {
	return r.rt.ElementName(id)
}

func (r elementDeclarationModelRuntime) ElementType(id runtime.ElementID) (runtime.TypeID, bool) {
	return r.rt.ElementType(id)
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

// CompileContentModels compiles every runtime content model into its validation
// representation.
func CompileContentModels(
	names *runtime.NameTable,
	rt runtime.CompiledModelRuntime,
	count int,
	maxContentModelStates int,
) ([]runtime.CompiledModel, error) {
	cc := newContentModelCompiler(names, rt, maxContentModelStates)
	compiled := make([]runtime.CompiledModel, count)
	for id := range count {
		m, err := cc.compileContentModel(runtime.ContentModelID(id))
		if err != nil {
			return nil, err
		}
		m.Source = runtime.ContentModelID(id)
		if err := runtime.IndexCompiledModelRows(rt, &m); err != nil {
			return nil, xsderrors.InternalInvariant(err.Error())
		}
		compiled[id] = m
	}
	return compiled, nil
}

// CheckContentModelsUPA validates direct unique-particle-attribution checks
// that can be proven before compiled DFA construction.
func CheckContentModelsUPA(
	names *runtime.NameTable,
	rt runtime.CompiledModelRuntime,
	count int,
) error {
	cc := newContentModelCompiler(names, rt, 0)
	seen := make([]bool, count)
	for id := range count {
		modelID := runtime.ContentModelID(id)
		model, ok := cc.contentModel(modelID)
		if !ok {
			return xsderrors.InternalInvariant("UPA check references missing content model")
		}
		clear(seen)
		if cc.modelNeedsRuntimeSplitSeen(modelID, model, seen) || cc.sequenceHasWildcardEquivalentOverlap(model) {
			continue
		}
		if err := cc.checkDirectUPA(model); err != nil {
			return err
		}
	}
	return nil
}

// CheckContentModelElementDeclarationsConsistent validates element-declaration
// consistency for every compiled content model.
func CheckContentModelElementDeclarationsConsistent(rt ElementDeclarationRuntime, count int) error {
	modelRT := elementDeclarationModelRuntime{rt: rt}
	if build, ok := rt.(*runtime.SchemaBuild); ok {
		modelRT.models = build.Models
	}
	for id := range count {
		model, ok := modelRT.ContentModel(runtime.ContentModelID(id))
		if !ok {
			return xsderrors.InternalInvariant("element declaration consistency check references missing content model")
		}
		if err := CheckElementDeclarationsConsistent(modelRT, model); err != nil {
			return err
		}
	}
	return nil
}

// CheckElementDeclarationsConsistent rejects content models that expose one
// element name with multiple element types.
func CheckElementDeclarationsConsistent(rt ElementDeclarationRuntime, model runtime.ContentModel) error {
	var types elementDeclarationTypes
	return collectElementDeclarationType(rt, &types, model.Particles)
}

type elementDeclarationTypes struct {
	seen      map[runtime.QName]runtime.TypeID
	firstName runtime.QName
	firstType runtime.TypeID
	hasFirst  bool
}

func (s *elementDeclarationTypes) add(name runtime.QName, typ runtime.TypeID) error {
	if !s.hasFirst {
		s.firstName = name
		s.firstType = typ
		s.hasFirst = true
		return nil
	}
	if s.seen == nil {
		if name == s.firstName {
			if s.firstType != typ {
				return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations with the same name must have the same type")
			}
			return nil
		}
		s.seen = map[runtime.QName]runtime.TypeID{
			s.firstName: s.firstType,
			name:        typ,
		}
		return nil
	}
	if prev, ok := s.seen[name]; ok && prev != typ {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, "element declarations with the same name must have the same type")
	}
	s.seen[name] = typ
	return nil
}

func collectElementDeclarationType(rt ElementDeclarationRuntime, types *elementDeclarationTypes, particles []runtime.Particle) error {
	for _, p := range particles {
		switch p.Kind {
		case runtime.ParticleElement:
			name, ok := rt.ElementName(p.Element)
			if !ok {
				return xsderrors.InternalInvariant("element declaration consistency check references missing element name")
			}
			typ, ok := rt.ElementType(p.Element)
			if !ok {
				return xsderrors.InternalInvariant("element declaration consistency check references missing element type")
			}
			if err := types.add(name, typ); err != nil {
				return err
			}
		case runtime.ParticleModel:
			model, ok := rt.ContentModel(p.Model)
			if !ok {
				return xsderrors.InternalInvariant("element declaration consistency check references missing content model")
			}
			if err := collectElementDeclarationType(rt, types, model.Particles); err != nil {
				return err
			}
		case runtime.ParticleWildcard:
		}
	}
	return nil
}

func (c *contentModelCompiler) modelNeedsRuntimeSplitSeen(id runtime.ContentModelID, model runtime.ContentModel, seen []bool) bool {
	if runtime.ValidUint32Index(uint32(id), len(seen)) {
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	if c.choiceNeedsRuntimeSplit(model, model.Occurs) {
		return true
	}
	for _, p := range model.Particles {
		if p.Kind != runtime.ParticleModel {
			continue
		}
		child, ok := c.contentModel(p.Model)
		if !ok {
			continue
		}
		if c.choiceNeedsRuntimeSplit(child, p.Occurs) || c.modelNeedsRuntimeSplitSeen(p.Model, child, seen) {
			return true
		}
	}
	return false
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
		for _, candidate := range c.particleContinuationParticles(p) {
			if err := c.checkSequenceContinuationUPA(model, candidate, i+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *contentModelCompiler) checkSequenceContinuationUPA(model runtime.ContentModel, candidate runtime.Particle, start int) error {
	for j := start; j < len(model.Particles); j++ {
		next := model.Particles[j]
		name, ok := c.particlesOverlap(candidate, next)
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

func (c *contentModelCompiler) particleContinuationParticles(p runtime.Particle) []runtime.Particle {
	if p.Occurs.Max == 0 && !p.Occurs.Unbounded {
		return nil
	}
	switch p.Kind {
	case runtime.ParticleElement, runtime.ParticleWildcard:
		if c.particleCanOverlapFollowing(p) {
			return []runtime.Particle{p}
		}
	case runtime.ParticleModel:
		model, ok := c.contentModel(p.Model)
		if !ok {
			return nil
		}
		var out []runtime.Particle
		if p.Occurs.Unbounded || p.Occurs.Max > p.Occurs.Min {
			out = append(out, c.modelStartParticles(model)...)
		}
		out = append(out, c.modelContinuationParticles(model)...)
		return out
	}
	return nil
}

func (c *contentModelCompiler) modelContinuationParticles(model runtime.ContentModel) []runtime.Particle {
	var out []runtime.Particle
	if model.Occurs.Unbounded || model.Occurs.Max > model.Occurs.Min {
		out = append(out, c.modelStartParticles(model)...)
	}
	switch model.Kind {
	case runtime.ModelSequence, runtime.ModelChoice:
		for _, p := range model.Particles {
			out = append(out, c.particleContinuationParticles(p)...)
		}
	default:
	}
	return out
}

func (c *contentModelCompiler) particleCanOverlapFollowing(p runtime.Particle) bool {
	r := runtime.ParticleCountRange(c, p)
	return r.Unbounded || r.Max > r.Min
}

func (c *contentModelCompiler) sequenceHasWildcardEquivalentOverlap(model runtime.ContentModel) bool {
	if model.Kind != runtime.ModelSequence {
		return false
	}
	for i, p := range model.Particles {
		for _, candidate := range c.particleContinuationParticles(p) {
			for j := i + 1; j < len(model.Particles); j++ {
				if c.wildcardEquivalentOverlap(candidate, model.Particles[j]) {
					return true
				}
				if model.Particles[j].Occurs.Min > 0 {
					break
				}
			}
		}
	}
	return false
}

func (c *contentModelCompiler) wildcardEquivalentOverlap(a, b runtime.Particle) bool {
	if a.Kind != runtime.ParticleWildcard || b.Kind != runtime.ParticleWildcard {
		return false
	}
	wa, ok := c.Wildcard(a.Wildcard)
	if !ok {
		return false
	}
	wb, ok := c.Wildcard(b.Wildcard)
	if !ok {
		return false
	}
	return runtime.WildcardNamespaceEqual(wa, wb)
}

func (c *contentModelCompiler) modelStartParticles(model runtime.ContentModel) []runtime.Particle {
	var out []runtime.Particle
	switch model.Kind {
	case runtime.ModelAll, runtime.ModelChoice:
		out = append(out, model.Particles...)
	case runtime.ModelSequence:
		for _, p := range model.Particles {
			out = append(out, p)
			if !runtime.ParticleEmptiable(c, p) {
				break
			}
		}
	default:
	}
	return out
}

// ValidateCompiledModelDerived recompiles one content model and checks that the
// stored compiled representation is exactly derivable from the source model.
func ValidateCompiledModelDerived(
	names *runtime.NameTable,
	rt runtime.CompiledModelRuntime,
	id runtime.ContentModelID,
	model runtime.CompiledModel,
) error {
	c := newContentModelCompiler(names, rt, int(^uint(0)>>1))
	expected, err := c.compileContentModel(id)
	if err != nil {
		return xsderrors.InternalInvariant("compiled content model cannot be rederived from source model")
	}
	expected.Source = id
	if err := runtime.IndexCompiledModelRows(rt, &expected); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}
	if !runtime.EqualCompiledModels(model, expected) {
		return xsderrors.InternalInvariant("compiled content model does not match source model")
	}
	return nil
}

func (c *contentModelCompiler) compileContentModel(id runtime.ContentModelID) (runtime.CompiledModel, error) {
	model, ok := c.contentModel(id)
	if !ok {
		return runtime.CompiledModel{}, xsderrors.InternalInvariant("content model compiler references missing content model")
	}
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
		if !c.compiledRowNeedsUPACheck(row) {
			continue
		}
		for i, a := range row.Edges {
			for j := i + 1; j < len(row.Edges); j++ {
				next := row.Edges[j]
				name, ok := c.particlesOverlap(a.Particle, next.Particle)
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

func (c *contentModelCompiler) compiledRowNeedsUPACheck(row runtime.CompiledModelRow) bool {
	for i, edge := range row.Edges {
		if edge.Particle.Kind != runtime.ParticleElement || c.elementHasSubstitutionMembers(edge.Particle.Element) {
			return true
		}
		name, ok := c.rt.ElementName(edge.Particle.Element)
		if !ok {
			return true
		}
		for j := i + 1; j < len(row.Edges); j++ {
			next := row.Edges[j].Particle
			if next.Kind != runtime.ParticleElement || c.elementHasSubstitutionMembers(next.Element) {
				return true
			}
			nextName, ok := c.rt.ElementName(next.Element)
			if !ok || nextName == name {
				return true
			}
		}
	}
	return false
}

func (c *contentModelCompiler) elementHasSubstitutionMembers(id runtime.ElementID) bool {
	if c.directBuildReads {
		return len(c.substitutions[id]) != 0
	}
	found := false
	c.rt.ForEachSubstitutionMember(id, func(runtime.ElementID) bool {
		found = true
		return false
	})
	return found
}

func (c *contentModelCompiler) particlesNeedUPACheck(particles []runtime.Particle) bool {
	for i, particle := range particles {
		if particle.Kind != runtime.ParticleElement || c.elementHasSubstitutionMembers(particle.Element) {
			return true
		}
		name, ok := c.rt.ElementName(particle.Element)
		if !ok {
			return true
		}
		for j := i + 1; j < len(particles); j++ {
			next := particles[j]
			if next.Kind != runtime.ParticleElement || c.elementHasSubstitutionMembers(next.Element) {
				return true
			}
			nextName, ok := c.rt.ElementName(next.Element)
			if !ok || nextName == name {
				return true
			}
		}
	}
	return false
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
	if !c.particlesNeedUPACheck(particles) {
		return nil
	}
	for i, p := range particles {
		for j := i + 1; j < len(particles); j++ {
			name, ok := c.particlesOverlap(p, particles[j])
			if ok {
				return c.upaError(msg, name)
			}
		}
	}
	return nil
}

func (c *contentModelCompiler) particlesOverlap(a, b runtime.Particle) (runtime.QName, bool) {
	return runtime.ParticlesOverlap(c, a, b)
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
		start = append(start, dfaEntry{Pos: dfaEndPos})
	}
	for _, tail := range root.Last {
		b.addFollow(tail.Pos, dfaEntry{
			Pos:     dfaEndPos,
			Guards:  slices.Clone(tail.Guards),
			Actions: slices.Clone(tail.Actions),
		})
	}
	startID, err := b.stateID(start)
	if err != nil {
		return runtime.CompiledModel{}, err
	}
	for len(b.queue) != 0 {
		entries := b.queue[0]
		b.queue[0] = nil
		b.queue = b.queue[1:]
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

type choiceLimitScope uint8

const (
	choiceLimitRoot choiceLimitScope = iota
	choiceLimitNested
)

func (b *dfaBuilder) modelNode(id runtime.ContentModelID, scope choiceLimitScope) (dfaNode, error) {
	model, ok := b.c.contentModel(id)
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
			node = b.concat(node, child)
		}
	case runtime.ModelChoice:
		for i, p := range model.Particles {
			child, err := b.particleNode(p, i, scope)
			if err != nil {
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
	var node dfaNode
	if scope == choiceLimitRoot {
		p = applyRepeatedChoiceLimit(p, index, b.limits)
	}
	switch p.Kind {
	case runtime.ParticleElement, runtime.ParticleWildcard:
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

func (b *dfaBuilder) concat(a, c dfaNode) dfaNode {
	for _, tail := range a.Last {
		for _, first := range c.First {
			b.addFollow(tail.Pos, composeEntry(tail.Guards, tail.Actions, first))
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
	}
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
		return countNode(child, slotID), nil
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
	last := repeatLastEntries(child, occurs, self, counted)
	if loop {
		b.addRepeatLoopFollows(child, occurs, self, counted)
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

func repeatLastEntries(child dfaNode, occurs runtime.Occurrence, self uint32, counted bool) []dfaEntry {
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
		last = append(last, dfaEntry{
			Pos:     tail.Pos,
			Guards:  appendGuards(tail.Guards, exitGuards),
			Actions: appendActions(tail.Actions, exitActions),
		})
	}
	return last
}

func (b *dfaBuilder) addRepeatLoopFollows(child dfaNode, occurs runtime.Occurrence, self uint32, counted bool) {
	for _, tail := range child.Last {
		for _, first := range child.First {
			guards := slices.Clone(tail.Guards)
			if counted && !occurs.Unbounded {
				guards = append(guards, compiledGuard{Slot: self, N: occurs.Max, Kind: compiledGuardLoopMax})
			}
			actions := slices.Clone(tail.Actions)
			if counted {
				actions = append(actions, compiledAction{Slot: self, Kind: compiledActionInc})
			}
			actions = append(actions, resetActions(child.Counters, self)...)
			b.addFollow(tail.Pos, composeEntry(guards, actions, first))
		}
	}
}

func repeatNeedsCounter(occurs runtime.Occurrence) bool {
	if occurs.Min > 1 {
		return true
	}
	return !occurs.Unbounded && occurs.Max > 1
}

func countNode(child dfaNode, slot uint32) dfaNode {
	action := []compiledAction{{Slot: slot, Kind: compiledActionInc}}
	last := make([]dfaEntry, 0, len(child.Last))
	for _, tail := range child.Last {
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
	}
}

func (b *dfaBuilder) newCounter() uint32 {
	slot := b.counters
	b.counters++
	return slot
}

func (b *dfaBuilder) addFollow(pos int, entry dfaEntry) {
	b.follow[pos] = append(b.follow[pos], entry)
	b.follow[pos] = normalizeDFAEntries(b.follow[pos])
}

func (b *dfaBuilder) checkUPA() error {
	for _, row := range b.rows {
		if !b.c.sourceRowNeedsUPACheck(row) {
			continue
		}
		for i, a := range row.Edges {
			for j := i + 1; j < len(row.Edges); j++ {
				next := row.Edges[j]
				if a.Pos == next.Pos {
					continue
				}
				name, ok := b.c.particlesOverlap(a.Particle, next.Particle)
				if !ok {
					continue
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

func (c *contentModelCompiler) sourceRowNeedsUPACheck(row dfaSourceRow) bool {
	for i, edge := range row.Edges {
		if edge.Particle.Kind != runtime.ParticleElement || c.elementHasSubstitutionMembers(edge.Particle.Element) {
			return true
		}
		name, ok := c.rt.ElementName(edge.Particle.Element)
		if !ok {
			return true
		}
		for j := i + 1; j < len(row.Edges); j++ {
			next := row.Edges[j].Particle
			if next.Kind != runtime.ParticleElement || c.elementHasSubstitutionMembers(next.Element) {
				return true
			}
			nextName, ok := c.rt.ElementName(next.Element)
			if !ok || nextName == name {
				return true
			}
		}
	}
	return false
}
