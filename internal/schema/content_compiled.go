package schema

import (
	"math"
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

// ContentState is the mutable validation state for one compiled content model.
type ContentState struct {
	model   contentModelID
	state   uint32
	count   uint32
	present bool
}

// HasModel reports whether state references a compiled content model.
func (st ContentState) HasModel() bool {
	return st.present && st.model != noContentModel
}

// ContentScratch is the caller-owned all-group occurrence bit storage for one
// content-model operation.
type ContentScratch struct {
	bits   []uint64
	base   int
	length int
}

// NewContentScratch returns an opaque view over caller-owned all-group
// occurrence bits.
func NewContentScratch(bits []uint64, base, length int) ContentScratch {
	return ContentScratch{bits: bits, base: base, length: length}
}

// AllSeen reports whether all-group particle i has already matched.
func (s *ContentScratch) AllSeen(i int) (seen, valid bool) {
	idx, bit, ok := s.allBit(i)
	if !ok {
		return false, false
	}
	return s.bits[idx]&bit != 0, true
}

// SetAllSeen marks all-group particle i as matched.
func (s *ContentScratch) SetAllSeen(i int) bool {
	idx, bit, ok := s.allBit(i)
	if !ok {
		return false
	}
	s.bits[idx] |= bit
	return true
}

func (s *ContentScratch) allBit(i int) (int, uint64, bool) {
	if s == nil || i < 0 || i/64 >= s.length {
		return 0, 0, false
	}
	idx := s.base + i/64
	if idx < 0 || idx >= len(s.bits) {
		return 0, 0, false
	}
	return idx, uint64(1) << uint(i%64), true
}

// ContentInput identifies a candidate child element for content-model matching.
type ContentInput struct {
	Name       RuntimeName
	HasXSIType bool
}

// ContentMatchKind identifies the schema assessment selected by a planned
// content-model transition.
type ContentMatchKind uint8

const (
	// ContentMatchInvalid is the zero value. It represents either a valid
	// non-match or an invalid transition, as reported by the surrounding status.
	ContentMatchInvalid ContentMatchKind = iota
	// ContentMatchDeclared selects a concrete element declaration.
	ContentMatchDeclared
	// ContentMatchAssessUndeclared selects wildcard assessment without a declaration.
	ContentMatchAssessUndeclared
	// ContentMatchSkip selects wildcard skip processing.
	ContentMatchSkip
	// ContentMatchStrictMissing records a strict wildcard without a declaration.
	ContentMatchStrictMissing
)

type contentMatch struct {
	element ElementID
	kind    ContentMatchKind
}

// ContentTransition is one validated content-model transition. It owns the
// parent state and all-group bit update until Commit applies both together.
type ContentTransition struct {
	match    contentMatch
	from     ContentState
	next     ContentState
	allIndex int
	markAll  bool
	valid    bool
}

// Match returns the assessment kind and, for ContentMatchDeclared, its element.
func (t ContentTransition) Match() (ContentMatchKind, ElementID) {
	if t.match.kind != ContentMatchDeclared {
		return t.match.kind, NoElement
	}
	return t.match.kind, t.match.element
}

// IsPlanned reports whether the transition was produced by a successful match.
func (t ContentTransition) IsPlanned() bool {
	return t.valid
}

// CanCommit reports whether state and scratch still match the transition's
// preconditions.
func (t ContentTransition) CanCommit(state ContentState, scratch *ContentScratch) bool {
	if !t.valid || state != t.from {
		return false
	}
	if !t.markAll {
		return true
	}
	seen, ok := scratch.AllSeen(t.allIndex)
	return ok && !seen
}

// Commit applies a transition produced for state and scratch. It returns false
// when either no transition was planned or its parent state is stale.
func (t ContentTransition) Commit(state *ContentState, scratch *ContentScratch) bool {
	if state == nil || !t.CanCommit(*state, scratch) {
		return false
	}
	if t.markAll {
		if !scratch.SetAllSeen(t.allIndex) {
			return false
		}
	}
	*state = t.next
	return true
}

// ContentTransitionStatus reports the outcome of planning one content-model transition.
type ContentTransitionStatus uint8

const (
	// ContentTransitionInvalid reports invalid published metadata or state.
	ContentTransitionInvalid ContentTransitionStatus = iota
	// ContentTransitionNoMatch reports a valid state with no matching transition.
	ContentTransitionNoMatch
	// contentTransitionMatched reports a valid, uncommitted matching transition.
	contentTransitionMatched
)

// ContentCompletionStatus reports whether a content-model state may end.
type ContentCompletionStatus uint8

const (
	// ContentCompletionInvalid reports invalid published metadata or state.
	ContentCompletionInvalid ContentCompletionStatus = iota
	// contentCompletionIncomplete reports a valid state that may not end.
	contentCompletionIncomplete
	// ContentCompletionComplete reports a valid state that may end.
	ContentCompletionComplete
)

// contentFrame is the initial validation state for one element frame.
type contentFrame struct {
	state  ContentState
	allLen int
}

type compiledModelRead struct {
	allNames  map[QName]uint32
	Rows      []compiledModelRowRead
	All       []compiledAllTermRead
	Start     uint32
	AllBitLen uint32
	Kind      compiledModelKind
	Empty     bool
}

// compiledModelRowRead has a per-architecture storage ceiling
// (TestCompiledModelRowReadStorageCeiling); optional dispatch metadata must not
// enlarge it. Each published row owns exactly one index variant, named by
// indexKind.
type compiledModelRowRead struct {
	Edges         []compiledModelEdgeRead
	index         publishedDFAIndex
	CountParticle compiledParticleRead
	Min           uint32
	Max           uint32
	Accept        bool
	Counted       bool
	Unbounded     bool
	indexKind     publishedDFAIndexKind
}

type publishedDFAIndexKind uint8

const (
	publishedDFALinear publishedDFAIndexKind = iota
	publishedDFAMap
	publishedDFADense
	// publishedDFARun marks a row whose edges are a window of its model's edge
	// run and whose index is the run's, shared by the run's other indexed
	// rows (dfaRowIndexAnalysis.indexRun).
	publishedDFARun
	// publishedDFARunDense is publishedDFARun with a dense run index.
	publishedDFARunDense
)

// Map data is ordered wildcard positions; dense data is metadata and slots;
// run data is the wildcard count, the ordered wildcard positions and the
// positions of repeated names; dense run data is dense metadata and slots of
// run positions. The row kind owns interpretation, so metadata
// never enters wildcard iteration.
type publishedDFAIndex struct {
	nameToEdge map[QName]uint32
	data       []uint32
}

type compiledModelEdgeRead struct {
	Particle compiledParticleRead
	To       uint32
}

type compiledAllTermRead struct {
	Particle compiledParticleRead
	Required bool
}

type compiledParticleRead struct {
	Element  ElementID
	Wildcard WildcardID
	Kind     particleKind
}

func newCompiledParticleRead(p contentParticle) compiledParticleRead {
	return compiledParticleRead{Element: p.Element, Wildcard: p.Wildcard, Kind: p.Kind}
}

// enteredContent is the set of compiled models validation can enter, with
// each complex type's index into the published table built from Models.
type enteredContent struct {
	// Models holds the compiled-table positions of the entered models,
	// ascending and deduplicated; it is the published table's order.
	Models []int
	// Published is parallel to the complex types: the published index of the
	// model each type enters.
	Published []contentModelID
}

// enterContentModels derives the entered models from the complex types.
// Complex-type roots are the only entry points: nested groups are compiled into
// their ancestors' rows, so no published particle references another model.
// Every empty-kind model compiles to the same read, so empty roots share the
// lowest such position and publication keeps one row for them. A compiled
// all model that no complex type enters violates All Group Limited and is an
// internal invariant violation.
func enterContentModels(types []complexTypeMetadata, content *compiledContent) (enteredContent, error) {
	roots, err := contentRoots(types, content)
	if err != nil {
		return enteredContent{}, err
	}
	entered := slices.Clone(roots)
	slices.Sort(entered)
	entered = slices.Compact(entered)
	for i := range content.Models {
		if _, ok := slices.BinarySearch(entered, i); !ok && content.Models[i].Kind == compiledModelAll {
			return enteredContent{}, xsderrors.InternalInvariant("schema compiled all content model is not the content of a complex type")
		}
	}
	position := make([]contentModelID, len(content.Models))
	for index, root := range entered {
		position[root] = contentModelID(index) // #nosec G115 -- entered holds at most one position per complex type, and complex-type IDs are uint32.
	}
	published := make([]contentModelID, len(roots))
	for i, root := range roots {
		published[i] = position[root]
	}
	return enteredContent{Models: entered, Published: published}, nil
}

// contentRoots returns the compiled-table position of each complex type's root
// model, with empty roots folded to the lowest empty root position.
func contentRoots(types []complexTypeMetadata, content *compiledContent) ([]int, error) {
	roots := make([]int, len(types))
	for i := range types {
		root, ok := slices.BinarySearch(content.Sources, types[i].Content)
		if !ok {
			return nil, xsderrors.InternalInvariant("schema compiled content-model table lacks a complex-type root")
		}
		roots[i] = root
	}
	foldEmptyRoots(roots, content.Models)
	return roots, nil
}

func foldEmptyRoots(roots []int, models []compiledModelRead) {
	empty := -1
	for _, root := range roots {
		if models[root].Kind == compiledModelEmpty && (empty < 0 || root < empty) {
			empty = root
		}
	}
	for i, root := range roots {
		if models[root].Kind == compiledModelEmpty {
			roots[i] = empty
		}
	}
}

// publishedContentModels returns the entered models in entered order. They
// share their tables with models.
func publishedContentModels(models []compiledModelRead, entered []int) []compiledModelRead {
	reads := make([]compiledModelRead, len(entered))
	for i, id := range entered {
		reads[i] = models[id]
	}
	return reads
}

// newCompiledModelRead projects one compiled model to its published form,
// charging one unit of work per model, row, edge and xs:all term. Each table
// is allocated at its exact length, and each row's Edges has no spare
// capacity, so appending to it never writes into another row's edges. When
// the model's rows are windows of one run of edges, the published rows are
// windows of its projected run, which newCompiledModelRead also returns.
func newCompiledModelRead(model compiledModel, work contentModelWork) (compiledModelRead, []compiledModelEdgeRead, error) {
	if err := chargeCompiledModelProjectionWork(model, work); err != nil {
		return compiledModelRead{}, nil, err
	}
	run := readEdges(model.Run)
	var rows []compiledModelRowRead
	if model.Run == nil {
		rows = readRows(model.Rows)
	} else {
		var err error
		if rows, err = readRunRows(model.Rows, run); err != nil {
			return compiledModelRead{}, nil, err
		}
	}
	return compiledModelRead{
		Rows:      rows,
		All:       readAll(model.All),
		Start:     model.Start,
		AllBitLen: model.AllBitLen,
		Kind:      model.Kind,
		Empty:     model.Empty,
	}, run, nil
}

func readRows(source []compiledModelRow) []compiledModelRowRead {
	if len(source) == 0 {
		return nil
	}
	edgeCount := 0
	for i := range source {
		edgeCount += len(source[i].Edges)
	}
	rows := make([]compiledModelRowRead, len(source))
	edges := make([]compiledModelEdgeRead, edgeCount)
	for i, row := range source {
		end := len(row.Edges)
		rowEdges := edges[:end:end]
		edges = edges[end:]
		for j, edge := range row.Edges {
			rowEdges[j] = newCompiledModelEdgeRead(edge)
		}
		if end == 0 {
			rowEdges = nil
		}
		rows[i] = newCompiledModelRowRead(row, rowEdges)
	}
	return rows
}

// readRunRows projects rows whose edges are windows of run. Edge k of run
// targets row k+1, so a window's first target locates it in run.
func readRunRows(source []compiledModelRow, run []compiledModelEdgeRead) ([]compiledModelRowRead, error) {
	for k, edge := range run {
		if uint64(edge.To) != uint64(k)+1 {
			return nil, xsderrors.InternalInvariant("compiled edge run does not target consecutive rows")
		}
	}
	rows := make([]compiledModelRowRead, len(source))
	for i, row := range source {
		var window []compiledModelEdgeRead
		if len(row.Edges) > 0 {
			start := uint64(row.Edges[0].To) - 1
			end := start + uint64(len(row.Edges))
			if row.Edges[0].To == 0 || end > uint64(len(run)) || uint64(row.Edges[len(row.Edges)-1].To) != end {
				return nil, xsderrors.InternalInvariant("compiled row edges are not a window of the model's edge run")
			}
			window = run[start:end:end]
		}
		rows[i] = newCompiledModelRowRead(row, window)
	}
	return rows, nil
}

func newCompiledModelRowRead(row compiledModelRow, edges []compiledModelEdgeRead) compiledModelRowRead {
	return compiledModelRowRead{
		Edges:         edges,
		CountParticle: newCompiledParticleRead(row.CountParticle),
		Min:           row.Min,
		Max:           row.Max,
		Accept:        row.Accept,
		Counted:       row.Counted,
		Unbounded:     row.Unbounded,
	}
}

func readEdges(source []compiledModelEdge) []compiledModelEdgeRead {
	if len(source) == 0 {
		return nil
	}
	edges := make([]compiledModelEdgeRead, len(source))
	for i, edge := range source {
		edges[i] = newCompiledModelEdgeRead(edge)
	}
	return edges
}

func newCompiledModelEdgeRead(edge compiledModelEdge) compiledModelEdgeRead {
	return compiledModelEdgeRead{Particle: newCompiledParticleRead(edge.Particle), To: edge.To}
}

func readAll(source []compiledAllTerm) []compiledAllTermRead {
	if len(source) == 0 {
		return nil
	}
	all := make([]compiledAllTermRead, len(source))
	for i, term := range source {
		all[i] = compiledAllTermRead{Particle: newCompiledParticleRead(term.Particle), Required: term.Required}
	}
	return all
}

// chargeCompiledModelProjectionWork charges one unit per model, row, xs:all
// term and published edge: each edge of a run once, and otherwise each edge of
// each row.
func chargeCompiledModelProjectionWork(model compiledModel, work contentModelWork) error {
	if err := spendContentModelWorkN(work, 1+len(model.All)+len(model.Run)); err != nil {
		return err
	}
	for _, row := range model.Rows {
		edges := len(row.Edges)
		if model.Run != nil {
			edges = 0
		}
		if err := spendContentModelWorkN(work, 1+edges); err != nil {
			return err
		}
	}
	return nil
}

func spendContentModelWorkN(work contentModelWork, count int) error {
	for range count {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
	}
	return nil
}

func sameCompiledParticleRead(a, b compiledParticleRead) bool {
	return a == b
}

// ContentState returns the initial mutable content-model state for the frame.
func (f contentFrame) ContentState() ContentState {
	return f.state
}

// AllBitLen returns the number of all-group bit words required by the frame.
func (f contentFrame) AllBitLen() int {
	return f.allLen
}

func (rt *Schema) contentFrameForModel(modelID contentModelID) (contentFrame, bool) {
	frame := contentFrame{state: ContentState{model: modelID, present: true}}
	if modelID == noContentModel {
		return frame, true
	}
	if !validContentModelID(modelID, len(rt.program.CompiledModels)) {
		return contentFrame{}, false
	}
	model := rt.program.CompiledModels[modelID]
	frame.state.state = model.Start
	frame.allLen = int(model.AllBitLen)
	return frame, true
}

// NextContent derives one transition from published schema slices without
// mutating the parent state or caller-owned all-group scratch.
func (rt *Schema) NextContent(st ContentState, in ContentInput, scratch *ContentScratch) (ContentTransition, ContentTransitionStatus) {
	if !st.HasModel() {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if !validContentModelID(st.model, len(rt.program.CompiledModels)) {
		return ContentTransition{}, ContentTransitionInvalid
	}
	model := &rt.program.CompiledModels[st.model]
	switch model.Kind {
	case compiledModelAny:
		return newContentTransition(st, st, rt.matchPublishedAnyContent(in)), contentTransitionMatched
	case compiledModelAll:
		return rt.nextPublishedAllContent(st, model, in, scratch)
	case compiledModelDFA:
		return rt.nextPublishedDFAContent(st, model, in)
	case compiledModelEmpty:
		return ContentTransition{}, ContentTransitionNoMatch
	default:
	}
	return ContentTransition{}, ContentTransitionInvalid
}

func newContentTransition(from, next ContentState, match contentMatch) ContentTransition {
	switch match.kind {
	case ContentMatchDeclared, ContentMatchAssessUndeclared, ContentMatchSkip, ContentMatchStrictMissing:
		return ContentTransition{match: match, from: from, next: next, valid: true}
	case ContentMatchInvalid:
		return ContentTransition{}
	default:
	}
	return ContentTransition{}
}

// CompleteContent reports whether a freeze-validated content state may end.
func (rt *Schema) CompleteContent(st ContentState, scratch *ContentScratch) ContentCompletionStatus {
	if !st.HasModel() {
		return ContentCompletionInvalid
	}
	if !validContentModelID(st.model, len(rt.program.CompiledModels)) {
		return ContentCompletionInvalid
	}
	model := &rt.program.CompiledModels[st.model]
	switch model.Kind {
	case compiledModelEmpty, compiledModelAny:
		return ContentCompletionComplete
	case compiledModelAll:
		return completePublishedAllContent(model, scratch)
	case compiledModelDFA:
		return completePublishedDFAContent(st, model)
	default:
		return ContentCompletionInvalid
	}
}

func (rt *Schema) matchPublishedAnyContent(in ContentInput) contentMatch {
	if in.Name.Known {
		if id, ok := rt.program.GlobalElements[in.Name.Name]; ok {
			return declaredContentMatch(id)
		}
	}
	return assessUndeclaredContentMatch()
}

func (rt *Schema) nextPublishedAllContent(st ContentState, model *compiledModelRead, in ContentInput, scratch *ContentScratch) (ContentTransition, ContentTransitionStatus) {
	if model.allNames != nil {
		return rt.nextIndexedAllContent(st, model, in, scratch)
	}
	for i, term := range model.All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if seen {
			continue
		}
		match, valid := rt.matchPublishedDirectParticle(term.Particle, in)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if match.kind == ContentMatchInvalid {
			continue
		}
		transition := newContentTransition(st, st, match)
		transition.allIndex = i
		transition.markAll = true
		return transition, contentTransitionMatched
	}
	return ContentTransition{}, ContentTransitionNoMatch
}

func (rt *Schema) nextIndexedAllContent(st ContentState, model *compiledModelRead, in ContentInput, scratch *ContentScratch) (ContentTransition, ContentTransitionStatus) {
	if _, valid := scratch.AllSeen(0); !valid {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if _, valid := scratch.AllSeen(len(model.All) - 1); !valid {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if !in.Name.Known {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	i, found := model.allNames[in.Name.Name]
	if !found {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	seen, valid := scratch.AllSeen(int(i))
	if !valid || !validUint32Index(i, len(model.All)) {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if seen {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	match, valid := rt.matchPublishedDirectParticle(model.All[i].Particle, in)
	if !valid {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if match.kind == ContentMatchInvalid {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	transition := newContentTransition(st, st, match)
	transition.allIndex = int(i)
	transition.markAll = true
	return transition, contentTransitionMatched
}

func completePublishedAllContent(model *compiledModelRead, scratch *ContentScratch) ContentCompletionStatus {
	empty, missingRequired, valid := inspectPublishedAllContent(model, scratch)
	if !valid {
		return ContentCompletionInvalid
	}
	if empty && model.Empty {
		return ContentCompletionComplete
	}
	if empty || missingRequired {
		return contentCompletionIncomplete
	}
	return ContentCompletionComplete
}

func inspectPublishedAllContent(model *compiledModelRead, scratch *ContentScratch) (empty, missingRequired, valid bool) {
	empty = true
	missingRequired = false
	for i, term := range model.All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return false, false, false
		}
		if seen {
			empty = false
			continue
		}
		if term.Required {
			missingRequired = true
		}
	}
	return empty, missingRequired, true
}

func (rt *Schema) nextPublishedDFAContent(st ContentState, model *compiledModelRead, in ContentInput) (ContentTransition, ContentTransitionStatus) {
	if !validUint32Index(st.state, len(model.Rows)) {
		return ContentTransition{}, ContentTransitionInvalid
	}
	row := &model.Rows[st.state]
	if row.indexKind == publishedDFAMap {
		return rt.nextPublishedIndexedDFAContent(st, model, row, row.index, in)
	}
	if row.indexKind == publishedDFARun || row.indexKind == publishedDFARunDense {
		return rt.nextPublishedRunDFAContent(st, model, row, in)
	}
	if row.indexKind == publishedDFADense {
		return rt.nextPublishedDenseDFAContent(st, model, row, in)
	}
	for _, edge := range row.Edges {
		match, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if match.kind == ContentMatchInvalid {
			continue
		}
		next, ok := nextPublishedDFAState(st, model, edge)
		if !ok {
			continue
		}
		return newContentTransition(st, next, match), contentTransitionMatched
	}
	return ContentTransition{}, ContentTransitionNoMatch
}

func (rt *Schema) nextPublishedDenseDFAContent(st ContentState, model *compiledModelRead, row *compiledModelRowRead, in ContentInput) (ContentTransition, ContentTransitionStatus) {
	if !in.Name.Known {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	pos := row.index.denseEdge(in.Name.Name)
	if pos < 0 {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	edge := row.Edges[pos]
	match, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
	if !valid {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if match.kind == ContentMatchInvalid {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	next, ok := nextPublishedDFAState(st, model, edge)
	if !ok {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	return newContentTransition(st, next, match), contentTransitionMatched
}

func completePublishedDFAContent(st ContentState, model *compiledModelRead) ContentCompletionStatus {
	if !validUint32Index(st.state, len(model.Rows)) {
		return ContentCompletionInvalid
	}
	row := &model.Rows[st.state]
	if row.Accept && (!row.Counted || st.count >= row.Min) {
		return ContentCompletionComplete
	}
	return contentCompletionIncomplete
}

func (rt *Schema) nextPublishedIndexedDFAContent(st ContentState, model *compiledModelRead, row *compiledModelRowRead, idx publishedDFAIndex, in ContentInput) (ContentTransition, ContentTransitionStatus) {
	candidates := newPublishedDFACandidates(idx, in)
	for {
		pos, ok := candidates.next()
		if !ok {
			return ContentTransition{}, ContentTransitionNoMatch
		}
		edge := row.Edges[pos]
		match, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if match.kind == ContentMatchInvalid {
			continue
		}
		next, ok := nextPublishedDFAState(st, model, edge)
		if !ok {
			continue
		}
		return newContentTransition(st, next, match), contentTransitionMatched
	}
}

type publishedDFACandidates struct {
	wildcards []uint32
	element   int
	nextWild  int
}

func newPublishedDFACandidates(idx publishedDFAIndex, in ContentInput) *publishedDFACandidates {
	element := -1
	if in.Name.Known {
		if pos, ok := idx.nameToEdge[in.Name.Name]; ok {
			element = int(pos)
		}
	}
	return &publishedDFACandidates{wildcards: idx.data, element: element}
}

// nextPublishedRunDFAContent dispatches a row whose edges are the run
// positions from start, which its first edge's target follows, to end, through
// its model's run index. Candidates are run positions.
func (rt *Schema) nextPublishedRunDFAContent(st ContentState, model *compiledModelRead, row *compiledModelRowRead, in ContentInput) (ContentTransition, ContentTransitionStatus) {
	start := row.Edges[0].To - 1
	end := start + uint32(len(row.Edges))
	if row.indexKind == publishedDFARunDense {
		if !in.Name.Known {
			return ContentTransition{}, ContentTransitionNoMatch
		}
		pos := row.index.denseEdge(in.Name.Name)
		if pos < 0 || uint32(pos) < start || uint32(pos) >= end {
			return ContentTransition{}, ContentTransitionNoMatch
		}
		return rt.takePublishedDFAEdge(st, model, row.Edges[uint32(pos)-start], in)
	}
	data := row.index.data
	wildcards := data[1 : 1+data[0]]
	low, _ := slices.BinarySearch(wildcards, start)
	high, _ := slices.BinarySearch(wildcards, end)
	element := -1
	if in.Name.Known {
		if value, ok := row.index.nameToEdge[in.Name.Name]; ok {
			if pos, ok := publishedRunPosition(data, value, start); ok && pos < end {
				element = int(pos)
			}
		}
	}
	candidates := publishedDFACandidates{wildcards: wildcards[low:high], element: element}
	for {
		pos, ok := candidates.next()
		if !ok {
			return ContentTransition{}, ContentTransitionNoMatch
		}
		transition, status := rt.takePublishedDFAEdge(st, model, row.Edges[pos-int(start)], in)
		if status != ContentTransitionNoMatch {
			return transition, status
		}
	}
}

// takePublishedDFAEdge reports the transition of edge for in, or no match when
// edge does not take in.
func (rt *Schema) takePublishedDFAEdge(st ContentState, model *compiledModelRead, edge compiledModelEdgeRead, in ContentInput) (ContentTransition, ContentTransitionStatus) {
	match, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
	if !valid {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if match.kind == ContentMatchInvalid {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	next, ok := nextPublishedDFAState(st, model, edge)
	if !ok {
		return ContentTransition{}, ContentTransitionNoMatch
	}
	return newContentTransition(st, next, match), contentTransitionMatched
}

// publishedRunPosition returns the first run position at or after start of
// the name a run index maps to value.
func publishedRunPosition(data []uint32, value, start uint32) (uint32, bool) {
	if value&publishedRunRepeated == 0 {
		return value, value >= start
	}
	group := data[value&^publishedRunRepeated:]
	positions := group[1 : 1+group[0]]
	i, _ := slices.BinarySearch(positions, start)
	if i == len(positions) {
		return 0, false
	}
	return positions[i], true
}

func (c *publishedDFACandidates) next() (int, bool) {
	switch {
	case c.nextWild < len(c.wildcards) && (c.element < 0 || int(c.wildcards[c.nextWild]) < c.element):
		position := int(c.wildcards[c.nextWild])
		c.nextWild++
		return position, true
	case c.element >= 0:
		position := c.element
		c.element = -1
		return position, true
	default:
		return 0, false
	}
}

func (rt *Schema) matchPublishedDirectParticle(p compiledParticleRead, in ContentInput) (contentMatch, bool) {
	switch p.Kind {
	case particleElement:
		return rt.matchPublishedElementParticle(p.Element, in)
	case particleWildcard:
		if !validWildcardID(p.Wildcard, len(rt.program.Wildcards)) {
			return contentMatch{}, false
		}
		return rt.matchPublishedWildcardParticle(rt.program.Wildcards[p.Wildcard], in), true
	case particleModel:
		return contentMatch{}, false
	default:
	}
	return contentMatch{}, false
}

func (rt *Schema) matchPublishedElementParticle(element ElementID, in ContentInput) (contentMatch, bool) {
	name, ok := rt.program.Elements.name(element)
	if !ok {
		return contentMatch{}, false
	}
	if !in.Name.Known {
		return contentMatch{}, true
	}
	if name == in.Name.Name {
		return declaredContentMatch(element), true
	}
	if member, ok := rt.program.Substitutions.MemberByName(element, in.Name.Name); ok {
		return declaredContentMatch(member), true
	}
	return contentMatch{}, true
}

func (rt *Schema) matchPublishedWildcardParticle(w WildcardView, in ContentInput) contentMatch {
	if !w.AllowsURI(in.Name.NS) {
		return contentMatch{}
	}
	switch w.Process() {
	case processStrict:
		return rt.matchPublishedStrictWildcard(in)
	case ProcessSkip:
		return skipContentMatch()
	case ProcessLax:
		if match, ok := rt.matchPublishedGlobalElement(in); ok {
			return match
		}
	}
	return assessUndeclaredContentMatch()
}

func (rt *Schema) matchPublishedStrictWildcard(in ContentInput) contentMatch {
	if match, ok := rt.matchPublishedGlobalElement(in); ok {
		return match
	}
	if in.HasXSIType {
		return assessUndeclaredContentMatch()
	}
	return strictMissingContentMatch()
}

func (rt *Schema) matchPublishedGlobalElement(in ContentInput) (contentMatch, bool) {
	if !in.Name.Known {
		return contentMatch{}, false
	}
	id, ok := rt.program.GlobalElements[in.Name.Name]
	if !ok {
		return contentMatch{}, false
	}
	return declaredContentMatch(id), true
}

func declaredContentMatch(element ElementID) contentMatch {
	return contentMatch{element: element, kind: ContentMatchDeclared}
}

func assessUndeclaredContentMatch() contentMatch {
	return contentMatch{element: NoElement, kind: ContentMatchAssessUndeclared}
}

func skipContentMatch() contentMatch {
	return contentMatch{element: NoElement, kind: ContentMatchSkip}
}

func strictMissingContentMatch() contentMatch {
	return contentMatch{element: NoElement, kind: ContentMatchStrictMissing}
}

func nextPublishedDFAState(st ContentState, model *compiledModelRead, edge compiledModelEdgeRead) (ContentState, bool) {
	to := edge.To
	from := &model.Rows[st.state]
	next := &model.Rows[to]
	var count uint32
	var ok bool
	if from.Counted && to == st.state && sameCompiledParticleRead(edge.Particle, from.CountParticle) {
		count, ok = nextPublishedDFASelfCount(st.count, from)
	} else {
		count, ok = nextPublishedDFATransitionCount(st.count, from, next, edge.Particle)
	}
	if !ok {
		return ContentState{}, false
	}
	st.state = to
	st.count = count
	return st, true
}

func nextPublishedDFASelfCount(count uint32, row *compiledModelRowRead) (uint32, bool) {
	if !row.Unbounded && count >= row.Max {
		return 0, false
	}
	if count != math.MaxUint32 {
		count++
	}
	return count, true
}

func nextPublishedDFATransitionCount(
	count uint32,
	from, next *compiledModelRowRead,
	particle compiledParticleRead,
) (uint32, bool) {
	if from.Counted && count < from.Min {
		return 0, false
	}
	if next.Counted && sameCompiledParticleRead(particle, next.CountParticle) {
		return 1, true
	}
	return 0, true
}
