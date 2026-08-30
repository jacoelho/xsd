package runtime

import (
	"errors"
	"maps"
	"slices"
)

// CompiledModelKind identifies the runtime representation used for a compiled
// content model.
type CompiledModelKind uint8

const (
	// CompiledModelEmpty is the compiled representation for empty content.
	CompiledModelEmpty CompiledModelKind = iota
	// CompiledModelAny is the compiled representation for xs:anyType content.
	CompiledModelAny
	// CompiledModelAll is the compiled representation for xs:all content.
	CompiledModelAll
	// CompiledModelDFA is the compiled DFA representation.
	CompiledModelDFA
)

// ValidCompiledModelKind reports whether kind is a known compiled-model kind.
func ValidCompiledModelKind(kind CompiledModelKind) bool {
	switch kind {
	case CompiledModelEmpty, CompiledModelAny, CompiledModelAll, CompiledModelDFA:
		return true
	default:
		return false
	}
}

// CompiledModel stores a runtime-ready content model.
type CompiledModel struct {
	Rows      []CompiledModelRow
	All       []CompiledAllTerm
	Source    ContentModelID
	Start     uint32
	AllBitLen uint32
	Kind      CompiledModelKind
	Mixed     bool
	Empty     bool
}

type dfaRowIndex struct {
	nameToEdge    map[QName]uint32
	wildcardEdges []uint32
}

func (idx dfaRowIndex) enabled() bool {
	return idx.nameToEdge != nil
}

// CompiledModelRow stores a compiled DFA state.
type CompiledModelRow struct {
	Edges         []CompiledModelEdge
	index         dfaRowIndex
	CountParticle Particle
	Min           uint32
	Max           uint32
	Accept        bool
	Counted       bool
	Unbounded     bool
}

// CompiledModelEdge stores one transition in a compiled DFA row.
type CompiledModelEdge struct {
	Particle Particle
	To       uint32
}

// CompiledAllTerm stores one xs:all term in compiled form.
type CompiledAllTerm struct {
	Particle Particle
	Required bool
}

// CompiledModelRuntime supplies runtime metadata needed to validate a compiled
// content model.
type CompiledModelRuntime interface {
	ParticleRuntime
	DFARowIndexRuntime
}

// SameCompiledParticle reports whether two compiled particles reference the
// same runtime term. Occurrence and nested model IDs are intentionally ignored:
// compiled transitions contain direct element or wildcard particles.
func SameCompiledParticle(a, b Particle) bool {
	return a.Kind == b.Kind && a.Element == b.Element && a.Wildcard == b.Wildcard
}

func equalDFARowIndex(a, b dfaRowIndex) bool {
	return maps.Equal(a.nameToEdge, b.nameToEdge) &&
		slices.Equal(a.wildcardEdges, b.wildcardEdges)
}

const compiledDFARowIndexMinEdges = 8

type dfaRowIndexAnalysis struct {
	rt      DFARowIndexRuntime
	work    ContentModelWork
	entries map[ElementID][]QName
}

// IndexCompiledModelsRows builds row indexes for a model table and
// reuses each substitution head's expanded entries across the table.
func IndexCompiledModelsRows(rt DFARowIndexRuntime, models []CompiledModel, work ContentModelWork) error {
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

func newDFARowIndexAnalysis(rt DFARowIndexRuntime, work ContentModelWork) *dfaRowIndexAnalysis {
	return &dfaRowIndexAnalysis{
		rt:      rt,
		work:    work,
		entries: make(map[ElementID][]QName),
	}
}

func (a *dfaRowIndexAnalysis) indexModel(model *CompiledModel) error {
	if model.Kind != CompiledModelDFA {
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

func (a *dfaRowIndexAnalysis) indexRow(row *CompiledModelRow) error {
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
	edge CompiledModelEdge,
) (bool, error) {
	switch edge.Particle.Kind {
	case ParticleElement:
		return a.indexElementEdge(index, position, edge.Particle.Element)
	case ParticleWildcard:
		*wildcards = append(*wildcards, position)
		return true, nil
	case ParticleModel:
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
	a.rt.ForEachSubstitutionEntry(id, func(name QName, _ ElementID) bool {
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

// ValidateCompiledModelsRuntime validates table-level and per-model runtime
// invariants for compiled content models. It does not recompile source models;
// callers that own compilation can perform that stronger derivation check
// separately.
func ValidateCompiledModelsRuntime(
	names *NameTable,
	rt CompiledModelRuntime,
	sources []ContentModel,
	models []CompiledModel,
	work ContentModelWork,
	analysis *ContentModelAnalysis,
) error {
	validator, err := newCompiledModelValidator(names, rt, work, analysis)
	if err != nil {
		return err
	}
	return validator.validateSet(sources, models)
}

type compiledModelValidator struct {
	names   *NameTable
	rt      CompiledModelRuntime
	work    ContentModelWork
	overlap *ContentModelAnalysis
	indexes *dfaRowIndexAnalysis
}

func newCompiledModelValidator(
	names *NameTable,
	rt CompiledModelRuntime,
	work ContentModelWork,
	overlap *ContentModelAnalysis,
) (compiledModelValidator, error) {
	if err := requireContentModelWork(work); err != nil {
		return compiledModelValidator{}, err
	}
	if overlap == nil {
		return compiledModelValidator{}, errors.New("compiled content model validation requires content model analysis")
	}
	return compiledModelValidator{
		names:   names,
		rt:      rt,
		work:    work,
		overlap: overlap,
		indexes: newDFARowIndexAnalysis(rt, work),
	}, nil
}

func (v *compiledModelValidator) validateSet(
	sources []ContentModel,
	models []CompiledModel,
) error {
	if len(models) != len(sources) {
		return errors.New("compiled content model count does not match model count")
	}
	for i, model := range models {
		if err := spendContentModelWork(v.work); err != nil {
			return err
		}
		if err := v.validateSlot(
			ContentModelID(i),
			sources[i],
			model,
		); err != nil {
			return err
		}
	}
	return nil
}

// CompiledModelRuntimeValidation is the complete input for validating one
// compiled content-model slot.
type CompiledModelRuntimeValidation struct {
	Runtime  CompiledModelRuntime
	Work     ContentModelWork
	Names    *NameTable
	Analysis *ContentModelAnalysis
	Source   ContentModel
	Model    CompiledModel
	ID       ContentModelID
}

// ValidateCompiledModelRuntime validates runtime invariants for a compiled
// content model against the source content-model slot. It does not recompile
// the source model; callers that own compilation can perform that stronger
// derivation check separately.
func ValidateCompiledModelRuntime(input CompiledModelRuntimeValidation) error {
	validator, err := newCompiledModelValidator(input.Names, input.Runtime, input.Work, input.Analysis)
	if err != nil {
		return err
	}
	return validator.validateSlot(input.ID, input.Source, input.Model)
}

func (v *compiledModelValidator) validateSlot(
	id ContentModelID,
	source ContentModel,
	model CompiledModel,
) error {
	if err := validateCompiledModelIdentity(id, source, model); err != nil {
		return err
	}
	switch model.Kind {
	case CompiledModelEmpty, CompiledModelAny:
		return validateCompiledEmptyOrAnyRuntime(source, model)
	case CompiledModelAll:
		return v.validateAllModel(source, model)
	case CompiledModelDFA:
		return v.validateDFAModel(source, model)
	}
	return nil
}

func validateCompiledModelIdentity(id ContentModelID, source ContentModel, model CompiledModel) error {
	if model.Source != id {
		return errors.New("compiled content model source does not match model slot")
	}
	if model.Mixed != source.Mixed {
		return errors.New("compiled content model mixed flag does not match source model")
	}
	if !ValidCompiledModelKind(model.Kind) {
		return errors.New("compiled content model has invalid kind")
	}
	return nil
}

func validateCompiledEmptyOrAnyRuntime(source ContentModel, model CompiledModel) error {
	if (source.Kind == ModelEmpty) != (model.Kind == CompiledModelEmpty) ||
		(source.Kind == ModelAny) != (model.Kind == CompiledModelAny) {
		return errors.New("compiled content model kind does not match source model")
	}
	if len(model.Rows) != 0 || len(model.All) != 0 || model.Start != 0 || model.AllBitLen != 0 || !model.Empty {
		return errors.New("compiled empty/any content model stores inactive fields")
	}
	return nil
}

func (v *compiledModelValidator) validateAllModel(
	source ContentModel,
	model CompiledModel,
) error {
	if source.Kind != ModelAll {
		return errors.New("compiled all content model kind does not match source model")
	}
	if len(model.Rows) != 0 || model.Start != 0 {
		return errors.New("compiled all content model stores inactive DFA fields")
	}
	if err := validateCompiledAllRuntime(source, model, v.work); err != nil {
		return err
	}
	for _, term := range model.All {
		if err := spendContentModelWork(v.work); err != nil {
			return err
		}
		if err := validateCompiledParticle(v.rt, term.Particle); err != nil {
			return err
		}
	}
	return nil
}

func (v *compiledModelValidator) validateDFAModel(
	source ContentModel,
	model CompiledModel,
) error {
	if source.Kind == ModelEmpty || source.Kind == ModelAny || source.Kind == ModelAll {
		return errors.New("compiled DFA content model kind does not match source model")
	}
	if len(model.All) != 0 || model.AllBitLen != 0 {
		return errors.New("compiled DFA content model stores inactive all fields")
	}
	return v.validateDFA(model)
}

func validateCompiledAllRuntime(source ContentModel, model CompiledModel, work ContentModelWork) error {
	if err := validateCompiledAllShape(source, model); err != nil {
		return err
	}
	required, err := validateCompiledAllTerms(source.Particles, model.All, work)
	if err != nil {
		return err
	}
	if model.Empty != (source.Occurs.Min == 0 || !required) {
		return errors.New("compiled all content model empty flag does not match source model")
	}
	return nil
}

func validateCompiledAllShape(source ContentModel, model CompiledModel) error {
	if len(model.All) != len(source.Particles) {
		return errors.New("compiled all content model term count does not match source model")
	}
	allBitLen := (len(model.All) + 63) / 64
	if allBitLen > int(^uint32(0)) || model.AllBitLen != uint32(allBitLen) {
		return errors.New("compiled all content model bit length does not match terms")
	}
	return nil
}

func validateCompiledAllTerms(source []Particle, terms []CompiledAllTerm, work ContentModelWork) (bool, error) {
	required := false
	for i, term := range terms {
		if err := spendContentModelWork(work); err != nil {
			return false, err
		}
		termRequired, err := validateCompiledAllTerm(source[i], term)
		if err != nil {
			return false, err
		}
		if termRequired {
			required = true
		}
	}
	return required, nil
}

func validateCompiledAllTerm(source Particle, term CompiledAllTerm) (bool, error) {
	if source.Kind == ParticleModel {
		return false, errors.New("compiled all content model source has model particle")
	}
	if !SameCompiledParticle(term.Particle, source) || term.Particle.Occurs != source.Occurs {
		return false, errors.New("compiled all content model term does not match source particle")
	}
	if term.Required != (source.Occurs.Min > 0) {
		return false, errors.New("compiled all content model required flag does not match source particle")
	}
	return term.Required, nil
}

func (v *compiledModelValidator) validateDFA(model CompiledModel) error {
	if !ValidUint32Index(model.Start, len(model.Rows)) {
		return errors.New("compiled content model start state is invalid")
	}
	if model.Empty != model.Rows[model.Start].Accept {
		return errors.New("compiled content model empty flag does not match start row")
	}
	for i, row := range model.Rows {
		if err := spendContentModelWork(v.work); err != nil {
			return err
		}
		if uint64(i) > uint64(^uint32(0)) {
			return errors.New("compiled content model row index is invalid")
		}
		if err := v.validateDFARow(model, row, uint32(i)); err != nil {
			return err
		}
	}
	return nil
}

func (v *compiledModelValidator) validateDFARow(
	model CompiledModel,
	row CompiledModelRow,
	index uint32,
) error {
	if err := validateCompiledCountedRow(v.rt, row); err != nil {
		return err
	}
	countedLoops, err := validateCompiledDFAEdges(v.rt, model, row, index, v.work)
	if err != nil {
		return err
	}
	if row.Counted && countedLoops != 1 {
		return errors.New("compiled content model counted state must have one counted self loop")
	}
	if err := validateCompiledDFARowUPA(row, index, v.work, v.overlap); err != nil {
		return err
	}
	if row.index.enabled() {
		return v.indexes.validateRow(v.names, row)
	}
	return nil
}

func validateCompiledCountedRow(rt CompiledModelRuntime, row CompiledModelRow) error {
	if row.Counted && !row.Unbounded && row.Max < row.Min {
		return errors.New("compiled content model counted state has invalid range")
	}
	if row.Counted {
		return validateCompiledParticle(rt, row.CountParticle)
	}
	return nil
}

func validateCompiledDFAEdges(
	rt CompiledModelRuntime,
	model CompiledModel,
	row CompiledModelRow,
	index uint32,
	work ContentModelWork,
) (int, error) {
	countedLoops := 0
	for _, edge := range row.Edges {
		if err := spendContentModelWork(work); err != nil {
			return 0, err
		}
		counted, err := validateCompiledDFAEdge(rt, model, row, index, edge)
		if err != nil {
			return 0, err
		}
		if counted {
			countedLoops++
		}
	}
	return countedLoops, nil
}

func validateCompiledDFAEdge(
	rt CompiledModelRuntime,
	model CompiledModel,
	row CompiledModelRow,
	index uint32,
	edge CompiledModelEdge,
) (bool, error) {
	if !ValidUint32Index(edge.To, len(model.Rows)) {
		return false, errors.New("compiled content model edge target is invalid")
	}
	if err := validateCompiledParticle(rt, edge.Particle); err != nil {
		return false, err
	}
	if !row.Counted || edge.To != index {
		return false, nil
	}
	if !SameCompiledParticle(edge.Particle, row.CountParticle) {
		return false, errors.New("compiled content model counted state has non-counted self loop")
	}
	return true, nil
}

func validateCompiledDFARowUPA(
	row CompiledModelRow,
	index uint32,
	work ContentModelWork,
	overlap *ContentModelAnalysis,
) error {
	for i, a := range row.Edges {
		if err := validateCompiledDFAEdgeOverlaps(row, index, i, a, work, overlap); err != nil {
			return err
		}
	}
	return nil
}

func validateCompiledDFAEdgeOverlaps(
	row CompiledModelRow,
	index uint32,
	position int,
	edge CompiledModelEdge,
	work ContentModelWork,
	overlap *ContentModelAnalysis,
) error {
	for nextPosition := position + 1; nextPosition < len(row.Edges); nextPosition++ {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
		next := row.Edges[nextPosition]
		_, overlaps, err := overlap.Overlap(edge.Particle, next.Particle)
		if err != nil {
			return err
		}
		if overlaps && !CompiledCountingException(index, row, edge, next) {
			return errors.New("compiled content model row has overlapping particles")
		}
	}
	return nil
}

// CompiledCountingException reports whether overlapping counted-row edges are
// the one deterministic loop/exit pair produced for a fixed repeated particle.
func CompiledCountingException(index uint32, row CompiledModelRow, a, b CompiledModelEdge) bool {
	if !row.Counted || row.Unbounded || row.Min != row.Max {
		return false
	}
	aLoop := a.To == index && SameCompiledParticle(a.Particle, row.CountParticle)
	bLoop := b.To == index && SameCompiledParticle(b.Particle, row.CountParticle)
	return (aLoop && b.To != index) || (bLoop && a.To != index)
}

func validateCompiledParticle(rt CompiledModelRuntime, p Particle) error {
	switch p.Kind {
	case ParticleElement:
		if _, ok := rt.ElementName(p.Element); !ok {
			return errors.New("compiled particle references invalid element")
		}
	case ParticleWildcard:
		if _, ok := rt.Wildcard(p.Wildcard); !ok {
			return errors.New("compiled particle references invalid wildcard")
		}
	case ParticleModel:
		return errors.New("compiled particle has invalid kind")
	default:
		err := errors.New("compiled particle has invalid kind")
		return err
	}
	return ValidateParticleShape(p)
}

// DFARowIndexRuntime supplies declarations needed to validate a DFA row index.
type DFARowIndexRuntime interface {
	ElementName(id ElementID) (QName, bool)
	SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool)
	ForEachSubstitutionEntry(id ElementID, fn func(QName, ElementID) bool)
}

func (a *dfaRowIndexAnalysis) validateRow(names *NameTable, row CompiledModelRow) error {
	idx := row.index
	if idx.nameToEdge == nil {
		return errors.New("compiled content model name index is nil")
	}
	if err := a.validateNameIndex(names, row); err != nil {
		return err
	}
	return a.validateIndexedEdges(row)
}

func (a *dfaRowIndexAnalysis) validateNameIndex(names *NameTable, row CompiledModelRow) error {
	idx := row.index
	for name, pos := range idx.nameToEdge {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		if err := a.validateNameIndexEntry(names, row, name, pos); err != nil {
			return err
		}
	}
	return nil
}

func (a *dfaRowIndexAnalysis) validateNameIndexEntry(
	names *NameTable,
	row CompiledModelRow,
	name QName,
	position uint32,
) error {
	if names == nil || !names.ValidQName(name) || !ValidUint32Index(position, len(row.Edges)) {
		return errors.New("compiled content model name index entry is invalid")
	}
	particle := row.Edges[position].Particle
	if particle.Kind != ParticleElement {
		return errors.New("compiled content model name index targets non-element edge")
	}
	elementName, ok := a.rt.ElementName(particle.Element)
	if !ok {
		return errors.New("compiled content model name index key does not match edge element")
	}
	if elementName == name {
		return nil
	}
	if _, ok := a.rt.SubstitutionMemberByName(particle.Element, name); !ok {
		return errors.New("compiled content model name index key does not match edge element")
	}
	return nil
}

func (a *dfaRowIndexAnalysis) validateIndexedEdges(row CompiledModelRow) error {
	idx := row.index
	wi := 0
	for pos, edge := range row.Edges {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		if err := a.validateIndexedEdge(idx, uint32(pos), edge, &wi); err != nil {
			return err
		}
	}
	if wi != len(idx.wildcardEdges) {
		return errors.New("compiled content model wildcard list does not match wildcard edges")
	}
	return nil
}

func (a *dfaRowIndexAnalysis) validateIndexedEdge(
	idx dfaRowIndex,
	position uint32,
	edge CompiledModelEdge,
	wildcardIndex *int,
) error {
	switch edge.Particle.Kind {
	case ParticleElement:
		return a.validateIndexedElementEdge(idx, position, edge.Particle.Element)
	case ParticleWildcard:
		if *wildcardIndex >= len(idx.wildcardEdges) || idx.wildcardEdges[*wildcardIndex] != position {
			return errors.New("compiled content model wildcard list does not match wildcard edges")
		}
		*wildcardIndex++
		return nil
	case ParticleModel:
		return errors.New("compiled content model indexed row has model edge")
	default:
		err := errors.New("compiled content model indexed row has model edge")
		return err
	}
}

func (a *dfaRowIndexAnalysis) validateIndexedElementEdge(
	idx dfaRowIndex,
	position uint32,
	element ElementID,
) error {
	name, ok := a.rt.ElementName(element)
	if !ok {
		return errors.New("compiled content model indexed row has invalid element edge")
	}
	if err := requireIndexedName(idx, name, position); err != nil {
		return err
	}
	entries, err := a.substitutionEntries(element)
	if err != nil {
		return err
	}
	return a.validateIndexedSubstitutionEntries(idx, position, entries)
}

func (a *dfaRowIndexAnalysis) validateIndexedSubstitutionEntries(
	idx dfaRowIndex,
	position uint32,
	entries []QName,
) error {
	for _, entry := range entries {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		if err := requireIndexedName(idx, entry, position); err != nil {
			return errors.New("compiled content model name index is missing element edge")
		}
	}
	return nil
}

func requireIndexedName(idx dfaRowIndex, name QName, pos uint32) error {
	if got, ok := idx.nameToEdge[name]; !ok || got != pos {
		return errors.New("compiled content model name index is missing element edge")
	}
	return nil
}
