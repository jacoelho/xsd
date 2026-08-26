package runtime

import (
	"errors"
	"maps"
	"math"
	"slices"
)

// ContentState is the mutable validation state for one compiled content model.
type ContentState struct {
	model   ContentModelID
	state   uint32
	count   uint32
	present bool
}

// HasModel reports whether state references a compiled content model.
func (st ContentState) HasModel() bool {
	return st.present && st.model != NoContentModel
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
func (s *ContentScratch) AllSeen(i int) (bool, bool) {
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

// ContentMatch is the runtime result of matching one content-model transition.
type ContentMatch struct {
	Element       ElementID
	Skip          bool
	StrictMissing bool
}

// ContentTransition is one validated content-model transition. It owns the
// parent state and all-group bit update until Commit applies both together.
type ContentTransition struct {
	match    ContentMatch
	from     ContentState
	next     ContentState
	allIndex int
	markAll  bool
	valid    bool
}

// Match returns the declaration or wildcard result selected by the transition.
func (t ContentTransition) Match() ContentMatch {
	return t.match
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
	// ContentTransitionMatched reports a valid, uncommitted matching transition.
	ContentTransitionMatched
)

// ContentCompletionStatus reports whether a content-model state may end.
type ContentCompletionStatus uint8

const (
	// ContentCompletionInvalid reports invalid published metadata or state.
	ContentCompletionInvalid ContentCompletionStatus = iota
	// ContentCompletionIncomplete reports a valid state that may not end.
	ContentCompletionIncomplete
	// ContentCompletionComplete reports a valid state that may end.
	ContentCompletionComplete
)

// ContentFrame is the initial validation state for one element frame.
type ContentFrame struct {
	state  ContentState
	allLen int
}

type compiledModelRead struct {
	Rows      []compiledModelRowRead
	All       []compiledAllTermRead
	Start     uint32
	AllBitLen uint32
	Kind      CompiledModelKind
	Empty     bool
}

type compiledModelRowRead struct {
	Edges         []compiledModelEdgeRead
	Index         DFARowIndex
	CountParticle compiledParticleRead
	Min           uint32
	Max           uint32
	Accept        bool
	Counted       bool
	Unbounded     bool
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
	Kind     ParticleKind
}

func newCompiledParticleRead(p Particle) compiledParticleRead {
	return compiledParticleRead{Element: p.Element, Wildcard: p.Wildcard, Kind: p.Kind}
}

func newCompiledModelReads(models []CompiledModel, work ContentModelWork) ([]compiledModelRead, error) {
	if err := requireContentModelWork(work); err != nil {
		return nil, err
	}
	if err := chargeCompiledModelProjectionWork(models, work); err != nil {
		return nil, err
	}
	rowCount, edgeCount, allCount := compiledModelReadCounts(models)
	reads := make([]compiledModelRead, len(models))
	rows := make([]compiledModelRowRead, rowCount)
	edges := make([]compiledModelEdgeRead, edgeCount)
	all := make([]compiledAllTermRead, allCount)
	rowOffset, edgeOffset, allOffset := 0, 0, 0
	for i, model := range models {
		var modelRows []compiledModelRowRead
		if len(model.Rows) != 0 {
			end := rowOffset + len(model.Rows)
			modelRows = rows[rowOffset:end:end]
			rowOffset = end
		}
		for j, row := range model.Rows {
			var rowEdges []compiledModelEdgeRead
			if len(row.Edges) != 0 {
				end := edgeOffset + len(row.Edges)
				rowEdges = edges[edgeOffset:end:end]
				edgeOffset = end
				for k, edge := range row.Edges {
					rowEdges[k] = compiledModelEdgeRead{Particle: newCompiledParticleRead(edge.Particle), To: edge.To}
				}
			}
			modelRows[j] = compiledModelRowRead{
				Edges: rowEdges,
				Index: DFARowIndex{
					NameToEdge:    maps.Clone(row.Index.NameToEdge),
					WildcardEdges: slices.Clone(row.Index.WildcardEdges),
					Enabled:       row.Index.Enabled,
				},
				CountParticle: newCompiledParticleRead(row.CountParticle),
				Min:           row.Min,
				Max:           row.Max,
				Accept:        row.Accept,
				Counted:       row.Counted,
				Unbounded:     row.Unbounded,
			}
		}
		var modelAll []compiledAllTermRead
		if len(model.All) != 0 {
			end := allOffset + len(model.All)
			modelAll = all[allOffset:end:end]
			allOffset = end
			for j, term := range model.All {
				modelAll[j] = compiledAllTermRead{Particle: newCompiledParticleRead(term.Particle), Required: term.Required}
			}
		}
		reads[i] = compiledModelRead{
			Rows:      modelRows,
			All:       modelAll,
			Start:     model.Start,
			AllBitLen: model.AllBitLen,
			Kind:      model.Kind,
			Empty:     model.Empty,
		}
	}
	return reads, nil
}

func chargeCompiledModelProjectionWork(models []CompiledModel, work ContentModelWork) error {
	for _, model := range models {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
		for range model.All {
			if err := spendContentModelWork(work); err != nil {
				return err
			}
		}
		for _, row := range model.Rows {
			if err := spendContentModelWork(work); err != nil {
				return err
			}
			for range row.Edges {
				if err := spendContentModelWork(work); err != nil {
					return err
				}
			}
			for range row.Index.NameToEdge {
				if err := spendContentModelWork(work); err != nil {
					return err
				}
			}
			for range row.Index.WildcardEdges {
				if err := spendContentModelWork(work); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func compiledModelReadCounts(models []CompiledModel) (rows, edges, all int) {
	for i := range models {
		rows = addCompiledModelReadCount(rows, len(models[i].Rows))
		all = addCompiledModelReadCount(all, len(models[i].All))
		for j := range models[i].Rows {
			edges = addCompiledModelReadCount(edges, len(models[i].Rows[j].Edges))
		}
	}
	return rows, edges, all
}

func addCompiledModelReadCount(total, count int) int {
	if count > math.MaxInt-total {
		panic("compiled model read projection size exceeds int capacity")
	}
	return total + count
}

func validateCompiledModelReadProjectionTable(
	reads []compiledModelRead,
	models []CompiledModel,
	work ContentModelWork,
) error {
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	if len(reads) != len(models) {
		return errors.New("compiled model read projection count does not match compiled models")
	}
	if err := chargeCompiledModelProjectionWork(models, work); err != nil {
		return err
	}
	for i := range reads {
		read, model := reads[i], models[i]
		if read.Start != model.Start || read.AllBitLen != model.AllBitLen ||
			read.Kind != model.Kind || read.Empty != model.Empty ||
			!equalCompiledModelRowReadsForSource(read.Rows, model.Rows) ||
			!equalCompiledAllTermReadsForSource(read.All, model.All) {
			return errors.New("compiled model read projection does not match compiled model")
		}
	}
	return nil
}

func equalCompiledModelRowReadsForSource(reads []compiledModelRowRead, rows []CompiledModelRow) bool {
	if len(reads) != len(rows) {
		return false
	}
	for i, read := range reads {
		row := rows[i]
		if read.Min != row.Min || read.Max != row.Max || read.Accept != row.Accept ||
			read.Counted != row.Counted || read.Unbounded != row.Unbounded ||
			read.CountParticle != newCompiledParticleRead(row.CountParticle) ||
			!equalDFARowIndex(read.Index, row.Index) || len(read.Edges) != len(row.Edges) {
			return false
		}
		for j, edge := range read.Edges {
			source := row.Edges[j]
			if edge.To != source.To || edge.Particle != newCompiledParticleRead(source.Particle) {
				return false
			}
		}
	}
	return true
}

func equalCompiledAllTermReadsForSource(reads []compiledAllTermRead, terms []CompiledAllTerm) bool {
	if len(reads) != len(terms) {
		return false
	}
	for i, read := range reads {
		if read.Required != terms[i].Required || read.Particle != newCompiledParticleRead(terms[i].Particle) {
			return false
		}
	}
	return true
}

func sameCompiledParticleRead(a, b compiledParticleRead) bool {
	return a == b
}

// ContentState returns the initial mutable content-model state for the frame.
func (f ContentFrame) ContentState() ContentState {
	return f.state
}

// AllBitLen returns the number of all-group bit words required by the frame.
func (f ContentFrame) AllBitLen() int {
	return f.allLen
}

// NoContentMatch returns the empty content-model match.
func NoContentMatch() ContentMatch {
	return ContentMatch{Element: NoElement}
}

// ContentFrame derives the initial content state directly from a published
// schema. Publication guarantees referenced model IDs are valid.
func (rt *Schema) ContentFrame(typ TypeID) ContentFrame {
	modelID := rt.ContentModelForType(typ)
	frame := ContentFrame{state: ContentState{model: modelID, present: true}}
	if modelID == NoContentModel {
		return frame
	}
	model := rt.runtime.CompiledModels[modelID]
	frame.state.state = model.Start
	frame.allLen = int(model.AllBitLen)
	return frame
}

// NextContent derives one transition from published schema slices without
// mutating the parent state or caller-owned all-group scratch.
func (rt *Schema) NextContent(st ContentState, in ContentInput, scratch *ContentScratch) (ContentTransition, ContentTransitionStatus) {
	if !st.HasModel() {
		return ContentTransition{}, ContentTransitionInvalid
	}
	if !ValidContentModelID(st.model, len(rt.runtime.CompiledModels)) {
		return ContentTransition{}, ContentTransitionInvalid
	}
	model := &rt.runtime.CompiledModels[st.model]
	switch model.Kind {
	case CompiledModelAny:
		return newContentTransition(st, st, rt.matchPublishedAnyContent(in)), ContentTransitionMatched
	case CompiledModelAll:
		return rt.nextPublishedAllContent(st, model, in, scratch)
	case CompiledModelDFA:
		return rt.nextPublishedDFAContent(st, model, in)
	default:
		return ContentTransition{}, ContentTransitionInvalid
	}
}

func newContentTransition(from, next ContentState, match ContentMatch) ContentTransition {
	return ContentTransition{match: match, from: from, next: next, valid: true}
}

// CompleteContent reports whether a freeze-validated content state may end.
func (rt *Schema) CompleteContent(st ContentState, scratch *ContentScratch) ContentCompletionStatus {
	if !st.HasModel() {
		return ContentCompletionInvalid
	}
	if !ValidContentModelID(st.model, len(rt.runtime.CompiledModels)) {
		return ContentCompletionInvalid
	}
	model := &rt.runtime.CompiledModels[st.model]
	switch model.Kind {
	case CompiledModelEmpty, CompiledModelAny:
		return ContentCompletionComplete
	case CompiledModelAll:
		return completePublishedAllContent(model, scratch)
	case CompiledModelDFA:
		return completePublishedDFAContent(st, model)
	default:
		return ContentCompletionInvalid
	}
}

func (rt *Schema) matchPublishedAnyContent(in ContentInput) ContentMatch {
	if in.Name.Known {
		if id, ok := rt.runtime.GlobalElements[in.Name.Name]; ok {
			return ContentMatch{Element: id}
		}
	}
	return ContentMatch{Element: NoElement}
}

func (rt *Schema) nextPublishedAllContent(st ContentState, model *compiledModelRead, in ContentInput, scratch *ContentScratch) (ContentTransition, ContentTransitionStatus) {
	for i, term := range model.All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if seen {
			continue
		}
		match, matched, valid := rt.matchPublishedDirectParticle(term.Particle, in)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if !matched {
			continue
		}
		transition := newContentTransition(st, st, match)
		transition.allIndex = i
		transition.markAll = true
		return transition, ContentTransitionMatched
	}
	return ContentTransition{}, ContentTransitionNoMatch
}

func completePublishedAllContent(model *compiledModelRead, scratch *ContentScratch) ContentCompletionStatus {
	empty := true
	missingRequired := false
	for i, term := range model.All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return ContentCompletionInvalid
		}
		if seen {
			empty = false
			continue
		}
		if term.Required {
			missingRequired = true
		}
	}
	if empty && model.Empty {
		return ContentCompletionComplete
	}
	if empty || missingRequired {
		return ContentCompletionIncomplete
	}
	return ContentCompletionComplete
}

func (rt *Schema) nextPublishedDFAContent(st ContentState, model *compiledModelRead, in ContentInput) (ContentTransition, ContentTransitionStatus) {
	if !ValidUint32Index(st.state, len(model.Rows)) {
		return ContentTransition{}, ContentTransitionInvalid
	}
	row := &model.Rows[st.state]
	if row.Index.IsEnabled() {
		return rt.nextPublishedIndexedDFAContent(st, model, row, row.Index, in)
	}
	for _, edge := range row.Edges {
		match, matched, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if !matched {
			continue
		}
		next, ok := nextPublishedDFAState(st, model, edge)
		if !ok {
			continue
		}
		return newContentTransition(st, next, match), ContentTransitionMatched
	}
	return ContentTransition{}, ContentTransitionNoMatch
}

func completePublishedDFAContent(st ContentState, model *compiledModelRead) ContentCompletionStatus {
	if !ValidUint32Index(st.state, len(model.Rows)) {
		return ContentCompletionInvalid
	}
	row := &model.Rows[st.state]
	if row.Accept && (!row.Counted || st.count >= row.Min) {
		return ContentCompletionComplete
	}
	return ContentCompletionIncomplete
}

func (rt *Schema) nextPublishedIndexedDFAContent(st ContentState, model *compiledModelRead, row *compiledModelRowRead, idx DFARowIndex, in ContentInput) (ContentTransition, ContentTransitionStatus) {
	elemPos := -1
	if in.Name.Known {
		if pos, ok := idx.NameToEdge[in.Name.Name]; ok {
			elemPos = int(pos)
		}
	}
	wi := 0
	for {
		var pos int
		switch {
		case wi < len(idx.WildcardEdges) && (elemPos < 0 || int(idx.WildcardEdges[wi]) < elemPos):
			pos = int(idx.WildcardEdges[wi])
			wi++
		case elemPos >= 0:
			pos = elemPos
			elemPos = -1
		default:
			return ContentTransition{}, ContentTransitionNoMatch
		}
		edge := row.Edges[pos]
		match, matched, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return ContentTransition{}, ContentTransitionInvalid
		}
		if !matched {
			continue
		}
		next, ok := nextPublishedDFAState(st, model, edge)
		if !ok {
			continue
		}
		return newContentTransition(st, next, match), ContentTransitionMatched
	}
}

func (rt *Schema) matchPublishedDirectParticle(p compiledParticleRead, in ContentInput) (ContentMatch, bool, bool) {
	switch p.Kind {
	case ParticleElement:
		name, ok := rt.runtime.Elements.name(p.Element)
		if !ok {
			return NoContentMatch(), false, false
		}
		if in.Name.Known {
			if name == in.Name.Name {
				return ContentMatch{Element: p.Element}, true, true
			}
			if member, ok := rt.runtime.Substitutions.MemberByName(p.Element, in.Name.Name); ok {
				return ContentMatch{Element: member}, true, true
			}
		}
	case ParticleWildcard:
		if !ValidWildcardID(p.Wildcard, len(rt.runtime.Wildcards)) {
			return NoContentMatch(), false, false
		}
		match, matched := rt.matchPublishedWildcardParticle(rt.runtime.Wildcards[p.Wildcard], in)
		return match, matched, true
	default:
		return NoContentMatch(), false, false
	}
	return NoContentMatch(), false, true
}

func (rt *Schema) matchPublishedWildcardParticle(w WildcardView, in ContentInput) (ContentMatch, bool) {
	if !w.AllowsURI(in.Name.NS) {
		return NoContentMatch(), false
	}
	switch w.Process() {
	case ProcessStrict:
		if in.Name.Known {
			if id, ok := rt.runtime.GlobalElements[in.Name.Name]; ok {
				return ContentMatch{Element: id}, true
			}
		}
		if in.HasXSIType {
			return NoContentMatch(), true
		}
		return ContentMatch{Element: NoElement, StrictMissing: true}, true
	case ProcessSkip:
		return ContentMatch{Element: NoElement, Skip: true}, true
	case ProcessLax:
		if in.Name.Known {
			if id, ok := rt.runtime.GlobalElements[in.Name.Name]; ok {
				return ContentMatch{Element: id}, true
			}
		}
	}
	return NoContentMatch(), true
}

func nextPublishedDFAState(st ContentState, model *compiledModelRead, edge compiledModelEdgeRead) (ContentState, bool) {
	to := edge.To
	from := &model.Rows[st.state]
	next := &model.Rows[to]
	count := uint32(0)
	if from.Counted && to == st.state && sameCompiledParticleRead(edge.Particle, from.CountParticle) {
		if !from.Unbounded && st.count >= from.Max {
			return ContentState{}, false
		}
		count = st.count
		if count != math.MaxUint32 {
			count++
		}
	} else {
		if from.Counted && st.count < from.Min {
			return ContentState{}, false
		}
		if next.Counted && sameCompiledParticleRead(edge.Particle, next.CountParticle) {
			count = 1
		}
	}
	st.state = to
	st.count = count
	return st, true
}
