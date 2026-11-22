package schema

import "github.com/jacoelho/xsd/xsderrors"

// compiledModelKind identifies the runtime representation used for a compiled
// content model.
type compiledModelKind uint8

const (
	// compiledModelEmpty is the compiled representation for empty content.
	compiledModelEmpty compiledModelKind = iota
	// compiledModelAny is the compiled representation for content that is one
	// lax ##any wildcard 0..unbounded, such as xs:anyType's.
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

// compiledContent holds the published form of each content model that
// complexTypeReachableModels returns, and no other. Each model's compiled
// form lives only while that model is compiled; publication takes these
// tables without copying them.
type compiledContent struct {
	// Sources holds the source IDs of the compiled models, ascending and
	// distinct.
	Sources []contentModelID
	// Models is parallel to Sources.
	Models []compiledModelRead
}

// compiledModelRow stores a compiled DFA state.
type compiledModelRow struct {
	Edges         []compiledModelEdge
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

// dfaRowIndexAnalysis builds the name indexes of published models: the
// edge index of each DFA row with at least compiledDFARowIndexMinEdges edges
// and the term index of each xs:all model with at least as many terms. Each
// substitution head's expanded names are computed once per analysis.
type dfaRowIndexAnalysis struct {
	rt       *schemaBuild
	work     contentModelWork
	retained *workBudget
	entries  map[ElementID][]QName
}

// newDFARowIndexAnalysis charges index traversal to work and each index
// entry, a name or a wildcard position, to retained before it is stored.
func newDFARowIndexAnalysis(rt *schemaBuild, work contentModelWork, retained *workBudget) *dfaRowIndexAnalysis {
	return &dfaRowIndexAnalysis{
		rt:       rt,
		work:     work,
		retained: retained,
		entries:  make(map[ElementID][]QName),
	}
}

func (a *dfaRowIndexAnalysis) indexModel(model *compiledModelRead) error {
	if err := spendContentModelWork(a.work); err != nil {
		return err
	}
	switch model.Kind {
	case compiledModelDFA:
		for i := range model.Rows {
			if err := spendContentModelWork(a.work); err != nil {
				return err
			}
			if err := a.indexRow(&model.Rows[i]); err != nil {
				return err
			}
		}
	case compiledModelAll:
		return a.indexAllTerms(model)
	case compiledModelEmpty, compiledModelAny:
	}
	return nil
}

func (a *dfaRowIndexAnalysis) indexRow(row *compiledModelRowRead) error {
	if len(row.Edges) < compiledDFARowIndexMinEdges {
		return nil
	}
	if err := a.spendEntries(len(row.Edges)); err != nil {
		return err
	}
	index := make(map[QName]uint32, len(row.Edges))
	var wildcards []uint32
	for pos, edge := range row.Edges {
		indexed, err := a.indexEdge(index, &wildcards, uint32(pos), edge.Particle)
		if err != nil {
			return err
		}
		if !indexed {
			return nil
		}
	}
	row.index = publishedDFAIndex{nameToEdge: index, data: wildcards}
	row.indexKind = publishedDFAMap
	return nil
}

// indexAllTerms installs the name index of a large xs:all model. The index
// uses the same substitution expansion as DFA rows; occurrence bits remain
// document-owned.
func (a *dfaRowIndexAnalysis) indexAllTerms(model *compiledModelRead) error {
	if len(model.All) < compiledDFARowIndexMinEdges {
		return nil
	}
	if err := a.spendEntries(len(model.All)); err != nil {
		return err
	}
	index := make(map[QName]uint32, len(model.All))
	for i, term := range model.All {
		if term.Particle.Kind != particleElement {
			return xsderrors.InternalInvariant("all-group index requires element particles")
		}
		indexed, err := a.indexElementEdge(index, uint32(i), term.Particle.Element)
		if err != nil {
			return err
		}
		if !indexed {
			return nil
		}
	}
	model.allNames = index
	return nil
}

func (a *dfaRowIndexAnalysis) spendEntries(entries int) error {
	if err := spendContentModelWorkN(a.work, entries); err != nil {
		return err
	}
	return a.retained.spend(entries)
}

func (a *dfaRowIndexAnalysis) indexEdge(
	index map[QName]uint32,
	wildcards *[]uint32,
	position uint32,
	particle compiledParticleRead,
) (bool, error) {
	switch particle.Kind {
	case particleElement:
		return a.indexElementEdge(index, position, particle.Element)
	case particleWildcard:
		*wildcards = append(*wildcards, position)
		return true, nil
	case particleModel:
		return false, nil
	default:
		return false, xsderrors.InternalInvariant("compiled content model index has invalid edge particle")
	}
}

func (a *dfaRowIndexAnalysis) indexElementEdge(index map[QName]uint32, position uint32, element ElementID) (bool, error) {
	name, ok := a.rt.ElementName(element)
	if !ok {
		return false, xsderrors.InternalInvariant("compiled content model index references invalid element")
	}
	if !indexEdgeName(index, name, position) {
		return false, nil
	}
	entries, err := a.substitutionEntries(element)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		if err := a.spendEntries(1); err != nil {
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
