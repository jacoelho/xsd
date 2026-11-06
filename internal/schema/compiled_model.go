package schema

import (
	"errors"
)

// compiledModelKind identifies the runtime representation used for a compiled
// content model.
type compiledModelKind uint8

const (
	// compiledModelEmpty is the compiled representation for empty content.
	compiledModelEmpty compiledModelKind = iota
	// compiledModelAny is the compiled representation for xs:anyType content.
	compiledModelAny
	// compiledModelAll is the compiled representation for xs:all content.
	compiledModelAll
	// compiledModelDFA is the compiled DFA representation.
	compiledModelDFA
)

// compiledModel stores a runtime-ready content model.
type compiledModel struct {
	Rows      []compiledModelRow
	All       []compiledAllTerm
	Start     uint32
	AllBitLen uint32
	Kind      compiledModelKind
	Empty     bool
}

type dfaRowIndex struct {
	nameToEdge    map[QName]uint32
	wildcardEdges []uint32
}

func (idx dfaRowIndex) enabled() bool {
	return idx.nameToEdge != nil
}

// compiledModelRow stores a compiled DFA state.
type compiledModelRow struct {
	Edges         []compiledModelEdge
	index         dfaRowIndex
	CountParticle contentParticle
	Min           uint32
	Max           uint32
	Accept        bool
	Counted       bool
	Unbounded     bool
}

// compiledModelEdge stores one transition in a compiled DFA row.
type compiledModelEdge struct {
	Particle contentParticle
	To       uint32
}

// compiledAllTerm stores one xs:all term in compiled form.
type compiledAllTerm struct {
	Particle contentParticle
	Required bool
}

// sameCompiledParticle reports whether two compiled particles reference the
// same runtime term. occurrence and nested model IDs are intentionally ignored:
// compiled transitions contain direct element or wildcard particles.
func sameCompiledParticle(a, b contentParticle) bool {
	return a.Kind == b.Kind && a.Element == b.Element && a.Wildcard == b.Wildcard
}

const compiledDFARowIndexMinEdges = 8

type dfaRowIndexAnalysis struct {
	rt      *schemaBuild
	work    contentModelWork
	entries map[ElementID][]QName
}

// indexCompiledModelsRows builds row indexes for a model table and
// reuses each substitution head's expanded entries across the table.
func indexCompiledModelsRows(rt *schemaBuild, models []compiledModel, work contentModelWork) error {
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	analysis := newDFARowIndexAnalysis(rt, work)
	for i := range models {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
		if err := analysis.indexModel(&models[i]); err != nil {
			return err
		}
	}
	return nil
}

func newDFARowIndexAnalysis(rt *schemaBuild, work contentModelWork) *dfaRowIndexAnalysis {
	return &dfaRowIndexAnalysis{
		rt:      rt,
		work:    work,
		entries: make(map[ElementID][]QName),
	}
}

func (a *dfaRowIndexAnalysis) indexModel(model *compiledModel) error {
	if model.Kind != compiledModelDFA {
		return nil
	}
	for i := range model.Rows {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		if err := a.indexRow(&model.Rows[i]); err != nil {
			return err
		}
	}
	return nil
}

func (a *dfaRowIndexAnalysis) indexRow(row *compiledModelRow) error {
	if len(row.Edges) < compiledDFARowIndexMinEdges {
		return nil
	}
	if err := spendContentModelWorkN(a.work, len(row.Edges)); err != nil {
		return err
	}
	index := make(map[QName]uint32, len(row.Edges))
	var wildcards []uint32
	for pos, edge := range row.Edges {
		indexed, err := a.indexEdge(index, &wildcards, uint32(pos), edge)
		if err != nil {
			return err
		}
		if !indexed {
			return nil
		}
	}
	row.index = dfaRowIndex{nameToEdge: index, wildcardEdges: wildcards}
	return nil
}

func (a *dfaRowIndexAnalysis) indexEdge(
	index map[QName]uint32,
	wildcards *[]uint32,
	position uint32,
	edge compiledModelEdge,
) (bool, error) {
	switch edge.Particle.Kind {
	case particleElement:
		return a.indexElementEdge(index, position, edge.Particle.Element)
	case particleWildcard:
		*wildcards = append(*wildcards, position)
		return true, nil
	case particleModel:
		return false, nil
	default:
		return false, errors.New("compiled content model index has invalid edge particle")
	}
}

func (a *dfaRowIndexAnalysis) indexElementEdge(index map[QName]uint32, position uint32, element ElementID) (bool, error) {
	name, ok := a.rt.ElementName(element)
	if !ok {
		return false, errors.New("compiled content model index references invalid element")
	}
	if !indexEdgeName(index, name, position) {
		return false, nil
	}
	entries, err := a.substitutionEntries(element)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if err := spendContentModelWork(a.work); err != nil {
			return false, err
		}
		if !indexEdgeName(index, entry, position) {
			return false, nil
		}
	}
	return true, nil
}

func (a *dfaRowIndexAnalysis) substitutionEntries(id ElementID) ([]QName, error) {
	if entries, ok := a.entries[id]; ok {
		return entries, nil
	}
	var entries []QName
	var workErr error
	a.rt.Substitutions.ForEachEntry(id, func(name QName, _ ElementID) bool {
		workErr = spendContentModelWork(a.work)
		if workErr != nil {
			return false
		}
		entries = append(entries, name)
		return true
	})
	if workErr != nil {
		return nil, workErr
	}
	a.entries[id] = entries
	return entries, nil
}

func indexEdgeName(index map[QName]uint32, name QName, pos uint32) bool {
	if prev, ok := index[name]; ok {
		return prev == pos
	}
	index[name] = pos
	return true
}

// compiledCountingException reports whether overlapping counted-row edges are
// the one deterministic loop/exit pair produced for a fixed repeated particle.
func compiledCountingException(index uint32, row compiledModelRow, a, b compiledModelEdge) bool {
	if !row.Counted || row.Unbounded || row.Min != row.Max {
		return false
	}
	aLoop := a.To == index && sameCompiledParticle(a.Particle, row.CountParticle)
	bLoop := b.To == index && sameCompiledParticle(b.Particle, row.CountParticle)
	return (aLoop && b.To != index) || (bLoop && a.To != index)
}
