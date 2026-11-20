package schema

import (
	"errors"
	"slices"
)

// continuationSets holds the particles of one particle that compete with the
// particles after it in a sequence (cos-nonambig), as two shared slices: the
// start particles of its group when the particle repeats, and the
// continuation set of its model or, for a repeated element or wildcard, the
// particle itself. Callers must not write either slice.
type continuationSets [2][]contentParticle

// ParticleContinuations returns the continuation sets of particles[i].
func (a *contentModelAnalysis) ParticleContinuations(particles []contentParticle, i int) (continuationSets, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return continuationSets{}, err
	}
	p := particles[i]
	switch {
	case p.Occurs.IsAbsent():
		return continuationSets{}, nil
	case p.Kind != particleModel:
		return a.leafContinuations(particles[i : i+1])
	}
	return a.groupContinuations(p)
}

func (a *contentModelAnalysis) leafContinuations(leaf []contentParticle) (continuationSets, error) {
	r, err := a.ParticleCountRange(leaf[0])
	if err != nil || !countsRepeat(r.counts) {
		return continuationSets{}, err
	}
	return continuationSets{nil, leaf}, nil
}

func (a *contentModelAnalysis) groupContinuations(p contentParticle) (continuationSets, error) {
	var sets continuationSets
	if countsRepeat(p.Occurs) {
		model, ok := a.contentModel(p.Model)
		if !ok {
			return continuationSets{}, errors.New("content model continuation analysis references missing content model")
		}
		start, err := a.startParticles(model)
		if err != nil {
			return continuationSets{}, err
		}
		sets[0] = start
	}
	continuations, err := a.ModelContinuations(p.Model)
	sets[1] = continuations
	return sets, err
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
		return nil, errors.New("content model continuation analysis contains a cycle")
	}
	if err := spendContentModelWork(a.work); err != nil {
		return nil, err
	}
	model, ok := a.contentModel(id)
	if !ok {
		return nil, errors.New("content model continuation analysis references missing content model")
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
