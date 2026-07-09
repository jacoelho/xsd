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

// NewCompiledModelView returns a read-only validation view over model.
func NewCompiledModelView(model *CompiledModel) CompiledModelView {
	if model == nil {
		return CompiledModelView{}
	}
	cloned := CloneCompiledModel(*model)
	return CompiledModelView{model: &cloned}
}

// NewCompiledModelViews returns read-only validation views over compiled
// content models.
func NewCompiledModelViews(models []CompiledModel) []CompiledModelView {
	out := make([]CompiledModelView, len(models))
	for i := range models {
		out[i] = NewCompiledModelView(&models[i])
	}
	return out
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

// EqualCompiledModelViews reports whether two compiled-model validation views
// expose identical content-model state.
func EqualCompiledModelViews(a, b CompiledModelView) bool {
	return EqualCompiledModels(*a.compiled(), *b.compiled())
}

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

func (v CompiledModelView) kind() CompiledModelKind {
	return v.compiled().Kind
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

// CompiledContentRuntime supplies runtime facts needed to execute compiled
// content models.
type CompiledContentRuntime interface {
	CompiledContentModelView(id ContentModelID) (CompiledModelView, bool)
	GlobalElement(name QName) (ElementID, bool)
	ElementName(id ElementID) (QName, bool)
	WildcardView(id WildcardID) (WildcardView, bool)
	SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool)
}

// ContentFrameRuntime supplies runtime facts needed to initialize an element
// content frame.
type ContentFrameRuntime interface {
	ContentModelForType(id TypeID) ContentModelID
	CompiledContentModelView(id ContentModelID) (CompiledModelView, bool)
}

// NoContentMatch returns the empty content-model match.
func NoContentMatch() ContentMatch {
	return ContentMatch{Element: NoElement}
}

// ContentFrameForType derives the frame state needed to validate children of a
// runtime type.
func ContentFrameForType[RT ContentFrameRuntime](rt RT, typ TypeID) ContentFrame {
	modelID := rt.ContentModelForType(typ)
	frame := ContentFrame{state: ContentState{model: modelID, present: true}}
	if modelID == NoContentModel {
		return frame
	}
	model, ok := rt.CompiledContentModelView(modelID)
	if !ok {
		return frame
	}
	frame.state.state = model.start()
	frame.allLen = int(model.allBitLen())
	return frame
}

// ContentFrameForPublishedSchema derives a frame directly from freeze-validated
// published schema slices.
func (rt *Schema) ContentFrameForPublishedSchema(typ TypeID) ContentFrame {
	modelID := ContentModelForTypeByID(rt.reads.ComplexContentModelIDs, typ)
	frame := ContentFrame{state: ContentState{model: modelID, present: true}}
	if modelID == NoContentModel || !ValidContentModelID(modelID, len(rt.reads.CompiledModels)) {
		return frame
	}
	model := rt.reads.CompiledModels[modelID]
	frame.state.state = model.start()
	frame.allLen = int(model.allBitLen())
	return frame
}

// AdvanceCompiledContent advances one compiled content-model state.
func AdvanceCompiledContent[RT CompiledContentRuntime](rt RT, st *ContentState, in ContentInput, scratch *ContentScratch) (ContentMatch, bool, bool) {
	if st == nil {
		return NoContentMatch(), false, false
	}
	if !st.HasModel() {
		return NoContentMatch(), false, false
	}
	model, ok := rt.CompiledContentModelView(st.model)
	if !ok {
		return NoContentMatch(), false, false
	}
	switch model.kind() {
	case CompiledModelAny:
		return matchAnyContent(rt, in), true, true
	case CompiledModelAll:
		return advanceAllContent(rt, model, in, scratch)
	case CompiledModelDFA:
		return advanceDFAContent(rt, st, model, in)
	default:
		return NoContentMatch(), false, false
	}
}

// AdvancePublishedSchemaContent advances one freeze-validated content-model
// state directly from published schema slices.
func (rt *Schema) AdvancePublishedSchemaContent(st *ContentState, in ContentInput, scratch *ContentScratch) (ContentMatch, bool, bool) {
	if st == nil {
		return NoContentMatch(), false, false
	}
	if !st.HasModel() {
		return NoContentMatch(), false, false
	}
	if !ValidContentModelID(st.model, len(rt.reads.CompiledModels)) {
		return NoContentMatch(), false, false
	}
	model := rt.reads.CompiledModels[st.model].compiled()
	switch model.Kind {
	case CompiledModelAny:
		return rt.matchPublishedAnyContent(in), true, true
	case CompiledModelAll:
		return rt.advancePublishedAllContent(model, in, scratch)
	case CompiledModelDFA:
		return rt.advancePublishedDFAContent(st, model, in)
	default:
		return NoContentMatch(), false, false
	}
}

// CompleteCompiledContent reports whether a compiled content-model state may end.
func CompleteCompiledContent[RT CompiledContentRuntime](rt RT, st ContentState, scratch *ContentScratch) (bool, bool) {
	if !st.HasModel() {
		return false, false
	}
	model, ok := rt.CompiledContentModelView(st.model)
	if !ok {
		return false, false
	}
	switch model.kind() {
	case CompiledModelEmpty, CompiledModelAny:
		return true, true
	case CompiledModelAll:
		return completeAllContent(model, scratch)
	case CompiledModelDFA:
		return completeDFAContent(st, model)
	default:
		return false, false
	}
}

// CompletePublishedSchemaContent reports whether a freeze-validated content
// state may end directly from published schema slices.
func (rt *Schema) CompletePublishedSchemaContent(st ContentState, scratch *ContentScratch) (bool, bool) {
	if !st.HasModel() {
		return false, false
	}
	if !ValidContentModelID(st.model, len(rt.reads.CompiledModels)) {
		return false, false
	}
	model := rt.reads.CompiledModels[st.model].compiled()
	switch model.Kind {
	case CompiledModelEmpty, CompiledModelAny:
		return true, true
	case CompiledModelAll:
		return completePublishedAllContent(model, scratch)
	case CompiledModelDFA:
		return completePublishedDFAContent(st, model)
	default:
		return false, false
	}
}

func matchAnyContent[RT CompiledContentRuntime](rt RT, in ContentInput) ContentMatch {
	if in.Name.Known {
		if id, ok := rt.GlobalElement(in.Name.Name); ok {
			return ContentMatch{Element: id}
		}
	}
	return ContentMatch{Element: NoElement}
}

func (rt *Schema) matchPublishedAnyContent(in ContentInput) ContentMatch {
	if in.Name.Known {
		if id, ok := rt.reads.GlobalElements[in.Name.Name]; ok {
			return ContentMatch{Element: id}
		}
	}
	return ContentMatch{Element: NoElement}
}

func advanceAllContent[RT CompiledContentRuntime](rt RT, model CompiledModelView, in ContentInput, scratch *ContentScratch) (ContentMatch, bool, bool) {
	for i, term := range model.compiled().All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return NoContentMatch(), false, false
		}
		if seen {
			continue
		}
		match, matched, valid := matchDirectParticle(rt, term.Particle, in)
		if !valid {
			return NoContentMatch(), false, false
		}
		if !matched {
			continue
		}
		if !scratch.SetAllSeen(i) {
			return NoContentMatch(), false, false
		}
		return match, true, true
	}
	return NoContentMatch(), false, true
}

func (rt *Schema) advancePublishedAllContent(model *CompiledModel, in ContentInput, scratch *ContentScratch) (ContentMatch, bool, bool) {
	for i, term := range model.All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return NoContentMatch(), false, false
		}
		if seen {
			continue
		}
		match, matched, valid := rt.matchPublishedDirectParticle(term.Particle, in)
		if !valid {
			return NoContentMatch(), false, false
		}
		if !matched {
			continue
		}
		if !scratch.SetAllSeen(i) {
			return NoContentMatch(), false, false
		}
		return match, true, true
	}
	return NoContentMatch(), false, true
}

func completeAllContent(model CompiledModelView, scratch *ContentScratch) (bool, bool) {
	empty := true
	missingRequired := false
	for i, term := range model.compiled().All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return false, false
		}
		if seen {
			empty = false
			continue
		}
		if term.Required {
			missingRequired = true
		}
	}
	if empty && model.compiled().Empty {
		return true, true
	}
	if empty || missingRequired {
		return false, true
	}
	return true, true
}

func completePublishedAllContent(model *CompiledModel, scratch *ContentScratch) (bool, bool) {
	empty := true
	missingRequired := false
	for i, term := range model.All {
		seen, valid := scratch.AllSeen(i)
		if !valid {
			return false, false
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
		return true, true
	}
	if empty || missingRequired {
		return false, true
	}
	return true, true
}

func advanceDFAContent[RT CompiledContentRuntime](rt RT, st *ContentState, model CompiledModelView, in ContentInput) (ContentMatch, bool, bool) {
	if !ValidUint32Index(st.state, len(model.compiled().Rows)) {
		return NoContentMatch(), false, false
	}
	row := model.compiled().Rows[st.state]
	if row.Index.IsEnabled() {
		match, ok, valid := advanceIndexedDFAContent(rt, st, model, row, row.Index, in)
		return match, ok, valid
	}
	for _, edge := range row.Edges {
		match, matched, valid := matchDirectParticle(rt, edge.Particle, in)
		if !valid {
			return NoContentMatch(), false, false
		}
		if !matched {
			continue
		}
		if !advanceDFAState(st, model, edge) {
			continue
		}
		return match, true, true
	}
	return NoContentMatch(), false, true
}

func completeDFAContent(st ContentState, model CompiledModelView) (bool, bool) {
	if !ValidUint32Index(st.state, len(model.compiled().Rows)) {
		return false, false
	}
	row := model.compiled().Rows[st.state]
	return row.Accept && (!row.Counted || st.count >= row.Min), true
}

func (rt *Schema) advancePublishedDFAContent(st *ContentState, model *CompiledModel, in ContentInput) (ContentMatch, bool, bool) {
	if !ValidUint32Index(st.state, len(model.Rows)) {
		return NoContentMatch(), false, false
	}
	row := &model.Rows[st.state]
	if row.Index.IsEnabled() {
		return rt.advancePublishedIndexedDFAContent(st, model, row, row.Index, in)
	}
	for _, edge := range row.Edges {
		match, matched, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return NoContentMatch(), false, false
		}
		if !matched {
			continue
		}
		if !advancePublishedDFAState(st, model, edge) {
			continue
		}
		return match, true, true
	}
	return NoContentMatch(), false, true
}

func completePublishedDFAContent(st ContentState, model *CompiledModel) (bool, bool) {
	if !ValidUint32Index(st.state, len(model.Rows)) {
		return false, false
	}
	row := &model.Rows[st.state]
	return row.Accept && (!row.Counted || st.count >= row.Min), true
}

// advanceIndexedDFAContent tries the name-indexed element edge and the
// wildcard edges in ascending edge position, preserving linear-scan order.
func advanceIndexedDFAContent[RT CompiledContentRuntime](rt RT, st *ContentState, model CompiledModelView, row CompiledModelRow, idx DFARowIndex, in ContentInput) (ContentMatch, bool, bool) {
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
			return NoContentMatch(), false, true
		}
		edge := row.Edges[pos]
		match, matched, valid := matchDirectParticle(rt, edge.Particle, in)
		if !valid {
			return NoContentMatch(), false, false
		}
		if !matched {
			continue
		}
		if !advanceDFAState(st, model, edge) {
			continue
		}
		return match, true, true
	}
}

func (rt *Schema) advancePublishedIndexedDFAContent(st *ContentState, model *CompiledModel, row *CompiledModelRow, idx DFARowIndex, in ContentInput) (ContentMatch, bool, bool) {
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
			return NoContentMatch(), false, true
		}
		edge := row.Edges[pos]
		match, matched, valid := rt.matchPublishedDirectParticle(edge.Particle, in)
		if !valid {
			return NoContentMatch(), false, false
		}
		if !matched {
			continue
		}
		if !advancePublishedDFAState(st, model, edge) {
			continue
		}
		return match, true, true
	}
}

func matchDirectParticle[RT CompiledContentRuntime](rt RT, p Particle, in ContentInput) (ContentMatch, bool, bool) {
	switch p.Kind {
	case ParticleElement:
		name, ok := rt.ElementName(p.Element)
		if !ok {
			return NoContentMatch(), false, false
		}
		if in.Name.Known {
			if name == in.Name.Name {
				return ContentMatch{Element: p.Element}, true, true
			}
			if member, ok := rt.SubstitutionMemberByName(p.Element, in.Name.Name); ok {
				return ContentMatch{Element: member}, true, true
			}
		}
	case ParticleWildcard:
		w, ok := rt.WildcardView(p.Wildcard)
		if !ok {
			return NoContentMatch(), false, false
		}
		match, matched := matchWildcardParticle(rt, w, in)
		return match, matched, true
	default:
		return NoContentMatch(), false, false
	}
	return NoContentMatch(), false, true
}

func matchWildcardParticle[RT CompiledContentRuntime](rt RT, w WildcardView, in ContentInput) (ContentMatch, bool) {
	if !w.AllowsURI(in.Name.NS) {
		return NoContentMatch(), false
	}
	switch w.Process() {
	case ProcessStrict:
		if in.Name.Known {
			if id, ok := rt.GlobalElement(in.Name.Name); ok {
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
			if id, ok := rt.GlobalElement(in.Name.Name); ok {
				return ContentMatch{Element: id}, true
			}
		}
	}
	return NoContentMatch(), true
}

func (rt *Schema) matchPublishedDirectParticle(p Particle, in ContentInput) (ContentMatch, bool, bool) {
	switch p.Kind {
	case ParticleElement:
		if !ValidElementID(p.Element, len(rt.reads.ElementNames)) {
			return NoContentMatch(), false, false
		}
		name := rt.reads.ElementNames[p.Element]
		if in.Name.Known {
			if name == in.Name.Name {
				return ContentMatch{Element: p.Element}, true, true
			}
			if byName := rt.reads.SubstitutionLookup[p.Element]; byName != nil {
				if member, ok := byName[in.Name.Name]; ok {
					return ContentMatch{Element: member}, true, true
				}
			}
		}
	case ParticleWildcard:
		if !ValidWildcardID(p.Wildcard, len(rt.reads.Wildcards)) {
			return NoContentMatch(), false, false
		}
		match, matched := rt.matchPublishedWildcardParticle(rt.reads.Wildcards[p.Wildcard], in)
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
			if id, ok := rt.reads.GlobalElements[in.Name.Name]; ok {
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
			if id, ok := rt.reads.GlobalElements[in.Name.Name]; ok {
				return ContentMatch{Element: id}, true
			}
		}
	}
	return NoContentMatch(), true
}

// advanceDFAState indexes model.Rows without bounds checks: freeze validates
// the start state and every edge target, so st.state and edge.To are always in
// range for published runtime schemas.
func advanceDFAState(st *ContentState, model CompiledModelView, edge CompiledModelEdge) bool {
	to := edge.To
	from := model.compiled().Rows[st.state]
	next := model.compiled().Rows[to]
	count := uint32(0)
	if from.Counted && to == st.state && SameCompiledParticle(edge.Particle, from.CountParticle) {
		if !from.Unbounded && st.count >= from.Max {
			return false
		}
		// Saturate instead of wrapping: a saturated count satisfies any Min,
		// and bounded particles stop at the Max guard above before reaching it.
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
