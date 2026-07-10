package runtime

import (
	"errors"
	"math"
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

// ContentAdvanceStatus reports the outcome of one content-model transition.
type ContentAdvanceStatus uint8

const (
	// ContentAdvanceInvalid reports invalid published metadata or state.
	ContentAdvanceInvalid ContentAdvanceStatus = iota
	// ContentAdvanceNoMatch reports a valid state with no matching transition.
	ContentAdvanceNoMatch
	// ContentAdvanceMatched reports a committed matching transition.
	ContentAdvanceMatched
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

// CompiledModelView is a read-only validation view over a frozen compiled
// content model.
type CompiledModelView struct {
	model *CompiledModel
}

// NewBorrowedCompiledModelViews returns validation views over immutable compiled
// content models owned by a published schema.
func NewBorrowedCompiledModelViews(models []CompiledModel) []CompiledModelView {
	out := make([]CompiledModelView, len(models))
	for i := range models {
		out[i] = CompiledModelView{model: &models[i]}
	}
	return out
}

var emptyCompiledModelViewModel CompiledModel

// EqualCompiledModelViewProjection reports whether view matches the validation
// view derived from model.
func EqualCompiledModelViewProjection(view CompiledModelView, model *CompiledModel) bool {
	if model == nil {
		return EqualCompiledModels(*view.compiled(), CompiledModel{})
	}
	return EqualCompiledModels(*view.compiled(), *model)
}

// EqualCompiledModelViewProjectionTable reports whether views match validation
// views derived from models.
func EqualCompiledModelViewProjectionTable(views []CompiledModelView, models []CompiledModel) bool {
	if len(views) != len(models) {
		return false
	}
	for i := range views {
		if !EqualCompiledModelViewProjection(views[i], &models[i]) {
			return false
		}
	}
	return true
}

// ValidateCompiledModelViewProjectionTable validates compiled-model read
// projections against frozen compiled model records.
func ValidateCompiledModelViewProjectionTable(views []CompiledModelView, models []CompiledModel) error {
	if len(views) != len(models) {
		return errors.New("compiled model view projection count does not match compiled models")
	}
	if !EqualCompiledModelViewProjectionTable(views, models) {
		return errors.New("compiled model view projection does not match compiled model")
	}
	return nil
}

// CompiledModelViewByID returns a validation compiled-model view from the
// frozen compiled-model view projection table.
func CompiledModelViewByID(views []CompiledModelView, id ContentModelID) (CompiledModelView, bool) {
	if !ValidContentModelID(id, len(views)) {
		return CompiledModelView{}, false
	}
	return views[id], true
}

func (v CompiledModelView) compiled() *CompiledModel {
	if v.model == nil {
		return &emptyCompiledModelViewModel
	}
	return v.model
}

func (v CompiledModelView) start() uint32 {
	return v.compiled().Start
}

func (v CompiledModelView) allBitLen() uint32 {
	return v.compiled().AllBitLen
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
	frame.state.state = model.start()
	frame.allLen = int(model.allBitLen())
	return frame
}

// AdvanceContent advances one freeze-validated content-model state directly
// from published schema slices.
func (rt *Schema) AdvanceContent(st *ContentState, in ContentInput, scratch *ContentScratch) (ContentMatch, ContentAdvanceStatus) {
	if st == nil {
		return NoContentMatch(), ContentAdvanceInvalid
	}
	if !st.HasModel() {
		return NoContentMatch(), ContentAdvanceInvalid
	}
	if !ValidContentModelID(st.model, len(rt.runtime.CompiledModels)) {
		return NoContentMatch(), ContentAdvanceInvalid
	}
	model := rt.runtime.CompiledModels[st.model].compiled()
	switch model.Kind {
	case CompiledModelAny:
		return rt.matchPublishedAnyContent(in), ContentAdvanceMatched
	case CompiledModelAll:
		return rt.advancePublishedAllContent(model, in, scratch)
	case CompiledModelDFA:
		return rt.advancePublishedDFAContent(st, model, in)
	default:
		return NoContentMatch(), ContentAdvanceInvalid
	}
}

// CompleteContent reports whether a freeze-validated content state may end.
func (rt *Schema) CompleteContent(st ContentState, scratch *ContentScratch) ContentCompletionStatus {
	if !st.HasModel() {
		return ContentCompletionInvalid
	}
	if !ValidContentModelID(st.model, len(rt.runtime.CompiledModels)) {
		return ContentCompletionInvalid
	}
	model := rt.runtime.CompiledModels[st.model].compiled()
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

func (rt *Schema) advancePublishedAllContent(model *CompiledModel, in ContentInput, scratch *ContentScratch) (ContentMatch, ContentAdvanceStatus) {
	for i, term := range model.All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return NoContentMatch(), ContentAdvanceInvalid
		}
		if seen {
			continue
		}
		match, matched, valid := rt.matchPublishedDirectParticle(term.Particle, in)
		if !valid {
			return NoContentMatch(), ContentAdvanceInvalid
		}
		if !matched {
			continue
		}
		if !scratch.SetAllSeen(i) {
			return NoContentMatch(), ContentAdvanceInvalid
		}
		return match, ContentAdvanceMatched
	}
	return NoContentMatch(), ContentAdvanceNoMatch
}

func completePublishedAllContent(model *CompiledModel, scratch *ContentScratch) ContentCompletionStatus {
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

func (rt *Schema) advancePublishedDFAContent(st *ContentState, model *CompiledModel, in ContentInput) (ContentMatch, ContentAdvanceStatus) {
	if !ValidUint32Index(st.state, len(model.Rows)) {
		return NoContentMatch(), ContentAdvanceInvalid
	}
	row := &model.Rows[st.state]
	if row.Index.IsEnabled() {
		return rt.advancePublishedIndexedDFAContent(st, model, row, row.Index, in)
	}
	for _, edge := range row.Edges {
		match, matched, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return NoContentMatch(), ContentAdvanceInvalid
		}
		if !matched {
			continue
		}
		if !advancePublishedDFAState(st, model, edge) {
			continue
		}
		return match, ContentAdvanceMatched
	}
	return NoContentMatch(), ContentAdvanceNoMatch
}

func completePublishedDFAContent(st ContentState, model *CompiledModel) ContentCompletionStatus {
	if !ValidUint32Index(st.state, len(model.Rows)) {
		return ContentCompletionInvalid
	}
	row := &model.Rows[st.state]
	if row.Accept && (!row.Counted || st.count >= row.Min) {
		return ContentCompletionComplete
	}
	return ContentCompletionIncomplete
}

func (rt *Schema) advancePublishedIndexedDFAContent(st *ContentState, model *CompiledModel, row *CompiledModelRow, idx DFARowIndex, in ContentInput) (ContentMatch, ContentAdvanceStatus) {
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
			return NoContentMatch(), ContentAdvanceNoMatch
		}
		edge := row.Edges[pos]
		match, matched, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return NoContentMatch(), ContentAdvanceInvalid
		}
		if !matched {
			continue
		}
		if !advancePublishedDFAState(st, model, edge) {
			continue
		}
		return match, ContentAdvanceMatched
	}
}

func (rt *Schema) matchPublishedDirectParticle(p Particle, in ContentInput) (ContentMatch, bool, bool) {
	switch p.Kind {
	case ParticleElement:
		if !ValidElementID(p.Element, len(rt.runtime.ElementNames)) {
			return NoContentMatch(), false, false
		}
		name := rt.runtime.ElementNames[p.Element]
		if in.Name.Known {
			if name == in.Name.Name {
				return ContentMatch{Element: p.Element}, true, true
			}
			if byName := rt.runtime.SubstitutionLookup[p.Element]; byName != nil {
				if member, ok := byName[in.Name.Name]; ok {
					return ContentMatch{Element: member}, true, true
				}
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

func advancePublishedDFAState(st *ContentState, model *CompiledModel, edge CompiledModelEdge) bool {
	to := edge.To
	from := &model.Rows[st.state]
	next := &model.Rows[to]
	count := uint32(0)
	if from.Counted && to == st.state && SameCompiledParticle(edge.Particle, from.CountParticle) {
		if !from.Unbounded && st.count >= from.Max {
			return false
		}
		count = st.count
		if count != math.MaxUint32 {
			count++
		}
	} else {
		if from.Counted && st.count < from.Min {
			return false
		}
		if next.Counted && SameCompiledParticle(edge.Particle, next.CountParticle) {
			count = 1
		}
	}
	st.state = to
	st.count = count
	return true
}
