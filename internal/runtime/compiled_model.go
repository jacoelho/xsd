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

// DFARowIndex stores the optional name index for a compiled DFA row.
type DFARowIndex struct {
	NameToEdge    map[QName]uint32
	WildcardEdges []uint32
	Enabled       bool
}

// IsEnabled reports whether the row index should be used.
func (idx DFARowIndex) IsEnabled() bool {
	return idx.Enabled
}

// CompiledModelRow stores a compiled DFA state.
type CompiledModelRow struct {
	Edges         []CompiledModelEdge
	Index         DFARowIndex
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

func equalDFARowIndex(a, b DFARowIndex) bool {
	return a.Enabled == b.Enabled &&
		maps.Equal(a.NameToEdge, b.NameToEdge) &&
		slices.Equal(a.WildcardEdges, b.WildcardEdges)
}

// CompiledDFARowIndexMinEdges is the edge count at which a compiled DFA row
// gets a name index instead of using the linear edge scan during validation.
const CompiledDFARowIndexMinEdges = 8

// IndexCompiledModelRows builds optional name indexes for wide compiled DFA
// rows. Rows where one name maps to multiple edges keep the linear scan.
func IndexCompiledModelRows(rt DFARowIndexRuntime, model *CompiledModel) error {
	if model.Kind != CompiledModelDFA {
		return nil
	}
	for i := range model.Rows {
		if err := indexCompiledModelRow(rt, &model.Rows[i]); err != nil {
			return err
		}
	}
	return nil
}

func indexCompiledModelRow(rt DFARowIndexRuntime, row *CompiledModelRow) error {
	if len(row.Edges) < CompiledDFARowIndexMinEdges {
		return nil
	}
	index := make(map[QName]uint32, len(row.Edges))
	var wildcards []uint32
	for pos, edge := range row.Edges {
		edgePos := uint32(pos)
		switch edge.Particle.Kind {
		case ParticleElement:
			name, ok := rt.ElementName(edge.Particle.Element)
			if !ok {
				return errors.New("compiled content model index references invalid element")
			}
			if !indexEdgeName(index, name, edgePos) {
				return nil
			}
			for name := range rt.SubstitutionMembersByName(edge.Particle.Element) {
				if !indexEdgeName(index, name, edgePos) {
					return nil
				}
			}
		case ParticleWildcard:
			wildcards = append(wildcards, edgePos)
		case ParticleModel:
			return nil
		default:
			return errors.New("compiled content model index has invalid edge particle")
		}
	}
	row.Index = DFARowIndex{NameToEdge: index, WildcardEdges: wildcards, Enabled: true}
	return nil
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
func ValidateCompiledModelsRuntime(names *NameTable, rt CompiledModelRuntime, sources []ContentModel, models []CompiledModel) error {
	return validateCompiledModelsRuntime(names, rt, sources, models, true)
}

func validateCompiledModelsRuntime(names *NameTable, rt CompiledModelRuntime, sources []ContentModel, models []CompiledModel, validateUPA bool) error {
	if len(models) != len(sources) {
		return errors.New("compiled content model count does not match model count")
	}
	for i, model := range models {
		if err := validateCompiledModelRuntime(names, rt, ContentModelID(i), sources[i], model, validateUPA); err != nil {
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
) error {
	return validateCompiledModelRuntime(names, rt, id, source, model, true)
}

func validateCompiledModelRuntime(
	names *NameTable,
	rt CompiledModelRuntime,
	id ContentModelID,
	source ContentModel,
	model CompiledModel,
	validateUPA bool,
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
		if err := validateCompiledAllRuntime(source, model); err != nil {
			return err
		}
		for _, term := range model.All {
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
		if err := validateCompiledDFARuntime(names, rt, model, validateUPA); err != nil {
			return err
		}
	}
	return nil
}

func validateCompiledAllRuntime(source ContentModel, model CompiledModel) error {
	if len(model.All) != len(source.Particles) {
		return errors.New("compiled all content model term count does not match source model")
	}
	allBitLen := (len(model.All) + 63) / 64
	if allBitLen > int(^uint32(0)) || model.AllBitLen != uint32(allBitLen) {
		return errors.New("compiled all content model bit length does not match terms")
	}
	required := false
	for i, term := range model.All {
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

func validateCompiledDFARuntime(names *NameTable, rt CompiledModelRuntime, model CompiledModel, validateUPA bool) error {
	if !ValidUint32Index(model.Start, len(model.Rows)) {
		return errors.New("compiled content model start state is invalid")
	}
	if model.Empty != model.Rows[model.Start].Accept {
		return errors.New("compiled content model empty flag does not match start row")
	}
	for i, row := range model.Rows {
		if uint64(i) > uint64(^uint32(0)) {
			return errors.New("compiled content model row index is invalid")
		}
		if err := validateCompiledDFARow(names, rt, model, row, uint32(i), validateUPA); err != nil {
			return err
		}
	}
	return nil
}

func validateCompiledDFARow(names *NameTable, rt CompiledModelRuntime, model CompiledModel, row CompiledModelRow, index uint32, validateUPA bool) error {
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
		if err := validateCompiledDFARowUPA(rt, row, index); err != nil {
			return err
		}
	}
	if row.Index.IsEnabled() {
		return ValidateDFARowIndex(names, rt, row)
	}
	if row.Index.NameToEdge != nil || row.Index.WildcardEdges != nil {
		return errors.New("compiled content model inactive row index stores data")
	}
	return nil
}

func validateCompiledDFARowUPA(rt CompiledModelRuntime, row CompiledModelRow, index uint32) error {
	for i, a := range row.Edges {
		for j := i + 1; j < len(row.Edges); j++ {
			next := row.Edges[j]
			if _, ok := ParticlesOverlap(rt, a.Particle, next.Particle); !ok {
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
	SubstitutionMembersByName(id ElementID) map[QName]ElementID
}

// ValidateDFARowIndex checks that row.Index mirrors row.Edges exactly.
func ValidateDFARowIndex(names *NameTable, rt DFARowIndexRuntime, row CompiledModelRow) error {
	idx := row.Index
	if idx.NameToEdge == nil {
		return errors.New("compiled content model name index is nil")
	}
	for name, pos := range idx.NameToEdge {
		if names == nil || !names.ValidQName(name) || !ValidUint32Index(pos, len(row.Edges)) {
			return errors.New("compiled content model name index entry is invalid")
		}
		p := row.Edges[pos].Particle
		if p.Kind != ParticleElement {
			return errors.New("compiled content model name index targets non-element edge")
		}
		elementName, ok := rt.ElementName(p.Element)
		if !ok {
			return errors.New("compiled content model name index key does not match edge element")
		}
		if elementName != name {
			if _, ok := rt.SubstitutionMemberByName(p.Element, name); !ok {
				return errors.New("compiled content model name index key does not match edge element")
			}
		}
	}
	wi := 0
	for pos, edge := range row.Edges {
		edgePos := uint32(pos)
		switch edge.Particle.Kind {
		case ParticleElement:
			name, ok := rt.ElementName(edge.Particle.Element)
			if !ok {
				return errors.New("compiled content model indexed row has invalid element edge")
			}
			if err := requireIndexedName(idx, name, edgePos); err != nil {
				return err
			}
			for name := range rt.SubstitutionMembersByName(edge.Particle.Element) {
				if err := requireIndexedName(idx, name, edgePos); err != nil {
					return errors.New("compiled content model name index is missing element edge")
				}
			}
		case ParticleWildcard:
			if wi >= len(idx.WildcardEdges) || idx.WildcardEdges[wi] != edgePos {
				return errors.New("compiled content model wildcard list does not match wildcard edges")
			}
			wi++
		default:
			return errors.New("compiled content model indexed row has model edge")
		}
	}
	if wi != len(idx.WildcardEdges) {
		return errors.New("compiled content model wildcard list does not match wildcard edges")
	}
	return nil
}

func requireIndexedName(idx DFARowIndex, name QName, pos uint32) error {
	if got, ok := idx.NameToEdge[name]; !ok || got != pos {
		return errors.New("compiled content model name index is missing element edge")
	}
	return nil
}
