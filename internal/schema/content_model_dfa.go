package schema

import (
	"cmp"
	"encoding/binary"
	"slices"
	"strconv"

	"github.com/jacoelho/xsd/xsderrors"
)

// dfaConfig is one configuration of a deterministic state: a state of the
// counted source automaton and the interned values of its counters. Interned
// vectors make a configuration a fixed-size comparable value, so equal
// configurations are equal values.
type dfaConfig struct {
	State    uint32
	Counters dfaCounterVectorID
}

type dfaCounterVectorID uint32

// dfaZeroCounters is interned first by every determinization, so the zero
// configuration value is the start configuration of source state zero.
const dfaZeroCounters dfaCounterVectorID = 0

// dfaConfigKeyBytes is the encoded width of one configuration in a state key:
// the source state and the counter vector ID, little endian.
const dfaConfigKeyBytes = 8

// dfaCounterVectors interns the counter vectors of one determinization. A
// vector is stored once however many configurations hold it.
type dfaCounterVectors struct {
	ids    map[string]dfaCounterVectorID
	values []uint32
	key    []byte
	width  int
}

func newDFACounterVectors(width int) (dfaCounterVectors, error) {
	v := dfaCounterVectors{ids: make(map[string]dfaCounterVectorID), width: width}
	if _, err := v.intern(make([]uint32, width)); err != nil {
		return dfaCounterVectors{}, err
	}
	return v, nil
}

func (v *dfaCounterVectors) intern(counters []uint32) (dfaCounterVectorID, error) {
	v.key = v.key[:0]
	for _, counter := range counters {
		v.key = binary.LittleEndian.AppendUint32(v.key, counter)
	}
	if id, ok := v.ids[string(v.key)]; ok {
		return id, nil
	}
	n, err := checkedSchemaUint32(len(v.ids), "content model DFA counter vector limit exceeded")
	if err != nil {
		return 0, err
	}
	id := dfaCounterVectorID(n)
	v.ids[string(v.key)] = id
	v.values = append(v.values, counters...)
	return id, nil
}

// vector returns the interned counters of id. The result is capped at its
// width, and interning never rewrites stored values, so it stays valid while
// further vectors are interned.
func (v *dfaCounterVectors) vector(id dfaCounterVectorID) []uint32 {
	start := int(id) * v.width
	end := start + v.width
	return v.values[start:end:end]
}

// compareConfigs orders configurations by source state, then by counter
// values. The order is independent of interning order, so a state's
// configurations, and the particle that first reaches each transition term,
// do not depend on which vector was interned first.
func (v *dfaCounterVectors) compareConfigs(a, b dfaConfig) int {
	if n := cmp.Compare(a.State, b.State); n != 0 {
		return n
	}
	if a.Counters == b.Counters {
		return 0
	}
	return slices.Compare(v.vector(a.Counters), v.vector(b.Counters))
}

type dfaDeterministicRow struct {
	Edges  []compiledModelEdge
	Accept bool
}

type dfaTransitionSet struct {
	Configs  []dfaConfig
	Particle contentParticle
	Key      particleTermKey
}

type particleTermKey struct {
	Element  ElementID
	Wildcard WildcardID
	Kind     particleKind
}

func (b *dfaBuilder) compileDeterministicModel(start uint32) (compiledModel, error) {
	caps, capsErr := b.counterCaps()
	if capsErr != nil {
		return compiledModel{}, capsErr
	}
	compilation, err := newDeterministicModelCompilation(b, caps)
	if err != nil {
		return compiledModel{}, err
	}
	startID, err := compilation.start(start)
	if err != nil {
		return compiledModel{}, err
	}
	if err := compilation.compileRows(); err != nil {
		return compiledModel{}, err
	}
	rows := compilation.table()
	if err := b.c.checkCompiledRowsUPA(rows); err != nil {
		return compiledModel{}, err
	}
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  rows,
		Start: startID,
		Empty: rows[startID].Accept,
	}, nil
}

// deterministicModelCompilation determinizes one counted source automaton.
// A deterministic state is identified by its key, the encoding of its sorted
// and deduplicated configurations; keys[id] is the only stored form of state
// id's configurations. States are discovered in ID order and their rows are
// built in the same order, so keys[len(rows):] is the work queue. Row
// construction reuses the scratch fields across rows.
type deterministicModelCompilation struct {
	builder    *dfaBuilder
	states     map[string]uint32
	groupIndex map[particleTermKey]int
	caps       []uint32
	keys       []string
	rows       []dfaDeterministicRow
	configs    []dfaConfig
	groups     []dfaTransitionSet
	counters   []uint32
	key        []byte
	vectors    dfaCounterVectors
}

func newDeterministicModelCompilation(b *dfaBuilder, caps []uint32) (*deterministicModelCompilation, error) {
	vectors, err := newDFACounterVectors(int(b.counters))
	if err != nil {
		return nil, err
	}
	return &deterministicModelCompilation{
		builder:    b,
		states:     make(map[string]uint32),
		groupIndex: make(map[particleTermKey]int),
		caps:       caps,
		vectors:    vectors,
		counters:   make([]uint32, b.counters),
	}, nil
}

func (c *deterministicModelCompilation) start(start uint32) (uint32, error) {
	if err := c.builder.c.work.spend(int(c.builder.counters)); err != nil {
		return 0, err
	}
	return c.stateID([]dfaConfig{{State: start, Counters: dfaZeroCounters}})
}

func (c *deterministicModelCompilation) stateID(configs []dfaConfig) (uint32, error) {
	if err := c.builder.spendDFAConfigs(len(configs)); err != nil {
		return 0, err
	}
	return c.stateIDNormalized(c.normalize(configs))
}

func (c *deterministicModelCompilation) stateIDNormalized(configs []dfaConfig) (uint32, error) {
	if len(configs) > c.builder.limit {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded")
	}
	c.key = c.key[:0]
	for _, config := range configs {
		c.key = binary.LittleEndian.AppendUint32(c.key, config.State)
		c.key = binary.LittleEndian.AppendUint32(c.key, uint32(config.Counters))
	}
	if id, ok := c.states[string(c.key)]; ok {
		return id, nil
	}
	if len(c.states) >= c.builder.limit {
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded")
	}
	id, err := checkedSchemaUint32(len(c.states), "content model DFA state limit exceeded")
	if err != nil {
		return 0, err
	}
	key := string(c.key)
	c.states[key] = id
	c.keys = append(c.keys, key)
	return id, nil
}

func (c *deterministicModelCompilation) normalize(configs []dfaConfig) []dfaConfig {
	slices.SortFunc(configs, c.vectors.compareConfigs)
	return slices.Compact(configs)
}

func (c *deterministicModelCompilation) compileRows() error {
	for len(c.rows) < len(c.keys) {
		configs := c.decodeState(c.keys[len(c.rows)])
		if err := c.builder.c.work.spend(len(configs) + 1); err != nil {
			return err
		}
		row, err := c.deterministicRow(configs, c.stateIDNormalized)
		if err != nil {
			return err
		}
		c.appendRow(row)
	}
	return nil
}

func (c *deterministicModelCompilation) decodeState(key string) []dfaConfig {
	c.configs = c.configs[:0]
	for i := 0; i+dfaConfigKeyBytes <= len(key); i += dfaConfigKeyBytes {
		c.configs = append(c.configs, dfaConfig{
			State:    decodeUint32LE(key[i:]),
			Counters: dfaCounterVectorID(decodeUint32LE(key[i+4:])),
		})
	}
	return c.configs
}

func decodeUint32LE(s string) uint32 {
	return uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
}

// appendRow keeps the two fields a deterministic row sets and grows them by
// doubling: append's own growth for large slices allocates about five times
// the final length, and compiledModelRow is three times their size.
func (c *deterministicModelCompilation) appendRow(row compiledModelRow) {
	if len(c.rows) == cap(c.rows) {
		c.rows = slices.Grow(c.rows, max(len(c.rows), len(c.keys)-len(c.rows)))
	}
	c.rows = append(c.rows, dfaDeterministicRow{Edges: row.Edges, Accept: row.Accept})
}

// table returns the built rows at their exact length; compiled models live
// until publication.
func (c *deterministicModelCompilation) table() []compiledModelRow {
	rows := make([]compiledModelRow, len(c.rows))
	for i, row := range c.rows {
		rows[i] = compiledModelRow{Edges: row.Edges, Accept: row.Accept}
	}
	return rows
}

func (c *deterministicModelCompilation) deterministicRow(configs []dfaConfig, stateID func([]dfaConfig) (uint32, error)) (compiledModelRow, error) {
	var row compiledModelRow
	c.groups = c.groups[:0]
	clear(c.groupIndex)
	for _, config := range configs {
		if err := c.collectConfig(&row, config); err != nil {
			return compiledModelRow{}, err
		}
	}
	if err := c.builder.c.work.spend(len(c.groups)); err != nil {
		return compiledModelRow{}, err
	}
	slices.SortFunc(c.groups, func(a, b dfaTransitionSet) int {
		return compareParticleTermKey(a.Key, b.Key)
	})
	if len(c.groups) != 0 {
		row.Edges = make([]compiledModelEdge, 0, len(c.groups))
	}
	for i := range c.groups {
		if err := c.appendTransition(&row, &c.groups[i], stateID); err != nil {
			return compiledModelRow{}, err
		}
	}
	return row, nil
}

func (c *deterministicModelCompilation) collectConfig(row *compiledModelRow, config dfaConfig) error {
	b := c.builder
	if !validUint32Index(config.State, len(b.rows)) {
		return xsderrors.InternalInvariant("content model DFA state out of range")
	}
	source := b.rows[config.State]
	if err := b.c.work.spend(len(source.Accept) + len(source.Edges)); err != nil {
		return err
	}
	counters := c.vectors.vector(config.Counters)
	if err := b.collectDeterministicAccept(row, source.Accept, counters, c.caps); err != nil {
		return err
	}
	for _, edge := range source.Edges {
		if err := c.collectEdge(edge, counters); err != nil {
			return err
		}
	}
	return nil
}

func (b *dfaBuilder) collectDeterministicAccept(row *compiledModelRow, accepts []dfaAccept, counters, caps []uint32) error {
	for _, accept := range accepts {
		if err := b.c.work.spend(len(accept.Guards)); err != nil {
			return err
		}
		if dfaGuardsOK(counters, caps, accept.Guards) {
			row.Accept = true
			break
		}
	}
	return nil
}

func (c *deterministicModelCompilation) collectEdge(edge dfaSourceEdge, counters []uint32) error {
	work := c.builder.c.work
	if err := work.spend(len(edge.Guards)); err != nil {
		return err
	}
	if !dfaGuardsOK(counters, c.caps, edge.Guards) {
		return nil
	}
	if err := work.spend(len(counters) + len(edge.Actions) + 1); err != nil {
		return err
	}
	next, err := c.applyActions(counters, edge.Actions)
	if err != nil {
		return err
	}
	group := c.group(edge.Particle)
	group.Configs = append(group.Configs, dfaConfig{State: edge.To, Counters: next})
	return nil
}

func (c *deterministicModelCompilation) applyActions(counters []uint32, actions []compiledAction) (dfaCounterVectorID, error) {
	c.counters = append(c.counters[:0], counters...)
	if err := applyDFAActions(c.counters, c.caps, actions); err != nil {
		return 0, err
	}
	return c.vectors.intern(c.counters)
}

// group returns the transition set of p's term in the row being built. The
// first particle to reach a term is the particle of its edge.
func (c *deterministicModelCompilation) group(p contentParticle) *dfaTransitionSet {
	key := particleTermKeyOf(p)
	if i, ok := c.groupIndex[key]; ok {
		return &c.groups[i]
	}
	i := len(c.groups)
	c.groupIndex[key] = i
	if i < cap(c.groups) {
		c.groups = c.groups[:i+1]
		c.groups[i] = dfaTransitionSet{Configs: c.groups[i].Configs[:0], Particle: p, Key: key}
	} else {
		c.groups = append(c.groups, dfaTransitionSet{Particle: p, Key: key})
	}
	return &c.groups[i]
}

func (c *deterministicModelCompilation) appendTransition(row *compiledModelRow, group *dfaTransitionSet, stateID func([]dfaConfig) (uint32, error)) error {
	if err := c.builder.spendDFAConfigs(len(group.Configs)); err != nil {
		return err
	}
	group.Configs = c.normalize(group.Configs)
	if len(group.Configs) > c.builder.limit {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded")
	}
	to, err := stateID(group.Configs)
	if err != nil {
		return err
	}
	row.Edges = append(row.Edges, compiledModelEdge{Particle: group.Particle, To: to})
	return nil
}

func (b *dfaBuilder) counterCaps() ([]uint32, error) {
	if err := b.c.work.spend(int(b.counters) + len(b.rows) + 1); err != nil {
		return nil, err
	}
	caps := make([]uint32, b.counters)
	for _, row := range b.rows {
		if err := b.addRowCounterCaps(caps, row); err != nil {
			return nil, err
		}
	}
	return caps, nil
}

func (b *dfaBuilder) addRowCounterCaps(caps []uint32, row dfaSourceRow) error {
	for _, edge := range row.Edges {
		if err := b.c.work.spend(len(edge.Guards) + 1); err != nil {
			return err
		}
		addCounterCaps(caps, edge.Guards)
	}
	for _, accept := range row.Accept {
		if err := b.c.work.spend(len(accept.Guards) + 1); err != nil {
			return err
		}
		addCounterCaps(caps, accept.Guards)
	}
	return nil
}

func (b *dfaBuilder) spendDFAConfigs(count int) error {
	if err := b.c.work.spend(count + 1); err != nil {
		return err
	}
	return b.c.work.spendProduct(count, int(b.counters))
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
		if !dfaGuardOK(counters, caps, guard) {
			return false
		}
	}
	return true
}

func dfaGuardOK(counters, caps []uint32, guard compiledGuard) bool {
	if !validUint32Index(guard.Slot, len(counters)) || !validUint32Index(guard.Slot, len(caps)) {
		return false
	}
	count := counters[guard.Slot]
	switch guard.Kind {
	case compiledGuardExitMin:
		return guard.N == 0 || count >= guard.N-1
	case compiledGuardLoopMax:
		return guard.N > 0 && count < guard.N-1
	default:
		return false
	}
}

func applyDFAActions(counters, caps []uint32, actions []compiledAction) error {
	for _, action := range actions {
		if !validUint32Index(action.Slot, len(counters)) {
			return xsderrors.InternalInvariant("content model DFA action references invalid counter")
		}
		switch action.Kind {
		case compiledActionInc:
			if counters[action.Slot] < caps[action.Slot] {
				counters[action.Slot]++
			}
		case compiledActionReset:
			counters[action.Slot] = 0
		default:
			return xsderrors.InternalInvariant("content model DFA action kind out of range")
		}
	}
	return nil
}

func particleTermKeyOf(p contentParticle) particleTermKey {
	switch p.Kind {
	case particleElement:
		return particleTermKey{Kind: p.Kind, Element: p.Element}
	case particleWildcard:
		return particleTermKey{Kind: p.Kind, Wildcard: p.Wildcard}
	case particleModel:
		return particleTermKey{Kind: p.Kind, Element: p.Element, Wildcard: p.Wildcard}
	default:
	}
	return particleTermKey{Kind: p.Kind, Element: p.Element, Wildcard: p.Wildcard}
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
