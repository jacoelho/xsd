package xsd

import (
	"cmp"
	"slices"
	"strconv"
)

const dfaEndPos = -1

type compiledGuardKind uint8

const (
	compiledGuardExitMin compiledGuardKind = iota
	compiledGuardLoopMax
)

type compiledGuard struct {
	Slot uint32
	N    uint32
	Kind compiledGuardKind
}

type compiledActionKind uint8

const (
	compiledActionInc compiledActionKind = iota
	compiledActionReset
)

type compiledAction struct {
	Slot uint32
	Kind compiledActionKind
}

type dfaEntry struct {
	Guards  []compiledGuard
	Actions []compiledAction
	Pos     int
}

type dfaTail struct {
	Guards  []compiledGuard
	Actions []compiledAction
	Pos     int
}

type dfaNode struct {
	First    []dfaEntry
	Last     []dfaTail
	Counters []uint32
	Nullable bool
}

type dfaBuilder struct {
	c         *compiler
	follow    map[int][]dfaEntry
	states    map[string]uint32
	positions []particle
	rows      []dfaSourceRow
	queue     [][]dfaEntry
	limits    []uint32
	limit     int
	counters  uint32
}

type dfaSourceRow struct {
	Edges  []dfaSourceEdge
	Accept []dfaAccept
}

type dfaSourceEdge struct {
	Guards   []compiledGuard
	Actions  []compiledAction
	Pos      int
	Particle particle
	To       uint32
}

type dfaAccept struct {
	Guards  []compiledGuard
	Actions []compiledAction
}

func (c *compiler) compileContentModels() error {
	c.rt.CompiledModels = make([]compiledModel, len(c.rt.Models))
	for id := range c.rt.Models {
		m, err := c.compileContentModel(contentModelID(id))
		if err != nil {
			return err
		}
		c.rt.CompiledModels[id] = m
	}
	return nil
}

func (c *compiler) compileContentModel(id contentModelID) (compiledModel, error) {
	model := c.rt.Models[id]
	switch model.Kind {
	case modelEmpty:
		return compiledModel{Kind: compiledModelEmpty, Mixed: model.Mixed, Empty: true}, nil
	case modelAny:
		return compiledModel{Kind: compiledModelAny, Mixed: model.Mixed, Empty: true}, nil
	case modelAll:
		return c.compileAllModel(model)
	default:
		limits := c.choiceLimitByModel[id]
		if m, ok, err := c.compileDirectModel(model, limits); ok || err != nil {
			return m, err
		}
		b := &dfaBuilder{
			c:      c,
			follow: make(map[int][]dfaEntry),
			states: make(map[string]uint32),
			limits: limits,
			limit:  c.limits.maxContentModelStates,
		}
		return b.compile(id)
	}
}

func (c *compiler) compileDirectModel(model contentModel, limits []uint32) (compiledModel, bool, error) {
	if !model.occurs.isExactlyOne() {
		return compiledModel{}, false, nil
	}
	switch model.Kind {
	case modelSequence:
		return c.compileDirectSequenceModel(model, limits)
	case modelChoice:
		return c.compileDirectChoiceModel(model)
	default:
		return compiledModel{}, false, nil
	}
}

func (c *compiler) compileDirectSequenceModel(model contentModel, limits []uint32) (compiledModel, bool, error) {
	rows := []compiledModelRow{{}}
	active := []uint32{0}
	for i, p := range model.Particles {
		p = applyRepeatedChoiceLimit(p, i, limits)
		if p.Kind != particleElement && p.Kind != particleWildcard {
			return compiledModel{}, false, nil
		}
		if p.occurs.Max == 0 && !p.occurs.Unbounded {
			continue
		}
		edge := compiledModelEdge{Particle: singleParticle(p)}
		if p.occurs.isExactlyOne() {
			to := uint32(len(rows))
			rows = append(rows, compiledModelRow{})
			edge.To = to
			for _, state := range active {
				rows[state].Edges = append(rows[state].Edges, edge)
			}
			active = []uint32{to}
			continue
		}
		to := uint32(len(rows))
		rows = append(rows, compiledParticleRow(edge.Particle, p.occurs, false))
		edge.To = to
		for _, state := range active {
			rows[state].Edges = append(rows[state].Edges, edge)
		}
		if p.occurs.Unbounded || p.occurs.Max > 1 {
			rows[to].Edges = append(rows[to].Edges, edge)
		}
		next := []uint32{to}
		if p.occurs.Min == 0 {
			next = append(next, active...)
			slices.Sort(next)
			next = slices.Compact(next)
		}
		active = next
	}
	for _, state := range active {
		rows[state].Accept = true
	}
	if len(rows) > c.limits.maxContentModelStates {
		return compiledModel{}, false, schemaCompile(ErrSchemaLimit, "content model DFA state limit exceeded")
	}
	if err := c.checkCompiledRowsUPA(rows); err != nil {
		return compiledModel{}, false, err
	}
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  rows,
		Start: 0,
		Mixed: model.Mixed,
		Empty: rows[0].Accept,
	}, true, nil
}

func (c *compiler) compileDirectChoiceModel(model contentModel) (compiledModel, bool, error) {
	rows := []compiledModelRow{{}}
	for _, p := range model.Particles {
		if p.Kind != particleElement && p.Kind != particleWildcard {
			return compiledModel{}, false, nil
		}
		if p.occurs.Max == 0 && !p.occurs.Unbounded {
			rows[0].Accept = true
			continue
		}
		if p.occurs.Min == 0 {
			rows[0].Accept = true
		}
		edge := compiledModelEdge{Particle: singleParticle(p), To: uint32(len(rows))}
		if p.occurs.isExactlyOne() {
			rows = append(rows, compiledModelRow{Accept: true})
			rows[0].Edges = append(rows[0].Edges, edge)
			continue
		}
		rows = append(rows, compiledParticleRow(edge.Particle, p.occurs, true))
		rows[0].Edges = append(rows[0].Edges, edge)
		if p.occurs.Unbounded || p.occurs.Max > 1 {
			rows[edge.To].Edges = append(rows[edge.To].Edges, edge)
		}
	}
	if len(rows) > c.limits.maxContentModelStates {
		return compiledModel{}, false, schemaCompile(ErrSchemaLimit, "content model DFA state limit exceeded")
	}
	if err := c.checkCompiledRowsUPA(rows); err != nil {
		return compiledModel{}, false, err
	}
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  rows,
		Start: 0,
		Mixed: model.Mixed,
		Empty: rows[0].Accept,
	}, true, nil
}

func compiledParticleRow(p particle, occurs occurrence, accept bool) compiledModelRow {
	row := compiledModelRow{Accept: accept}
	if repeatNeedsCounter(occurs) {
		row.Counted = true
		row.CountParticle = p
		row.Min = occurs.Min
		row.Max = occurs.Max
		row.Unbounded = occurs.Unbounded
	}
	return row
}

func (c *compiler) checkCompiledRowsUPA(rows []compiledModelRow) error {
	for state, row := range rows {
		for i, a := range row.Edges {
			for j := i + 1; j < len(row.Edges); j++ {
				next := row.Edges[j]
				name, ok := c.particlesOverlap(a.Particle, next.Particle)
				if !ok {
					continue
				}
				if compiledCountingException(uint32(state), row, a, next) {
					continue
				}
				msg := "UPA violation: overlapping particles"
				if name.Local != 0 || name.Namespace != 0 {
					msg += " " + c.rt.Names.Format(name)
				}
				return schemaCompile(ErrSchemaContentModel, msg)
			}
		}
	}
	return nil
}

func compiledCountingException(state uint32, row compiledModelRow, a, b compiledModelEdge) bool {
	if !row.Counted || row.Unbounded || row.Min != row.Max {
		return false
	}
	aLoop := a.To == state && sameCompiledParticle(a.Particle, row.CountParticle)
	bLoop := b.To == state && sameCompiledParticle(b.Particle, row.CountParticle)
	return (aLoop && b.To != state) || (bLoop && a.To != state)
}

func singleParticle(p particle) particle {
	p.occurs = occurrence{Min: 1, Max: 1}
	return p
}

func applyRepeatedChoiceLimit(p particle, index int, limits []uint32) particle {
	if !slices.Contains(limits, uint32(index)) || p.occurs.Min > 1 {
		return p
	}
	if p.occurs.Unbounded || p.occurs.Max > 1 {
		p.occurs.Unbounded = false
		p.occurs.Max = 1
	}
	return p
}

func sameCompiledParticle(a, b particle) bool {
	return a.Kind == b.Kind && a.Element == b.Element && a.wildcard == b.wildcard
}

func (c *compiler) compileAllModel(model contentModel) (compiledModel, error) {
	if err := c.checkAllUPA(model); err != nil {
		return compiledModel{}, err
	}
	terms := make([]compiledAllTerm, 0, len(model.Particles))
	required := false
	for _, p := range model.Particles {
		if p.Kind == particleModel {
			return compiledModel{}, schemaCompile(ErrSchemaContentModel, "xs:all cannot contain model group particles")
		}
		if p.occurs.Min > 0 {
			required = true
		}
		terms = append(terms, compiledAllTerm{
			Particle: p,
			Required: p.occurs.Min > 0,
		})
	}
	return compiledModel{
		Kind:      compiledModelAll,
		All:       terms,
		AllBitLen: uint32((len(terms) + 63) / 64),
		Mixed:     model.Mixed,
		Empty:     model.occurs.Min == 0 || !required,
	}, nil
}

func (c *compiler) checkAllUPA(model contentModel) error {
	for i, a := range model.Particles {
		for j := i + 1; j < len(model.Particles); j++ {
			if name, ok := c.particlesOverlap(a, model.Particles[j]); ok {
				msg := "UPA violation: overlapping particles in all"
				if name.Local != 0 || name.Namespace != 0 {
					msg += " " + c.rt.Names.Format(name)
				}
				return schemaCompile(ErrSchemaContentModel, msg)
			}
		}
	}
	return nil
}

func (b *dfaBuilder) compile(id contentModelID) (compiledModel, error) {
	root, err := b.modelNode(id, true)
	if err != nil {
		return compiledModel{}, err
	}
	start := root.First
	if root.Nullable {
		start = append(start, dfaEntry{Pos: dfaEndPos})
	}
	for _, tail := range root.Last {
		b.addFollow(tail.Pos, dfaEntry{
			Pos:     dfaEndPos,
			Guards:  slices.Clone(tail.Guards),
			Actions: slices.Clone(tail.Actions),
		})
	}
	startID, err := b.stateID(start)
	if err != nil {
		return compiledModel{}, err
	}
	for len(b.queue) != 0 {
		entries := b.queue[0]
		b.queue = b.queue[1:]
		row, err := b.row(entries)
		if err != nil {
			return compiledModel{}, err
		}
		b.rows = append(b.rows, row)
	}
	if err := b.checkUPA(); err != nil {
		return compiledModel{}, err
	}
	return b.compileDeterministicModel(id, startID)
}

func (b *dfaBuilder) row(entries []dfaEntry) (dfaSourceRow, error) {
	var row dfaSourceRow
	for _, e := range entries {
		if e.Pos == dfaEndPos {
			row.Accept = append(row.Accept, dfaAccept{
				Guards:  slices.Clone(e.Guards),
				Actions: slices.Clone(e.Actions),
			})
			continue
		}
		if e.Pos < 0 || e.Pos >= len(b.positions) {
			return dfaSourceRow{}, internalInvariant("content model DFA references invalid position")
		}
		to, err := b.stateID(b.follow[e.Pos])
		if err != nil {
			return dfaSourceRow{}, err
		}
		row.Edges = append(row.Edges, dfaSourceEdge{
			Particle: b.positions[e.Pos],
			Guards:   slices.Clone(e.Guards),
			Actions:  slices.Clone(e.Actions),
			To:       to,
			Pos:      e.Pos,
		})
	}
	return row, nil
}

func (b *dfaBuilder) stateID(entries []dfaEntry) (uint32, error) {
	entries = normalizeDFAEntries(entries)
	key := dfaStateKey(entries)
	if id, ok := b.states[key]; ok {
		return id, nil
	}
	if len(b.states) >= b.limit {
		return 0, schemaCompile(ErrSchemaLimit, "content model DFA state limit exceeded")
	}
	id := uint32(len(b.states))
	b.states[key] = id
	b.queue = append(b.queue, entries)
	return id, nil
}

func (b *dfaBuilder) modelNode(id contentModelID, top bool) (dfaNode, error) {
	model := b.c.rt.Models[id]
	var node dfaNode
	switch model.Kind {
	case modelEmpty:
		node.Nullable = true
	case modelSequence:
		node = dfaNode{Nullable: true}
		for i, p := range model.Particles {
			child, err := b.particleNode(p, i, top)
			if err != nil {
				return dfaNode{}, err
			}
			node = b.concat(node, child)
		}
	case modelChoice:
		for i, p := range model.Particles {
			child, err := b.particleNode(p, i, top)
			if err != nil {
				return dfaNode{}, err
			}
			node = b.choice(node, child)
		}
	case modelAll:
		return dfaNode{}, schemaCompile(ErrSchemaContentModel, "xs:all cannot be nested in DFA content models")
	default:
		return dfaNode{}, schemaCompile(ErrSchemaContentModel, "unsupported content model")
	}
	return b.repeat(node, model.occurs, -1)
}

func (b *dfaBuilder) particleNode(p particle, index int, top bool) (dfaNode, error) {
	var node dfaNode
	if top {
		p = applyRepeatedChoiceLimit(p, index, b.limits)
	}
	switch p.Kind {
	case particleElement, particleWildcard:
		node = b.leaf(p)
	case particleModel:
		child, err := b.modelNode(p.Model, false)
		if err != nil {
			return dfaNode{}, err
		}
		node = child
	default:
		return dfaNode{}, schemaCompile(ErrSchemaContentModel, "unsupported particle")
	}
	slot := -1
	return b.repeat(node, p.occurs, slot)
}

func (b *dfaBuilder) leaf(p particle) dfaNode {
	pos := len(b.positions)
	b.positions = append(b.positions, p)
	return dfaNode{
		First: []dfaEntry{{Pos: pos}},
		Last:  []dfaTail{{Pos: pos}},
	}
}

func (b *dfaBuilder) concat(a, c dfaNode) dfaNode {
	for _, tail := range a.Last {
		for _, first := range c.First {
			b.addFollow(tail.Pos, composeEntry(tail.Guards, tail.Actions, first))
		}
	}
	first := slices.Clone(a.First)
	if a.Nullable {
		first = append(first, c.First...)
	}
	last := slices.Clone(c.Last)
	if c.Nullable {
		last = append(last, a.Last...)
	}
	return dfaNode{
		First:    normalizeDFAEntries(first),
		Last:     normalizeDFATails(last),
		Counters: mergeCounters(a.Counters, c.Counters),
		Nullable: a.Nullable && c.Nullable,
	}
}

func (b *dfaBuilder) choice(a, c dfaNode) dfaNode {
	return dfaNode{
		First:    normalizeDFAEntries(append(slices.Clone(a.First), c.First...)),
		Last:     normalizeDFATails(append(slices.Clone(a.Last), c.Last...)),
		Counters: mergeCounters(a.Counters, c.Counters),
		Nullable: a.Nullable || c.Nullable,
	}
}

func (b *dfaBuilder) repeat(child dfaNode, occurs occurrence, slot int) (dfaNode, error) {
	if occurs.Max == 0 && !occurs.Unbounded {
		return dfaNode{Nullable: true}, nil
	}
	if occurs.isExactlyOne() {
		if slot < 0 {
			return child, nil
		}
		return countNode(child, uint32(slot)), nil
	}
	if slot < 0 && repeatNeedsCounter(occurs) {
		slot = int(b.newCounter())
	}
	self := uint32(slot)
	loop := occurs.Unbounded || occurs.Max > 1
	var exitGuards []compiledGuard
	var exitActions []compiledAction
	if slot >= 0 {
		if occurs.Min > 0 && !child.Nullable {
			exitGuards = append(exitGuards, compiledGuard{Slot: self, N: occurs.Min, Kind: compiledGuardExitMin})
		}
		exitActions = append(exitActions, compiledAction{Slot: self, Kind: compiledActionInc})
	}
	var last []dfaTail
	for _, tail := range child.Last {
		last = append(last, dfaTail{
			Pos:     tail.Pos,
			Guards:  appendGuards(tail.Guards, exitGuards),
			Actions: appendActions(tail.Actions, exitActions),
		})
	}
	if loop {
		for _, tail := range child.Last {
			for _, first := range child.First {
				guards := slices.Clone(tail.Guards)
				if slot >= 0 && !occurs.Unbounded {
					guards = append(guards, compiledGuard{Slot: self, N: occurs.Max, Kind: compiledGuardLoopMax})
				}
				actions := slices.Clone(tail.Actions)
				if slot >= 0 {
					actions = append(actions, compiledAction{Slot: self, Kind: compiledActionInc})
				}
				actions = append(actions, resetActions(child.Counters, self)...)
				b.addFollow(tail.Pos, composeEntry(guards, actions, first))
			}
		}
	}
	counters := slices.Clone(child.Counters)
	if slot >= 0 && !slices.Contains(counters, self) {
		counters = append(counters, self)
		slices.Sort(counters)
	}
	return dfaNode{
		First:    child.First,
		Last:     normalizeDFATails(last),
		Counters: counters,
		Nullable: occurs.Min == 0 || child.Nullable,
	}, nil
}

func repeatNeedsCounter(occurs occurrence) bool {
	if occurs.Min > 1 {
		return true
	}
	return !occurs.Unbounded && occurs.Max > 1
}

func countNode(child dfaNode, slot uint32) dfaNode {
	action := []compiledAction{{Slot: slot, Kind: compiledActionInc}}
	last := make([]dfaTail, 0, len(child.Last))
	for _, tail := range child.Last {
		last = append(last, dfaTail{
			Pos:     tail.Pos,
			Guards:  slices.Clone(tail.Guards),
			Actions: appendActions(tail.Actions, action),
		})
	}
	counters := slices.Clone(child.Counters)
	if !slices.Contains(counters, slot) {
		counters = append(counters, slot)
		slices.Sort(counters)
	}
	return dfaNode{
		First:    child.First,
		Last:     normalizeDFATails(last),
		Counters: counters,
		Nullable: child.Nullable,
	}
}

func (b *dfaBuilder) newCounter() uint32 {
	slot := b.counters
	b.counters++
	return slot
}

func (b *dfaBuilder) addFollow(pos int, entry dfaEntry) {
	b.follow[pos] = append(b.follow[pos], entry)
	b.follow[pos] = normalizeDFAEntries(b.follow[pos])
}

func (b *dfaBuilder) checkUPA() error {
	for _, row := range b.rows {
		for i, a := range row.Edges {
			for j := i + 1; j < len(row.Edges); j++ {
				next := row.Edges[j]
				if a.Pos == next.Pos {
					continue
				}
				name, ok := b.c.particlesOverlap(a.Particle, next.Particle)
				if !ok {
					continue
				}
				if countingException(a, next) {
					continue
				}
				msg := "UPA violation: overlapping particles"
				if name.Local != 0 || name.Namespace != 0 {
					msg += " " + b.c.rt.Names.Format(name)
				}
				return schemaCompile(ErrSchemaContentModel, msg)
			}
		}
	}
	return nil
}

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
		id := uint32(len(states))
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
	if err := b.checkDeterministicUPA(rows); err != nil {
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
			if len(group.Configs) >= b.limit {
				return compiledModelRow{}, schemaCompile(ErrSchemaLimit, "content model DFA state limit exceeded")
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

func (b *dfaBuilder) checkDeterministicUPA(rows []compiledModelRow) error {
	return b.c.checkCompiledRowsUPA(rows)
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

func normalizeDFATails(tails []dfaTail) []dfaTail {
	tails = slices.Clone(tails)
	slices.SortFunc(tails, compareDFATail)
	return slices.CompactFunc(tails, func(a, b dfaTail) bool {
		return compareDFATail(a, b) == 0
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

func compareDFATail(a, b dfaTail) int {
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
