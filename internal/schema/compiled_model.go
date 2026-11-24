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
// complexTypeReachableModels returns, and no other. A root model that an
// instance can reach (detachUnreachableComponents) holds its published form
// over the published element IDs of Elements, which publication takes as it
// is; any other model holds only its kind and the scalars of its form.
type compiledContent struct {
	// Elements is the element numbering the published tables were built
	// over, or nil for the identity, which publication checks against
	// schemaBuild.ReachableElements.
	Elements *publishedElements
	// Sources holds the source IDs of the compiled models, ascending and
	// distinct.
	Sources []contentModelID
	// Models is parallel to Sources.
	Models []compiledModelRead
	// Runs is parallel to Models: the edge run that every row of a root's
	// published table is a window of, or nil when each row holds its own
	// edges. It lets publication check each published edge once.
	Runs [][]compiledModelEdgeRead
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
const publishedRunRepeated uint32 = 1 << 31

// dfaRowIndexAnalysis builds the name indexes of published models: the edge
// index of each DFA row with at least compiledDFARowIndexMinEdges edges and
// the term index of each xs:all model with at least as many terms. A row is
// indexed only when no name selects two of its edges. The rows that are
// windows of one edge run share one index of the run. Each substitution
// head's member names are computed once per analysis and kept for the
// analysis, at most the schema's substitution closure. A table names
// declarations by their IDs in elements, which the analysis maps back to
// build IDs to read names and substitution members.
type dfaRowIndexAnalysis struct {
	rt            *schemaBuild
	elements      *publishedElements
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
func newDFARowIndexAnalysis(rt *schemaBuild, elements *publishedElements, work contentModelWork, retained *workBudget) *dfaRowIndexAnalysis {
	return &dfaRowIndexAnalysis{
		rt:            rt,
		elements:      elements,
		work:          work,
		retained:      retained,
		substitutions: make(map[ElementID][]QName),
	}
}

// over returns an analysis of tables numbered by elements that shares a's
// budgets and substitution member names.
func (a *dfaRowIndexAnalysis) over(elements *publishedElements) *dfaRowIndexAnalysis {
	over := *a
	over.elements = elements
	return &over
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
		return a.indexRows(model.Rows)
	case compiledModelAll:
		return a.indexAllTerms(model)
	case compiledModelEmpty, compiledModelAny:
	}
	return nil
}

func (a *dfaRowIndexAnalysis) indexRows(rows []compiledModelRowRead) error {
	for i := range rows {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		if err := a.indexRow(&rows[i]); err != nil {
			return err
		}
	}
	return nil
}

func (a *dfaRowIndexAnalysis) indexRow(row *compiledModelRowRead) error {
	if len(row.Edges) < compiledDFARowIndexMinEdges {
		return nil
	}
	wildcards, ok, err := indexableEdges(row.Edges)
	if !ok || err != nil {
		return err
	}
	names := make(map[QName]uint32, len(row.Edges))
	for pos, edge := range row.Edges {
		if edge.Particle.Kind != particleElement {
			continue
		}
		unique, err := a.collectElementNames(edge.Particle.Element, uint32(pos), uniqueNames(names))
		if err != nil || !unique {
			return err
		}
	}
	if err := a.retained.spend(len(names) + len(wildcards)); err != nil {
		return err
	}
	row.index = publishedDFAIndex{nameToEdge: names, data: wildcards}
	row.indexKind = publishedDFAMap
	return nil
}

// indexableEdges returns the ascending positions of the wildcard edges, or
// false when an edge's particle is a nested model, which no name index
// covers.
func indexableEdges(edges []compiledModelEdgeRead) ([]uint32, bool, error) {
	var wildcards []uint32
	for pos, edge := range edges {
		switch edge.Particle.Kind {
		case particleWildcard:
			wildcards = append(wildcards, uint32(pos))
		case particleElement:
		case particleModel:
			return nil, false, nil
		default:
			return nil, false, xsderrors.InternalInvariant("compiled content model index has invalid edge particle")
		}
	}
	return wildcards, true, nil
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

type runNames struct {
	last   map[QName]uint32
	counts map[QName]uint32
	// nextRepeat[p] is the nearest position after p of a name that also
	// occurs at or after p, or math.MaxUint32.
	nextRepeat []uint32
	// wildcards holds the ascending run positions of wildcard edges.
	wildcards []uint32
}

func (n *runNames) add(name QName, pos uint32) bool {
	prev, seen := n.last[name]
	switch {
	case !seen:
		n.counts[name] = 1
	case prev != pos:
		n.nextRepeat[prev] = min(n.nextRepeat[prev], pos)
		n.counts[name]++
	}
	n.last[name] = pos
	return true
}

// holdsNoRepeat reports whether a row's window holds no name twice. A row's
// window starts at the run position its first edge's target follows.
func (n *runNames) holdsNoRepeat(row *compiledModelRowRead) bool {
	start := row.Edges[0].To - 1
	return uint64(n.nextRepeat[start]) >= uint64(start)+uint64(len(row.Edges))
}

// indexDataSize returns the length of a run index's data: the wildcard count
// and positions, then each repeated name's position count and positions.
func (n *runNames) indexDataSize() int {
	size := 1 + len(n.wildcards)
	for _, count := range n.counts {
		if count > 1 {
			size += 1 + int(count)
		}
	}
	return size
}

func (a *dfaRowIndexAnalysis) scanRunNames(run []compiledModelEdgeRead) (*runNames, error) {
	wildcards, ok, err := indexableEdges(run)
	if !ok || err != nil {
		return nil, err
	}
	names := &runNames{
		last:       make(map[QName]uint32, len(run)),
		counts:     make(map[QName]uint32, len(run)),
		nextRepeat: make([]uint32, len(run)),
		wildcards:  wildcards,
	}
	for p := range names.nextRepeat {
		names.nextRepeat[p] = math.MaxUint32
	}
	if err := a.collectRunNames(run, names.add); err != nil {
		return nil, err
	}
	for p := len(run) - 2; p >= 0; p-- {
		names.nextRepeat[p] = min(names.nextRepeat[p], names.nextRepeat[p+1])
	}
	return names, nil
}

func (a *dfaRowIndexAnalysis) collectRunNames(run []compiledModelEdgeRead, add func(QName, uint32) bool) error {
	for p, edge := range run {
		if edge.Particle.Kind != particleElement {
			continue
		}
		if _, err := a.collectElementNames(edge.Particle.Element, uint32(p), add); err != nil {
			return err
		}
	}
	return nil
}

// indexRun installs one index of run on each row that takes at least
// compiledDFARowIndexMinEdges of its edges and holds no name twice. Run
// positions of a window are consecutive, so a row's index lookups translate
// run positions by the window's first position. The index maps a name to its
// one run position, or, with publishedRunRepeated set, to the offset in data
// of its position count followed by its ascending positions. data starts with
// the wildcard count and the ascending wildcard positions.
func (a *dfaRowIndexAnalysis) indexRun(rows []compiledModelRowRead, run []compiledModelEdgeRead) error {
	wide, err := a.hasWideRow(rows)
	if !wide || err != nil || uint64(len(run)) >= uint64(publishedRunRepeated) {
		return err
	}
	names, err := a.scanRunNames(run)
	if names == nil || err != nil {
		return err
	}
	indexable := func(row *compiledModelRowRead) bool {
		return len(row.Edges) >= compiledDFARowIndexMinEdges && names.holdsNoRepeat(row)
	}
	if !slices.ContainsFunc(rows, func(row compiledModelRowRead) bool { return indexable(&row) }) {
		return nil
	}
	return a.indexRunWindows(rows, run, names, indexable)
}

// indexRunWindows installs the dense index of run when it qualifies, and its
// map index otherwise, on the indexable rows.
func (a *dfaRowIndexAnalysis) indexRunWindows(rows []compiledModelRowRead, run []compiledModelEdgeRead, names *runNames, indexable func(*compiledModelRowRead) bool) error {
	size := names.indexDataSize()
	if size > int(publishedRunRepeated-1) {
		return nil
	}
	if size == 1 {
		if installed, err := a.indexDenseRun(rows, len(run), names.last, indexable); installed || err != nil {
			return err
		}
	}
	return a.indexRunMap(rows, run, names, size, indexable)
}

func (a *dfaRowIndexAnalysis) indexRunMap(rows []compiledModelRowRead, run []compiledModelEdgeRead, names *runNames, size int, indexable func(*compiledModelRowRead) bool) error {
	if err := a.retained.spend(len(names.counts) + size); err != nil {
		return err
	}
	index := runIndex{counts: names.counts, nameToEdge: make(map[QName]uint32, len(names.counts)), data: make([]uint32, size)}
	if err := index.addWildcards(names.wildcards); err != nil {
		return err
	}
	if err := a.collectRunNames(run, index.add); err != nil {
		return err
	}
	installRunIndex(rows, indexable, publishedDFAIndex{nameToEdge: index.nameToEdge, data: index.data}, publishedDFARun)
	return nil
}

func (a *dfaRowIndexAnalysis) hasWideRow(rows []compiledModelRowRead) (bool, error) {
	wide := false
	for i := range rows {
		if err := spendContentModelWork(a.work); err != nil {
			return false, err
		}
		wide = wide || len(rows[i].Edges) >= compiledDFARowIndexMinEdges
	}
	return wide, nil
}

func installRunIndex(rows []compiledModelRowRead, indexable func(*compiledModelRowRead) bool, index publishedDFAIndex, kind publishedDFAIndexKind) {
	for i := range rows {
		if indexable(&rows[i]) {
			rows[i].index = index
			rows[i].indexKind = kind
		}
	}
}

// runIndex builds the name index and data that indexRun describes. data is
// sized by runNames.indexDataSize, so every group offset stays below
// publishedRunRepeated.
type runIndex struct {
	counts     map[QName]uint32
	nameToEdge map[QName]uint32
	data       []uint32
	// groups is the data offset of the next repeated name's group.
	groups uint32
}

func (b *runIndex) addWildcards(positions []uint32) error {
	count, ok := newUint32Index(len(positions))
	if !ok {
		return xsderrors.InternalInvariant("run index wildcard count exceeds its data")
	}
	b.data[0] = count
	copy(b.data[1:], positions)
	b.groups = count + 1
	return nil
}

func (b *runIndex) add(name QName, pos uint32) bool {
	count := b.counts[name]
	if count == 1 {
		b.nameToEdge[name] = pos
		return true
	}
	value, ok := b.nameToEdge[name]
	if !ok {
		value = publishedRunRepeated | b.groups
		b.groups += 1 + count
		b.nameToEdge[name] = value
	}
	group := b.data[value&^publishedRunRepeated:]
	if group[0] == 0 || group[group[0]] != pos {
		group[0]++
		group[group[0]] = pos
	}
	return true
}

// indexDenseRun installs a dense index of a run without wildcards whose
// names, each at its one position, share one namespace and span at most twice
// as many local name IDs as there are names. Each slot holds a run position
// plus one, so the index grows with the run's names, as the map would.
func (a *dfaRowIndexAnalysis) indexDenseRun(rows []compiledModelRowRead, runLen int, positions map[QName]uint32, indexable func(*compiledModelRowRead) bool) (bool, error) {
	if len(positions) < 2 {
		return false, nil
	}
	bounds, valid := scanDenseIndexBounds(positions, runLen)
	if !valid {
		return false, xsderrors.InternalInvariant("run index name position is outside its run")
	}
	span := uint64(bounds.maxLocal) - uint64(bounds.minLocal) + 1
	if bounds.mixed || span > 2*uint64(len(positions)) {
		return false, nil
	}
	data := make([]uint32, span+denseEdgeSlotsOffset)
	if err := a.retained.spend(len(data)); err != nil {
		return false, err
	}
	data[denseNamespaceOffset] = uint32(bounds.namespace)
	data[denseMinLocalOffset] = uint32(bounds.minLocal)
	for name, pos := range positions {
		data[uint64(name.Local)-uint64(bounds.minLocal)+denseEdgeSlotsOffset] = pos + 1
	}
	installRunIndex(rows, indexable, publishedDFAIndex{data: data}, publishedDFARunDense)
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
func (a *dfaRowIndexAnalysis) collectElementNames(published publishedElementID, pos uint32, add func(QName, uint32) bool) (bool, error) {
	if err := spendContentModelWork(a.work); err != nil {
		return false, err
	}
	element, ok := a.elements.buildID(published)
	if !ok {
		return false, xsderrors.InternalInvariant("compiled content model index references invalid element")
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
