package schema

import (
	"math"
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

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

// compiledModel is one content model's compiled table, which lives only while
// that model is compiled and projected to its published form.
type compiledModel struct {
	Rows []compiledModelRow
	// Run, when set, is the edge array every row's Edges is a window of; edge
	// k targets row k+1. Only a direct sequence has one.
	Run       []compiledModelEdge
	All       []compiledAllTerm
	Start     uint32
	AllBitLen uint32
	Kind      compiledModelKind
	Empty     bool
}

// compiledContent holds one record for each content model that
// complexTypeReachableModels returns, and no other. A complex type's root
// model holds its published form, which publication takes without copying;
// any other model holds only its kind.
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

// publishedRunRepeated marks a run index value that locates the positions of
// a name the run holds more than once.
const publishedRunRepeated = 1 << 31

// dfaRowIndexAnalysis builds the name indexes of published models: the edge
// index of each DFA row with at least compiledDFARowIndexMinEdges edges and
// the term index of each xs:all model with at least as many terms. A row is
// indexed only when no name selects two of its edges. The rows that are
// windows of one edge run share one index of the run. Each substitution
// head's member names are computed once per analysis and kept for the
// analysis, at most the schema's substitution closure.
type dfaRowIndexAnalysis struct {
	rt            *schemaBuild
	work          contentModelWork
	retained      *workBudget
	substitutions map[ElementID][]QName
}

// newDFARowIndexAnalysis charges index traversal to work and each installed
// index entry (a name, a wildcard position, and the header and positions of a
// name a run repeats) to retained. An index is built before it is known to be
// installable, so a row's or run's transient index, bounded by its edges and
// their substitution members, is allocated before it is charged; an index that
// is not installed is never charged.
func newDFARowIndexAnalysis(rt *schemaBuild, work contentModelWork, retained *workBudget) *dfaRowIndexAnalysis {
	return &dfaRowIndexAnalysis{
		rt:            rt,
		work:          work,
		retained:      retained,
		substitutions: make(map[ElementID][]QName),
	}
}

// indexModel installs the name indexes of one published model. run is the
// edge run the model's rows are windows of, or nil when each row owns its
// edges.
func (a *dfaRowIndexAnalysis) indexModel(model *compiledModelRead, run []compiledModelEdgeRead) error {
	if err := spendContentModelWork(a.work); err != nil {
		return err
	}
	switch model.Kind {
	case compiledModelDFA:
		if run != nil {
			return a.indexRun(model.Rows, run)
		}
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
	names := make(map[QName]uint32, len(row.Edges))
	wildcards := 0
	for pos, edge := range row.Edges {
		switch edge.Particle.Kind {
		case particleWildcard:
			wildcards++
			continue
		case particleElement:
		case particleModel:
			return nil
		default:
			return xsderrors.InternalInvariant("compiled content model index has invalid edge particle")
		}
		unique, err := a.collectElementNames(edge.Particle.Element, uint32(pos), uniqueNames(names))
		if err != nil || !unique {
			return err
		}
	}
	if err := a.retained.spend(len(names) + wildcards); err != nil {
		return err
	}
	var positions []uint32
	if wildcards > 0 {
		positions = make([]uint32, 0, wildcards)
		for pos, edge := range row.Edges {
			if edge.Particle.Kind == particleWildcard {
				positions = append(positions, uint32(pos))
			}
		}
	}
	row.index = publishedDFAIndex{nameToEdge: names, data: positions}
	row.indexKind = publishedDFAMap
	return nil
}

// uniqueNames returns an add function for collectElementNames that records
// the position each name selects and reports false when a name already
// selects another position.
func uniqueNames(names map[QName]uint32) func(QName, uint32) bool {
	return func(name QName, pos uint32) bool {
		if prev, ok := names[name]; ok {
			return prev == pos
		}
		names[name] = pos
		return true
	}
}

// indexRun installs one index of run on each row that takes at least
// compiledDFARowIndexMinEdges of its edges and holds no name twice. Run
// positions of a window are consecutive, so a row's index lookups translate
// run positions by the window's first position. The index maps a name to its
// one run position, or, with publishedRunRepeated set, to the offset in data
// of its position count followed by its ascending positions. data starts with
// the wildcard count and the ascending wildcard positions.
func (a *dfaRowIndexAnalysis) indexRun(rows []compiledModelRowRead, run []compiledModelEdgeRead) error {
	wide := false
	for i := range rows {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		wide = wide || len(rows[i].Edges) >= compiledDFARowIndexMinEdges
	}
	if !wide || len(run) >= publishedRunRepeated {
		return nil
	}
	last := make(map[QName]uint32, len(run))
	counts := make(map[QName]uint32, len(run))
	// nextRepeat[p] is the nearest later position of a name at p.
	nextRepeat := make([]uint32, len(run))
	wildcards := 0
	for p, edge := range run {
		nextRepeat[p] = math.MaxUint32
		switch edge.Particle.Kind {
		case particleWildcard:
			wildcards++
			continue
		case particleElement:
		case particleModel:
			return nil
		default:
			return xsderrors.InternalInvariant("compiled content model index has invalid edge particle")
		}
		if _, err := a.collectElementNames(edge.Particle.Element, uint32(p), func(name QName, pos uint32) bool {
			prev, seen := last[name]
			switch {
			case !seen:
				counts[name] = 1
			case prev != pos:
				nextRepeat[prev] = min(nextRepeat[prev], pos)
				counts[name]++
			}
			last[name] = pos
			return true
		}); err != nil {
			return err
		}
	}
	for p := len(run) - 2; p >= 0; p-- {
		nextRepeat[p] = min(nextRepeat[p], nextRepeat[p+1])
	}
	indexable := func(row *compiledModelRowRead) bool {
		if len(row.Edges) < compiledDFARowIndexMinEdges {
			return false
		}
		start := row.Edges[0].To - 1
		return uint64(nextRepeat[start]) >= uint64(start)+uint64(len(row.Edges))
	}
	if !slices.ContainsFunc(rows, func(row compiledModelRowRead) bool { return indexable(&row) }) {
		return nil
	}
	size := 1 + wildcards
	for _, count := range counts {
		if count > 1 {
			size += 1 + int(count)
		}
	}
	if wildcards == 0 && size == 1 {
		if installed, err := a.indexDenseRun(rows, last, indexable); installed || err != nil {
			return err
		}
	}
	if err := a.retained.spend(len(counts) + size); err != nil {
		return err
	}
	index := make(map[QName]uint32, len(counts))
	data := make([]uint32, 1, size)
	data[0] = uint32(wildcards)
	for p, edge := range run {
		if edge.Particle.Kind == particleWildcard {
			data = append(data, uint32(p))
		}
	}
	groups := len(data)
	data = data[:size]
	for p, edge := range run {
		if edge.Particle.Kind != particleElement {
			continue
		}
		if _, err := a.collectElementNames(edge.Particle.Element, uint32(p), func(name QName, pos uint32) bool {
			if counts[name] == 1 {
				index[name] = pos
				return true
			}
			value, ok := index[name]
			if !ok {
				value = publishedRunRepeated | uint32(groups)
				groups += 1 + int(counts[name])
				index[name] = value
			}
			group := data[value&^publishedRunRepeated:]
			if group[0] == 0 || group[group[0]] != pos {
				group[0]++
				group[group[0]] = pos
			}
			return true
		}); err != nil {
			return err
		}
	}
	for i := range rows {
		if indexable(&rows[i]) {
			rows[i].index = publishedDFAIndex{nameToEdge: index, data: data}
			rows[i].indexKind = publishedDFARun
		}
	}
	return nil
}

// indexDenseRun installs a dense index of a run without wildcards whose
// names, each at its one position, share one namespace and span at most twice
// as many local name IDs as there are names. Each slot holds a run position
// plus one, so the index grows with the run's names, as the map would.
func (a *dfaRowIndexAnalysis) indexDenseRun(rows []compiledModelRowRead, positions map[QName]uint32, indexable func(*compiledModelRowRead) bool) (bool, error) {
	if len(positions) < 2 {
		return false, nil
	}
	var bounds denseNameBounds
	first := true
	for name := range positions {
		if first {
			bounds = newDenseNameBounds(name)
			first = false
		}
		bounds.include(name)
	}
	span := uint64(bounds.maxLocal) - uint64(bounds.minLocal) + 1
	if bounds.mixed || span > 2*uint64(len(positions)) {
		return false, nil
	}
	if err := a.retained.spend(int(span) + denseEdgeSlotsOffset); err != nil {
		return false, err
	}
	data := make([]uint32, span+denseEdgeSlotsOffset)
	data[denseNamespaceOffset] = uint32(bounds.namespace)
	data[denseMinLocalOffset] = uint32(bounds.minLocal)
	for name, pos := range positions {
		data[uint64(name.Local)-uint64(bounds.minLocal)+denseEdgeSlotsOffset] = pos + 1
	}
	for i := range rows {
		if indexable(&rows[i]) {
			rows[i].index = publishedDFAIndex{data: data}
			rows[i].indexKind = publishedDFARunDense
		}
	}
	return true, nil
}

// indexAllTerms installs the name index of a large xs:all model. The index
// uses the same substitution expansion as DFA rows; occurrence bits remain
// document-owned.
func (a *dfaRowIndexAnalysis) indexAllTerms(model *compiledModelRead) error {
	if len(model.All) < compiledDFARowIndexMinEdges {
		return nil
	}
	names := make(map[QName]uint32, len(model.All))
	for i, term := range model.All {
		if term.Particle.Kind != particleElement {
			return xsderrors.InternalInvariant("all-group index requires element particles")
		}
		unique, err := a.collectElementNames(term.Particle.Element, uint32(i), uniqueNames(names))
		if err != nil || !unique {
			return err
		}
	}
	if err := a.retained.spend(len(names)); err != nil {
		return err
	}
	model.allNames = names
	return nil
}

// collectElementNames passes the name of element and of each of its
// substitution members, with pos, to add, charging one unit of work for each,
// and stops when add reports false.
func (a *dfaRowIndexAnalysis) collectElementNames(element ElementID, pos uint32, add func(QName, uint32) bool) (bool, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return false, err
	}
	name, ok := a.rt.ElementName(element)
	if !ok {
		return false, xsderrors.InternalInvariant("compiled content model index references invalid element")
	}
	if !add(name, pos) {
		return false, nil
	}
	members, err := a.substitutionNames(element)
	if err != nil {
		return false, err
	}
	for _, member := range members {
		if err := spendContentModelWork(a.work); err != nil {
			return false, err
		}
		if !add(member, pos) {
			return false, nil
		}
	}
	return true, nil
}

func (a *dfaRowIndexAnalysis) substitutionNames(id ElementID) ([]QName, error) {
	if names, ok := a.substitutions[id]; ok {
		return names, nil
	}
	var names []QName
	var workErr error
	a.rt.Substitutions.ForEachEntry(id, func(name QName, _ ElementID) bool {
		workErr = spendContentModelWork(a.work)
		if workErr != nil {
			return false
		}
		names = append(names, name)
		return true
	})
	if workErr != nil {
		return nil, workErr
	}
	a.substitutions[id] = names
	return names, nil
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
