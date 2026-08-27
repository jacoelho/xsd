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
	for range row.Edges {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
	}
	index := make(map[QName]uint32, len(row.Edges))
	var wildcards []uint32
	for pos, edge := range row.Edges {
		edgePos := uint32(pos)
		switch edge.Particle.Kind {
		case ParticleElement:
			name, ok := a.rt.ElementName(edge.Particle.Element)
			if !ok {
				return errors.New("compiled content model index references invalid element")
			}
			if !indexEdgeName(index, name, edgePos) {
				return nil
			}
			unique := true
			entries, err := a.substitutionEntries(edge.Particle.Element)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if err := spendContentModelWork(a.work); err != nil {
					return err
				}
				unique = indexEdgeName(index, entry, edgePos)
				if !unique {
					break
				}
			}
			if !unique {
				return nil
			}
		case ParticleWildcard:
			wildcards = append(wildcards, edgePos)
		case ParticleModel:
			return nil
		default:
			return errors.New("compiled content model index has invalid edge particle")
		}
	}
	row.index = dfaRowIndex{nameToEdge: index, wildcardEdges: wildcards}
	return nil
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
	return validateCompiledModelsRuntime(names, rt, sources, models, true, work, analysis)
}

func validateCompiledModelsRuntime(
	names *NameTable,
	rt CompiledModelRuntime,
	sources []ContentModel,
	models []CompiledModel,
	validateUPA bool,
	work ContentModelWork,
	analysis *ContentModelAnalysis,
) error {
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	if len(models) != len(sources) {
		return errors.New("compiled content model count does not match model count")
	}
	if analysis == nil {
		return errors.New("compiled content model validation requires content model analysis")
	}
	indexes := newDFARowIndexAnalysis(rt, work)
	for i, model := range models {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
		if err := validateCompiledModelRuntime(
			names,
			rt,
			ContentModelID(i),
			sources[i],
			model,
			validateUPA,
			work,
			analysis,
			indexes,
		); err != nil {
			return err
		}
	}
	return nil
}

// ValidateCompiledModelRuntime validates runtime invariants for a compiled
// content model against the source content-model slot. It does not recompile
// the source model; callers that own compilation can perform that stronger
// derivation check separately.
func ValidateCompiledModelRuntime(
	names *NameTable,
	rt CompiledModelRuntime,
	id ContentModelID,
	source ContentModel,
	model CompiledModel,
	work ContentModelWork,
	analysis *ContentModelAnalysis,
) error {
	if err := requireContentModelWork(work); err != nil {
		return err
	}
	return validateCompiledModelRuntime(
		names,
		rt,
		id,
		source,
		model,
		true,
		work,
		analysis,
		newDFARowIndexAnalysis(rt, work),
	)
}

func validateCompiledModelRuntime(
	names *NameTable,
	rt CompiledModelRuntime,
	id ContentModelID,
	source ContentModel,
	model CompiledModel,
	validateUPA bool,
	work ContentModelWork,
	overlap *ContentModelAnalysis,
	indexes *dfaRowIndexAnalysis,
) error {
	if model.Source != id {
		return errors.New("compiled content model source does not match model slot")
	}
	if model.Mixed != source.Mixed {
		return errors.New("compiled content model mixed flag does not match source model")
	}
	if !ValidCompiledModelKind(model.Kind) {
		return errors.New("compiled content model has invalid kind")
	}
	switch model.Kind {
	case CompiledModelEmpty, CompiledModelAny:
		if (source.Kind == ModelEmpty) != (model.Kind == CompiledModelEmpty) ||
			(source.Kind == ModelAny) != (model.Kind == CompiledModelAny) {
			return errors.New("compiled content model kind does not match source model")
		}
		if len(model.Rows) != 0 || len(model.All) != 0 || model.Start != 0 || model.AllBitLen != 0 || !model.Empty {
			return errors.New("compiled empty/any content model stores inactive fields")
		}
	case CompiledModelAll:
		if source.Kind != ModelAll {
			return errors.New("compiled all content model kind does not match source model")
		}
		if len(model.Rows) != 0 || model.Start != 0 {
			return errors.New("compiled all content model stores inactive DFA fields")
		}
		if err := validateCompiledAllRuntime(source, model, work); err != nil {
			return err
		}
		for _, term := range model.All {
			if err := spendContentModelWork(work); err != nil {
				return err
			}
			if err := validateCompiledParticle(rt, term.Particle); err != nil {
				return err
			}
		}
	case CompiledModelDFA:
		if source.Kind == ModelEmpty || source.Kind == ModelAny || source.Kind == ModelAll {
			return errors.New("compiled DFA content model kind does not match source model")
		}
		if len(model.All) != 0 || model.AllBitLen != 0 {
			return errors.New("compiled DFA content model stores inactive all fields")
		}
		if err := validateCompiledDFARuntime(names, rt, model, validateUPA, work, overlap, indexes); err != nil {
			return err
		}
	}
	return nil
}

func validateCompiledAllRuntime(source ContentModel, model CompiledModel, work ContentModelWork) error {
	if len(model.All) != len(source.Particles) {
		return errors.New("compiled all content model term count does not match source model")
	}
	allBitLen := (len(model.All) + 63) / 64
	if allBitLen > int(^uint32(0)) || model.AllBitLen != uint32(allBitLen) {
		return errors.New("compiled all content model bit length does not match terms")
	}
	required := false
	for i, term := range model.All {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
		sourceParticle := source.Particles[i]
		if sourceParticle.Kind == ParticleModel {
			return errors.New("compiled all content model source has model particle")
		}
		if !SameCompiledParticle(term.Particle, sourceParticle) || term.Particle.Occurs != sourceParticle.Occurs {
			return errors.New("compiled all content model term does not match source particle")
		}
		if term.Required != (sourceParticle.Occurs.Min > 0) {
			return errors.New("compiled all content model required flag does not match source particle")
		}
		if term.Required {
			required = true
		}
	}
	if model.Empty != (source.Occurs.Min == 0 || !required) {
		return errors.New("compiled all content model empty flag does not match source model")
	}
	return nil
}

func validateCompiledDFARuntime(
	names *NameTable,
	rt CompiledModelRuntime,
	model CompiledModel,
	validateUPA bool,
	work ContentModelWork,
	overlap *ContentModelAnalysis,
	indexes *dfaRowIndexAnalysis,
) error {
	if !ValidUint32Index(model.Start, len(model.Rows)) {
		return errors.New("compiled content model start state is invalid")
	}
	if model.Empty != model.Rows[model.Start].Accept {
		return errors.New("compiled content model empty flag does not match start row")
	}
	for i, row := range model.Rows {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
		if uint64(i) > uint64(^uint32(0)) {
			return errors.New("compiled content model row index is invalid")
		}
		if err := validateCompiledDFARow(names, rt, model, row, uint32(i), validateUPA, work, overlap, indexes); err != nil {
			return err
		}
	}
	return nil
}

func validateCompiledDFARow(
	names *NameTable,
	rt CompiledModelRuntime,
	model CompiledModel,
	row CompiledModelRow,
	index uint32,
	validateUPA bool,
	work ContentModelWork,
	overlap *ContentModelAnalysis,
	indexes *dfaRowIndexAnalysis,
) error {
	if row.Counted && !row.Unbounded && row.Max < row.Min {
		return errors.New("compiled content model counted state has invalid range")
	}
	if row.Counted {
		if err := validateCompiledParticle(rt, row.CountParticle); err != nil {
			return err
		}
	}
	countedLoops := 0
	for _, edge := range row.Edges {
		if err := spendContentModelWork(work); err != nil {
			return err
		}
		if !ValidUint32Index(edge.To, len(model.Rows)) {
			return errors.New("compiled content model edge target is invalid")
		}
		if err := validateCompiledParticle(rt, edge.Particle); err != nil {
			return err
		}
		if row.Counted && edge.To == index {
			if !SameCompiledParticle(edge.Particle, row.CountParticle) {
				return errors.New("compiled content model counted state has non-counted self loop")
			}
			countedLoops++
		}
	}
	if row.Counted && countedLoops != 1 {
		return errors.New("compiled content model counted state must have one counted self loop")
	}
	if validateUPA {
		if err := validateCompiledDFARowUPA(row, index, work, overlap); err != nil {
			return err
		}
	}
	if row.index.enabled() {
		return indexes.validateRow(names, row)
	}
	return nil
}

func validateCompiledDFARowUPA(
	row CompiledModelRow,
	index uint32,
	work ContentModelWork,
	overlap *ContentModelAnalysis,
) error {
	for i, a := range row.Edges {
		for j := i + 1; j < len(row.Edges); j++ {
			if err := spendContentModelWork(work); err != nil {
				return err
			}
			next := row.Edges[j]
			_, overlaps, err := overlap.Overlap(a.Particle, next.Particle)
			if err != nil {
				return err
			}
			if !overlaps {
				continue
			}
			if CompiledCountingException(index, row, a, next) {
				continue
			}
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
	default:
		return errors.New("compiled particle has invalid kind")
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
	for name, pos := range idx.nameToEdge {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		if names == nil || !names.ValidQName(name) || !ValidUint32Index(pos, len(row.Edges)) {
			return errors.New("compiled content model name index entry is invalid")
		}
		p := row.Edges[pos].Particle
		if p.Kind != ParticleElement {
			return errors.New("compiled content model name index targets non-element edge")
		}
		elementName, ok := a.rt.ElementName(p.Element)
		if !ok {
			return errors.New("compiled content model name index key does not match edge element")
		}
		if elementName != name {
			if _, ok := a.rt.SubstitutionMemberByName(p.Element, name); !ok {
				return errors.New("compiled content model name index key does not match edge element")
			}
		}
	}
	wi := 0
	for pos, edge := range row.Edges {
		if err := spendContentModelWork(a.work); err != nil {
			return err
		}
		edgePos := uint32(pos)
		switch edge.Particle.Kind {
		case ParticleElement:
			name, ok := a.rt.ElementName(edge.Particle.Element)
			if !ok {
				return errors.New("compiled content model indexed row has invalid element edge")
			}
			if err := requireIndexedName(idx, name, edgePos); err != nil {
				return err
			}
			entries, err := a.substitutionEntries(edge.Particle.Element)
			if err != nil {
				return err
			}
			indexed := true
			for _, entry := range entries {
				if err := spendContentModelWork(a.work); err != nil {
					return err
				}
				indexed = requireIndexedName(idx, entry, edgePos) == nil
				if !indexed {
					break
				}
			}
			if !indexed {
				return errors.New("compiled content model name index is missing element edge")
			}
		case ParticleWildcard:
			if wi >= len(idx.wildcardEdges) || idx.wildcardEdges[wi] != edgePos {
				return errors.New("compiled content model wildcard list does not match wildcard edges")
			}
			wi++
		default:
			return errors.New("compiled content model indexed row has model edge")
		}
	}
	if wi != len(idx.wildcardEdges) {
		return errors.New("compiled content model wildcard list does not match wildcard edges")
	}
	return nil
}

func requireIndexedName(idx dfaRowIndex, name QName, pos uint32) error {
	if got, ok := idx.nameToEdge[name]; !ok || got != pos {
		return errors.New("compiled content model name index is missing element edge")
	}
	return nil
}
