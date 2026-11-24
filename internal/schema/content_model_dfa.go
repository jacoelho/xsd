package schema

import (
	"cmp"
	"encoding/binary"
	"slices"

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
//
// A vector of width w is new only when an action changed it, so before intern
// stores it, collectConfig has charged a step for its source edge and
// collectEdge w + 1 and at least one per action: at least w + 3 steps. A new
// vector holds its values, a copy of them as its ids key and an ids entry.
// With Go 1.27.3 on darwin/arm64 these held at most 67 live bytes, spare
// slice and map capacity included, at width 1 over 1,024 to 1,048,576 vectors,
// and fewer per step at widths up to 64, so vectors take at most about 17
// live bytes per step; fewer than 1,024 vectors held under 600 KiB in all.
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

// dfaTransitionSet is one transition of the row being built: the
// configurations its source edges reach, and the particle and source position
// of the first edge that joined it.
type dfaTransitionSet struct {
	Configs  []dfaConfig
	Particle contentParticle
	Pos      int
	Key      dfaTransitionKey
}

// dfaTransitionKey identifies the transition a source edge joins: its kind
// and the ID of the element declaration or wildcard it references, so the
// edges of one term share a transition, or, for the position of a choice
// without branches, the position, so no two such positions share one.
type dfaTransitionKey struct {
	ID   uint32
	Kind particleKind
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
	if b.emptyChoices {
		rows, startID = withoutEmptyChoiceEdges(rows, startID)
	}
	return compiledModel{
		Kind:  compiledModelDFA,
		Rows:  rows,
		Start: startID,
		Empty: rows[startID].Accept,
	}, nil
}

// withoutEmptyChoiceEdges removes, in place, the edges of the positions of
// choices without branches, which no item takes, and the rows only those
// edges reach, renumbering the rest in order, and returns the start's new
// number. The removed rows and edges stay charged to the retained budget:
// determinization built them and the Unique Particle Attribution checks read
// them.
func withoutEmptyChoiceEdges(rows []compiledModelRow, start uint32) ([]compiledModelRow, uint32) {
	reached := rowsReachedByItems(rows, start)
	number := make([]uint32, len(rows))
	var next uint32
	for state := range rows {
		if reached[state] {
			number[state] = next
			next++
		}
	}
	kept := rows[:0]
	for state, row := range rows {
		if !reached[state] {
			continue
		}
		row.Edges = slices.DeleteFunc(row.Edges, func(edge compiledModelEdge) bool {
			return edge.Particle.Kind == particleModel
		})
		for i := range row.Edges {
			row.Edges[i].To = number[row.Edges[i].To]
		}
		kept = append(kept, row)
	}
	clear(rows[len(kept):])
	return kept, number[start]
}

// rowsReachedByItems reports which rows edges other than those of choices
// without branches reach from start.
func rowsReachedByItems(rows []compiledModelRow, start uint32) []bool {
	reached := make([]bool, len(rows))
	reached[start] = true
	queue := []uint32{start}
	for len(queue) != 0 {
		state := queue[0]
		queue = queue[1:]
		for _, edge := range rows[state].Edges {
			if edge.Particle.Kind != particleModel && !reached[edge.To] {
				reached[edge.To] = true
				queue = append(queue, edge.To)
			}
		}
	}
	return reached
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
	groupIndex map[dfaTransitionKey]int
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
		groupIndex: make(map[dfaTransitionKey]int),
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
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded", nil)
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
		return 0, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded", nil)
	}
	id, err := checkedSchemaUint32(len(c.states), "content model DFA state limit exceeded")
	if err != nil {
		return 0, err
	}
	key := string(c.key)
	c.states[key] = id
	c.keys = appendDoubling(c.keys, key)
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
		if err := c.builder.c.retained.spend(1); err != nil {
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

// appendRow keeps only the two fields a deterministic row sets, a third of
// compiledModelRow, until the table's length is known.
func (c *deterministicModelCompilation) appendRow(row compiledModelRow) {
	c.rows = appendDoubling(c.rows, dfaDeterministicRow{Edges: row.Edges, Accept: row.Accept})
}

// appendDoubling appends e and doubles the capacity when it is full.
// Automaton construction appends one element at a time to slices whose final
// length it cannot know in advance. Over a slice's life, append's growth for
// large slices allocates about five times the final length; doubling
// allocates about twice.
func appendDoubling[S ~[]E, E any](s S, e E) S {
	if len(s) == cap(s) {
		s = slices.Grow(s, max(len(s), 4))
	}
	return append(s, e)
}

func (c *deterministicModelCompilation) table() []compiledModelRow {
	rows := make([]compiledModelRow, len(c.rows))
	for i, row := range c.rows {
		rows[i] = compiledModelRow{Edges: row.Edges, Accept: row.Accept}
	}
	return rows
}

// deterministicRow builds the row of one state. Its edges are the dominant heap
// term of determinization in the shapes measured, and the budget bounds them:
// each edge is one transition group, and before the group's edge is allocated
// collectConfig has charged a step for the source edge that opened the group,
// collectEdge a step for passing it, and the group count a step for the group.
// An edge is 32 bytes on every supported platform, so live row edges request
// at most 32/3 bytes per step. appendTransition charges at least two more
// steps per edge, so the edges of finished rows request at most 32/5 bytes per
// step; the 32/3 allows for the row in progress when the budget runs out.
// Each row's edges are one exact-capacity allocation, which the Go allocator
// rounds up by at most 40960/32800, about 1.249: the worst case is 1025 edges,
// 32,800 bytes, which exceeds the largest size class and rounds up to whole
// 8 KiB pages. Smaller rows round up by less. Allocated row edges therefore
// take at most about 13.3 bytes per step.
//
// The key of each state this row discovers stays within the same figure. A
// key is 8 bytes per configuration, at most 10 allocated, and before
// stateIDNormalized allocates it, each of its configurations has cost a step
// in collectConfig, one in collectEdge and one in appendTransition's
// spendDFAConfigs, and its group one more each in deterministicRow and
// spendDFAConfigs. A group of k configurations therefore holds at most
// 40 + 10k bytes of edge and key for at least 3k + 2 steps, at most 10 bytes
// per step, so a finished row stays under the 13.3 of a row in progress. The
// states map entry and the keys slot of each state are fixed per state, so
// MaxContentModelStates bounds them.
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
	if err := c.builder.c.retained.spend(len(c.groups)); err != nil {
		return compiledModelRow{}, err
	}
	slices.SortFunc(c.groups, func(a, b dfaTransitionSet) int {
		return compareDFATransitionKey(a.Key, b.Key)
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
		return xsderrors.InternalInvariant("content model DFA state out of range", nil)
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
		ok, err := dfaGuardsOK(counters, caps, accept.Guards)
		if err != nil {
			return err
		}
		if ok {
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
	ok, err := dfaGuardsOK(counters, c.caps, edge.Guards)
	if err != nil || !ok {
		return err
	}
	if err = work.spend(len(counters) + len(edge.Actions) + 1); err != nil {
		return err
	}
	next, err := c.applyActions(counters, edge.Actions)
	if err != nil {
		return err
	}
	group, err := c.group(edge)
	if err != nil {
		return err
	}
	if group.Pos != edge.Pos {
		if err := c.checkSharedTermUPA(group.Particle, edge.Particle); err != nil {
			return err
		}
	}
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

// checkSharedTermUPA reports the Unique Particle Attribution violation of a,
// the particle of a transition, and b, the particle of an edge from another
// position that joins it: one configuration set reaches both positions, and
// they consume the same items. The source rows compare only the positions of
// one source state, and the determinized rows only transitions of different
// terms, so a pair from two source states that share a term is seen only
// here. Edges of one position under different counter values share its
// transition without competing.
func (c *deterministicModelCompilation) checkSharedTermUPA(a, b contentParticle) error {
	particles, overlap, err := c.builder.c.particlesOverlap(a, b)
	if err != nil || !overlap {
		return err
	}
	return c.builder.c.upaError(particles)
}

// group returns the transition that edge joins in the row being built. The
// first edge to reach a transition gives its particle and position.
func (c *deterministicModelCompilation) group(edge dfaSourceEdge) (*dfaTransitionSet, error) {
	key, err := dfaTransitionKeyOf(edge.Particle, edge.Pos)
	if err != nil {
		return nil, err
	}
	if i, ok := c.groupIndex[key]; ok {
		return &c.groups[i], nil
	}
	i := len(c.groups)
	c.groupIndex[key] = i
	if i < cap(c.groups) {
		c.groups = c.groups[:i+1]
		c.groups[i] = dfaTransitionSet{Configs: c.groups[i].Configs[:0], Particle: edge.Particle, Pos: edge.Pos, Key: key}
	} else {
		c.groups = append(c.groups, dfaTransitionSet{Particle: edge.Particle, Pos: edge.Pos, Key: key})
	}
	return &c.groups[i], nil
}

func (c *deterministicModelCompilation) appendTransition(row *compiledModelRow, group *dfaTransitionSet, stateID func([]dfaConfig) (uint32, error)) error {
	if err := c.builder.spendDFAConfigs(len(group.Configs)); err != nil {
		return err
	}
	group.Configs = c.normalize(group.Configs)
	if len(group.Configs) > c.builder.limit {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content model DFA state limit exceeded", nil)
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
		if err := addCounterCaps(caps, edge.Guards); err != nil {
			return err
		}
	}
	for _, accept := range row.Accept {
		if err := b.c.work.spend(len(accept.Guards) + 1); err != nil {
			return err
		}
		if err := addCounterCaps(caps, accept.Guards); err != nil {
			return err
		}
	}
	return nil
}

func (b *dfaBuilder) spendDFAConfigs(count int) error {
	if err := b.c.work.spend(count + 1); err != nil {
		return err
	}
	return b.c.work.spendProduct(count, int(b.counters))
}

func addCounterCaps(caps []uint32, guards []compiledGuard) error {
	for _, guard := range guards {
		if !validUint32Index(guard.Slot, len(caps)) {
			return xsderrors.InternalInvariant("content model DFA guard references invalid counter", nil)
		}
		if guard.N > 0 {
			caps[guard.Slot] = max(caps[guard.Slot], guard.N-1)
		}
	}
	return nil
}

func dfaGuardsOK(counters, caps []uint32, guards []compiledGuard) (bool, error) {
	for _, guard := range guards {
		if ok, err := dfaGuardOK(counters, caps, guard); err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func dfaGuardOK(counters, caps []uint32, guard compiledGuard) (bool, error) {
	if !validUint32Index(guard.Slot, len(counters)) || !validUint32Index(guard.Slot, len(caps)) {
		return false, xsderrors.InternalInvariant("content model DFA guard references invalid counter", nil)
	}
	count := counters[guard.Slot]
	switch guard.Kind {
	case compiledGuardExitMin:
		return guard.N == 0 || count >= guard.N-1, nil
	case compiledGuardLoopMax:
		return guard.N > 0 && count < guard.N-1, nil
	default:
		return false, xsderrors.InternalInvariant("content model DFA guard kind out of range", nil)
	}
}

func applyDFAActions(counters, caps []uint32, actions []compiledAction) error {
	for _, action := range actions {
		if !validUint32Index(action.Slot, len(counters)) || !validUint32Index(action.Slot, len(caps)) {
			return xsderrors.InternalInvariant("content model DFA action references invalid counter", nil)
		}
		switch action.Kind {
		case compiledActionInc:
			if counters[action.Slot] < caps[action.Slot] {
				counters[action.Slot]++
			}
		case compiledActionReset:
			counters[action.Slot] = 0
		default:
			return xsderrors.InternalInvariant("content model DFA action kind out of range", nil)
		}
	}
	return nil
}

func dfaTransitionKeyOf(p contentParticle, pos int) (dfaTransitionKey, error) {
	switch p.Kind {
	case particleElement:
		return dfaTransitionKey{Kind: p.Kind, ID: uint32(p.Element)}, nil
	case particleWildcard:
		return dfaTransitionKey{Kind: p.Kind, ID: uint32(p.Wildcard)}, nil
	case particleModel:
		id, err := checkedSchemaUint32(pos, "content model DFA position limit exceeded")
		return dfaTransitionKey{Kind: p.Kind, ID: id}, err
	default:
		return dfaTransitionKey{}, xsderrors.InternalInvariant("content model DFA edge particle has unknown kind", nil)
	}
}

func compareDFATransitionKey(a, b dfaTransitionKey) int {
	if n := cmp.Compare(a.Kind, b.Kind); n != 0 {
		return n
	}
	return cmp.Compare(a.ID, b.ID)
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
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	out := slices.Clone(a)
	out = append(out, b...)
	slices.Sort(out)
	return slices.Compact(out)
}

// normalizeDFAEntries sorts and deduplicates entries in place; callers pass
// a slice they own.
func normalizeDFAEntries(entries []dfaEntry) []dfaEntry {
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

// appendDFAStateKey's keys are equal exactly when the normalized entry sets
// are, so a key identifies a DFA state.
func appendDFAStateKey(b []byte, entries []dfaStateEntry) []byte {
	for _, e := range entries {
		b = binary.AppendVarint(b, int64(e.Pos))
		b = binary.AppendUvarint(b, uint64(e.Prefix))
	}
	return b
}

func decodeDFAStateKey(entries []dfaStateEntry, key string) []dfaStateEntry {
	for key != "" {
		pos, n := varintString(key)
		key = key[n:]
		prefix, n := uvarintString(key)
		key = key[n:]
		entries = append(entries, dfaStateEntry{
			Pos:    int(pos),
			Prefix: uint32(prefix), //nolint:gosec // The key encodes a uint32 prefix.
		})
	}
	return entries
}

// uvarintString panics unless s starts with a complete
// binary.AppendUvarint encoding.
func uvarintString(s string) (uint64, int) {
	var x uint64
	for i := 0; ; i++ {
		c := s[i]
		x |= uint64(c&0x7f) << (7 * i)
		if c < 0x80 {
			return x, i + 1
		}
	}
}

func varintString(s string) (int64, int) {
	ux, n := uvarintString(s)
	x := int64(ux >> 1)
	if ux&1 != 0 {
		x = ^x
	}
	return x, n
}
