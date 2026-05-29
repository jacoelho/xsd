package xsd

import (
	"cmp"
	"slices"
	"strconv"
)

type dfaConfig struct {
	Counters []uint32
	State    uint32
}

type dfaDeterministicState struct {
	Configs []dfaConfig
}

type dfaTransitionSet struct {
	Configs  []dfaConfig
	Particle particle
}

type particleTermKey struct {
	Element  elementID
	Wildcard wildcardID
	Kind     particleKind
}

func (b *dfaBuilder) compileDeterministicModel(id contentModelID, start uint32) (compiledModel, error) {
	caps := b.counterCaps()
	states := make(map[string]uint32)
	var queue []dfaDeterministicState
	var rows []compiledModelRow
	stateID := func(state dfaDeterministicState) (uint32, error) {
		state.Configs = normalizeDFAConfigs(state.Configs)
		if len(state.Configs) > b.limit {
			return 0, schemaCompile(ErrSchemaLimit, "content model DFA state limit exceeded")
		}
		key := dfaConfigStateKey(state.Configs)
		if id, ok := states[key]; ok {
			return id, nil
		}
		if len(states) >= b.limit {
			return 0, schemaCompile(ErrSchemaLimit, "content model DFA state limit exceeded")
		}
		id, err := checkedUint32(len(states), "content model DFA state limit exceeded")
		if err != nil {
			return 0, err
		}
		states[key] = id
		queue = append(queue, state)
		return id, nil
	}
	startCounters := make([]uint32, b.counters)
	startID, err := stateID(dfaDeterministicState{Configs: []dfaConfig{{State: start, Counters: startCounters}}})
	if err != nil {
		return compiledModel{}, err
	}
	for len(queue) != 0 {
		state := queue[0]
		queue = queue[1:]
		row, err := b.deterministicRow(state, caps, stateID)
		if err != nil {
			return compiledModel{}, err
		}
		rows = append(rows, row)
	}
	if err := b.c.checkCompiledRowsUPA(rows); err != nil {
		return compiledModel{}, err
	}
	model := b.c.rt.Models[id]
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  rows,
		Start: startID,
		Mixed: model.Mixed,
		Empty: rows[startID].Accept,
	}, nil
}

func (b *dfaBuilder) deterministicRow(state dfaDeterministicState, caps []uint32, stateID func(dfaDeterministicState) (uint32, error)) (compiledModelRow, error) {
	var row compiledModelRow
	groups := make(map[particleTermKey]*dfaTransitionSet)
	for _, config := range state.Configs {
		if !validUint32Index(config.State, len(b.rows)) {
			return compiledModelRow{}, internalInvariant("content model DFA state out of range")
		}
		source := b.rows[config.State]
		for _, accept := range source.Accept {
			if dfaGuardsOK(config.Counters, caps, accept.Guards) {
				row.Accept = true
				break
			}
		}
		for _, edge := range source.Edges {
			if !dfaGuardsOK(config.Counters, caps, edge.Guards) {
				continue
			}
			counters, err := applyDFAActions(config.Counters, caps, edge.Actions)
			if err != nil {
				return compiledModelRow{}, err
			}
			key := particleTermKeyOf(edge.Particle)
			group := groups[key]
			if group == nil {
				group = &dfaTransitionSet{Particle: edge.Particle}
				groups[key] = group
			}
			group.Configs = append(group.Configs, dfaConfig{State: edge.To, Counters: counters})
		}
	}
	keys := make([]particleTermKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, compareParticleTermKey)
	for _, key := range keys {
		group := groups[key]
		group.Configs = normalizeDFAConfigs(group.Configs)
		if len(group.Configs) > b.limit {
			return compiledModelRow{}, schemaCompile(ErrSchemaLimit, "content model DFA state limit exceeded")
		}
		to, err := stateID(dfaDeterministicState{Configs: group.Configs})
		if err != nil {
			return compiledModelRow{}, err
		}
		row.Edges = append(row.Edges, compiledModelEdge{Particle: group.Particle, To: to})
	}
	return row, nil
}

func (b *dfaBuilder) counterCaps() []uint32 {
	caps := make([]uint32, b.counters)
	for _, row := range b.rows {
		for _, edge := range row.Edges {
			addCounterCaps(caps, edge.Guards)
		}
		for _, accept := range row.Accept {
			addCounterCaps(caps, accept.Guards)
		}
	}
	return caps
}

func addCounterCaps(caps []uint32, guards []compiledGuard) {
	for _, guard := range guards {
		if !validUint32Index(guard.Slot, len(caps)) || guard.N == 0 {
			continue
		}
		caps[guard.Slot] = max(caps[guard.Slot], guard.N-1)
	}
}

func dfaGuardsOK(counters, caps []uint32, guards []compiledGuard) bool {
	for _, guard := range guards {
		if !validUint32Index(guard.Slot, len(counters)) || !validUint32Index(guard.Slot, len(caps)) {
			return false
		}
		count := counters[guard.Slot]
		switch guard.Kind {
		case compiledGuardExitMin:
			if guard.N > 0 && count < guard.N-1 {
				return false
			}
		case compiledGuardLoopMax:
			if guard.N == 0 {
				return false
			}
			if count >= guard.N-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func applyDFAActions(counters, caps []uint32, actions []compiledAction) ([]uint32, error) {
	out := slices.Clone(counters)
	for _, action := range actions {
		if !validUint32Index(action.Slot, len(out)) {
			return nil, internalInvariant("content model DFA action references invalid counter")
		}
		switch action.Kind {
		case compiledActionInc:
			if out[action.Slot] < caps[action.Slot] {
				out[action.Slot]++
			}
		case compiledActionReset:
			out[action.Slot] = 0
		default:
			return nil, internalInvariant("content model DFA action kind out of range")
		}
	}
	return out, nil
}

// normalizeDFAConfigs canonicalizes DFA state keys before map lookup.
func normalizeDFAConfigs(configs []dfaConfig) []dfaConfig {
	configs = slices.Clone(configs)
	slices.SortFunc(configs, compareDFAConfig)
	return slices.CompactFunc(configs, func(a, b dfaConfig) bool {
		return compareDFAConfig(a, b) == 0
	})
}

func compareDFAConfig(a, b dfaConfig) int {
	if n := cmp.Compare(a.State, b.State); n != 0 {
		return n
	}
	if n := cmp.Compare(len(a.Counters), len(b.Counters)); n != 0 {
		return n
	}
	for i := range a.Counters {
		if n := cmp.Compare(a.Counters[i], b.Counters[i]); n != 0 {
			return n
		}
	}
	return 0
}

func dfaConfigStateKey(configs []dfaConfig) string {
	var b []byte
	for _, config := range configs {
		b = append(b, 's')
		b = strconv.AppendUint(b, uint64(config.State), 10)
		b = append(b, ':')
		for _, counter := range config.Counters {
			b = strconv.AppendUint(b, uint64(counter), 10)
			b = append(b, ',')
		}
		b = append(b, ';')
	}
	return string(b)
}

func particleTermKeyOf(p particle) particleTermKey {
	switch p.Kind {
	case particleElement:
		return particleTermKey{Kind: p.Kind, Element: p.Element}
	case particleWildcard:
		return particleTermKey{Kind: p.Kind, Wildcard: p.wildcard}
	default:
		return particleTermKey{Kind: p.Kind, Element: p.Element, Wildcard: p.wildcard}
	}
}

func compareParticleTermKey(a, b particleTermKey) int {
	if n := cmp.Compare(a.Kind, b.Kind); n != 0 {
		return n
	}
	if n := cmp.Compare(a.Element, b.Element); n != 0 {
		return n
	}
	return cmp.Compare(a.Wildcard, b.Wildcard)
}

func countingException(a, b dfaSourceEdge) bool {
	return complementaryCountingGuards(a.Guards, b.Guards) || complementaryCountingGuards(b.Guards, a.Guards)
}

func complementaryCountingGuards(loopEdge, exitEdge []compiledGuard) bool {
	for _, loop := range loopEdge {
		if loop.Kind != compiledGuardLoopMax {
			continue
		}
		for _, exit := range exitEdge {
			if exit.Kind == compiledGuardExitMin && exit.Slot == loop.Slot && exit.N == loop.N {
				return true
			}
		}
	}
	return false
}

func composeEntry(guards []compiledGuard, actions []compiledAction, entry dfaEntry) dfaEntry {
	return dfaEntry{
		Pos:     entry.Pos,
		Guards:  appendGuards(guards, entry.Guards),
		Actions: appendActions(actions, entry.Actions),
	}
}

func appendGuards(a, b []compiledGuard) []compiledGuard {
	out := slices.Clone(a)
	out = append(out, b...)
	slices.SortFunc(out, compareCompiledGuard)
	return slices.Compact(out)
}

func appendActions(a, b []compiledAction) []compiledAction {
	out := slices.Clone(a)
	out = append(out, b...)
	return out
}

func resetActions(counters []uint32, except uint32) []compiledAction {
	var out []compiledAction
	for _, slot := range counters {
		if slot == except {
			continue
		}
		out = append(out, compiledAction{Slot: slot, Kind: compiledActionReset})
	}
	return out
}

func mergeCounters(a, b []uint32) []uint32 {
	out := slices.Clone(a)
	out = append(out, b...)
	slices.Sort(out)
	return slices.Compact(out)
}

func normalizeDFAEntries(entries []dfaEntry) []dfaEntry {
	entries = slices.Clone(entries)
	slices.SortFunc(entries, compareDFAEntry)
	return slices.CompactFunc(entries, func(a, b dfaEntry) bool {
		return compareDFAEntry(a, b) == 0
	})
}

func compareDFAEntry(a, b dfaEntry) int {
	if n := cmp.Compare(a.Pos, b.Pos); n != 0 {
		return n
	}
	if n := compareCompiledGuards(a.Guards, b.Guards); n != 0 {
		return n
	}
	return compareCompiledActions(a.Actions, b.Actions)
}

func compareCompiledGuards(a, b []compiledGuard) int {
	if n := cmp.Compare(len(a), len(b)); n != 0 {
		return n
	}
	for i := range a {
		if n := compareCompiledGuard(a[i], b[i]); n != 0 {
			return n
		}
	}
	return 0
}

func compareCompiledGuard(a, b compiledGuard) int {
	if n := cmp.Compare(a.Slot, b.Slot); n != 0 {
		return n
	}
	if n := cmp.Compare(a.Kind, b.Kind); n != 0 {
		return n
	}
	return cmp.Compare(a.N, b.N)
}

func compareCompiledActions(a, b []compiledAction) int {
	if n := cmp.Compare(len(a), len(b)); n != 0 {
		return n
	}
	for i := range a {
		if n := cmp.Compare(a[i].Slot, b[i].Slot); n != 0 {
			return n
		}
		if n := cmp.Compare(a[i].Kind, b[i].Kind); n != 0 {
			return n
		}
	}
	return 0
}

func dfaStateKey(entries []dfaEntry) string {
	var b []byte
	for _, e := range entries {
		b = strconv.AppendInt(b, int64(e.Pos), 10)
		b = append(b, ':')
		for _, g := range e.Guards {
			b = append(b, 'g')
			b = strconv.AppendUint(b, uint64(g.Slot), 10)
			b = append(b, '/')
			b = strconv.AppendUint(b, uint64(g.Kind), 10)
			b = append(b, '/')
			b = strconv.AppendUint(b, uint64(g.N), 10)
			b = append(b, ',')
		}
		b = append(b, ':')
		for _, a := range e.Actions {
			b = append(b, 'a')
			b = strconv.AppendUint(b, uint64(a.Slot), 10)
			b = append(b, '/')
			b = strconv.AppendUint(b, uint64(a.Kind), 10)
			b = append(b, ',')
		}
		b = append(b, ';')
	}
	return string(b)
}
