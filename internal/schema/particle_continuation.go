package schema

import (
	"slices"
)

// continuationSets lists the particles inside one sequence member that can
// meet the members after it, which Unique Particle Attribution
// (cos-nonambig) checks. Both slices are shared with the analysis cache, so
// callers must not write either slice.
type continuationSets [2][]contentParticle

// continuationView selects the particles of a member's continuation sets a
// check reads: all of them for Unique Particle Attribution, or only the
// wildcards for the wildcard-overlap pass. Both views follow one rule for
// which sets a member has, so the two checks cannot disagree on them.
type continuationView uint8

const (
	allContinuations continuationView = iota
	wildcardContinuations
)

// ParticleContinuations returns the continuation sets of particles[i].
func (a *contentModelAnalysis) ParticleContinuations(particles []contentParticle, i int) (continuationSets, error) {
	return a.memberContinuations(particles, i, allContinuations)
}

// ParticleWildcardContinuations returns the wildcards among the continuation
// sets of particles[i]. A group's wildcards are filtered from each of its sets
// once per model, so a caller reads and charges only wildcard candidates.
func (a *contentModelAnalysis) ParticleWildcardContinuations(particles []contentParticle, i int) (continuationSets, error) {
	return a.memberContinuations(particles, i, wildcardContinuations)
}

// memberContinuations returns the continuation sets of particles[i] in view:
// a repeated leaf continues itself, and a group continues with its start
// particles when it repeats and with its model's continuation set.
func (a *contentModelAnalysis) memberContinuations(particles []contentParticle, i int, view continuationView) (continuationSets, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return continuationSets{}, err
	}
	p := particles[i]
	switch {
	case p.Occurs.IsAbsent():
		return continuationSets{}, nil
	case p.Kind != particleModel && view == wildcardContinuations && p.Kind != particleWildcard:
		return continuationSets{}, nil
	case p.Kind != particleModel:
		return a.leafContinuations(particles[i : i+1])
	}
	var sets continuationSets
	if countsRepeat(p.Occurs) {
		start, err := a.groupStart(p.Model, view)
		if err != nil {
			return continuationSets{}, err
		}
		sets[0] = start
	}
	continuations, err := a.groupModelContinuations(p.Model, view)
	sets[1] = continuations
	return sets, err
}

func (a *contentModelAnalysis) leafContinuations(leaf []contentParticle) (continuationSets, error) {
	r, err := a.ParticleCountRange(leaf[0])
	if err != nil || !countsRepeat(r.counts) {
		return continuationSets{}, err
	}
	return continuationSets{nil, leaf}, nil
}

func (a *contentModelAnalysis) groupStart(id contentModelID, view continuationView) ([]contentParticle, error) {
	if cached := a.wildcards[id]; cached.startKnown && view == wildcardContinuations {
		return cached.start, nil
	}
	model, ok := a.contentModel(id)
	if !ok {
		return nil, newBuildDefect("content model continuation analysis references missing content model")
	}
	start, err := a.startParticles(model)
	if err != nil || view == allContinuations {
		return start, err
	}
	wildcards, err := a.wildcardsOf(start)
	if err != nil {
		return nil, err
	}
	cached := a.wildcards[id]
	cached.start, cached.startKnown = wildcards, true
	a.storeWildcards(id, cached)
	return wildcards, nil
}

func (a *contentModelAnalysis) groupModelContinuations(id contentModelID, view continuationView) ([]contentParticle, error) {
	if cached := a.wildcards[id]; cached.continuationsKnown && view == wildcardContinuations {
		return cached.continuations, nil
	}
	continuations, err := a.ModelContinuations(id)
	if err != nil || view == allContinuations {
		return continuations, err
	}
	wildcards, err := a.wildcardsOf(continuations)
	if err != nil {
		return nil, err
	}
	cached := a.wildcards[id]
	cached.continuations, cached.continuationsKnown = wildcards, true
	a.storeWildcards(id, cached)
	return wildcards, nil
}

func (a *contentModelAnalysis) wildcardsOf(set []contentParticle) ([]contentParticle, error) {
	if err := a.work(len(set)); err != nil {
		return nil, err
	}
	var wildcards []contentParticle
	for _, p := range set {
		if p.Kind == particleWildcard {
			wildcards = append(wildcards, p)
		}
	}
	return wildcards, nil
}

// modelWildcards holds the wildcards among a model's start particles and
// among its continuation set, each filtered once.
type modelWildcards struct {
	start, continuations           []contentParticle
	startKnown, continuationsKnown bool
}

// storeWildcards records the wildcards of model id. The cache is allocated on
// the first store, so an analysis that never runs the wildcard-overlap pass
// allocates none.
func (a *contentModelAnalysis) storeWildcards(id contentModelID, wildcards modelWildcards) {
	if a.wildcards == nil {
		a.wildcards = make(map[contentModelID]modelWildcards)
	}
	a.wildcards[id] = wildcards
}

// ModelContinuations returns the continuation set of the model id: its start
// particles when the model repeats, and the continuations of every particle
// that can match its last element. The set is a fact of the model, computed
// once and shared by every check and reference that reads it; it holds each
// particle once, in the order of its first occurrence, so its size is bounded
// by the distinct particles the model reaches, not by its paths. Callers must
// not write the slice.
func (a *contentModelAnalysis) ModelContinuations(id contentModelID) ([]contentParticle, error) {
	if set, ok := a.continuations[id]; ok {
		return set, nil
	}
	if a.continuationPath[id] {
		return nil, newBuildDefect("content model continuation analysis contains a cycle")
	}
	if err := spendContentModelWork(a.work); err != nil {
		return nil, err
	}
	model, ok := a.contentModel(id)
	if !ok {
		return nil, newBuildDefect("content model continuation analysis references missing content model")
	}
	a.continuationPath[id] = true
	defer delete(a.continuationPath, id)
	set, err := a.continuationsOf(model)
	if err != nil {
		return nil, err
	}
	a.continuations[id] = set
	return set, nil
}

func (a *contentModelAnalysis) continuationsOf(model contentModelNode) ([]contentParticle, error) {
	last, err := a.lastParticles(model)
	if err != nil {
		return nil, err
	}
	if err = a.nestedContinuations(last); err != nil {
		return nil, err
	}
	merge := continuationMerge{analysis: a}
	defer merge.release()
	start, err := a.repeatedStart(model)
	if err != nil {
		return nil, err
	}
	if err = merge.add(start); err != nil {
		return nil, err
	}
	for i := range last {
		sets, err := a.ParticleContinuations(last, i)
		if err != nil {
			return nil, err
		}
		if err := merge.addSets(sets); err != nil {
			return nil, err
		}
	}
	return merge.set(), nil
}

// nestedContinuations computes the continuation sets of the groups among
// particles before a merge reads them, so the merge owns the shared index for
// its whole duration.
func (a *contentModelAnalysis) nestedContinuations(particles []contentParticle) error {
	for _, p := range particles {
		if p.Kind != particleModel || p.Occurs.IsAbsent() {
			continue
		}
		if _, err := a.ModelContinuations(p.Model); err != nil {
			return err
		}
	}
	return nil
}

// lastParticles returns the particles of model that can match its last
// element. In a sequence an element matched before its last non-emptiable
// particle is always followed by one that particle matches, so only that
// particle and the ones after it can.
func (a *contentModelAnalysis) lastParticles(model contentModelNode) ([]contentParticle, error) {
	switch model.Kind {
	case modelChoice:
		return model.Particles, nil
	case modelSequence:
		for i, p := range slices.Backward(model.Particles) {
			emptiable, err := a.cachedParticleEmptiable(p)
			if err != nil {
				return nil, err
			}
			if !emptiable {
				return model.Particles[i:], nil
			}
		}
		return model.Particles, nil
	case modelEmpty, modelAll:
	}
	return nil, nil
}

// repeatedStart returns the start particles of model when the model repeats.
func (a *contentModelAnalysis) repeatedStart(model contentModelNode) ([]contentParticle, error) {
	if !countsRepeat(model.Occurs) {
		return nil, nil
	}
	return a.startParticles(model)
}

// startParticles returns the particles of model that can match its first
// element, as a shared prefix of its particles.
func (a *contentModelAnalysis) startParticles(model contentModelNode) ([]contentParticle, error) {
	switch model.Kind {
	case modelAll, modelChoice:
		return model.Particles, a.work(len(model.Particles))
	case modelSequence:
		return a.sequenceStart(model.Particles)
	case modelEmpty:
	}
	return nil, nil
}

func (a *contentModelAnalysis) sequenceStart(particles []contentParticle) ([]contentParticle, error) {
	for i, p := range particles {
		if err := spendContentModelWork(a.work); err != nil {
			return nil, err
		}
		emptiable, err := a.cachedParticleEmptiable(p)
		if err != nil {
			return nil, err
		}
		if !emptiable {
			return particles[:i+1], nil
		}
	}
	return particles, nil
}

func countsRepeat(o occurrence) bool {
	return o.Unbounded || o.Max > o.Min
}

// continuationMerge builds one continuation set from the shared sets it is
// given. A set that is the only nonempty one is kept as it is; otherwise the
// particles are copied once each through the analysis-owned index, and release
// empties the index of the particles the merge added.
type continuationMerge struct {
	analysis *contentModelAnalysis
	shared   []contentParticle
	merged   []contentParticle
	copying  bool
}

func (m *continuationMerge) addSets(sets continuationSets) error {
	for _, set := range sets {
		if err := m.add(set); err != nil {
			return err
		}
	}
	return nil
}

func (m *continuationMerge) add(set []contentParticle) error {
	if len(set) == 0 {
		return nil
	}
	if !m.copying && m.shared == nil {
		m.shared = set
		return nil
	}
	if !m.copying {
		m.copying = true
		if err := m.copy(m.shared); err != nil {
			return err
		}
		m.shared = nil
	}
	return m.copy(set)
}

func (m *continuationMerge) copy(set []contentParticle) error {
	if err := m.analysis.work(len(set)); err != nil {
		return err
	}
	seen := m.analysis.continuationIndex
	for _, p := range set {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		m.merged = append(m.merged, p)
	}
	return nil
}

func (m *continuationMerge) set() []contentParticle {
	if m.copying {
		return m.merged
	}
	return m.shared
}

func (m *continuationMerge) release() {
	for _, p := range m.merged {
		delete(m.analysis.continuationIndex, p)
	}
}
