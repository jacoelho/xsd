package schema

import (
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
)

var (
	followOnce      = occurrence{Min: 1, Max: 1}
	followOptional  = occurrence{Min: 0, Max: 1}
	followStar      = occurrence{Unbounded: true}
	followPlus      = occurrence{Min: 1, Unbounded: true}
	followTwoThree  = occurrence{Min: 2, Max: 3}
	followUpToTwo   = occurrence{Min: 0, Max: 2}
	followOneThree  = occurrence{Min: 1, Max: 3}
	followOccurs    = []occurrence{followOnce, followOnce, followOptional, followStar, followPlus, followTwoThree, followUpToTwo, followOneThree}
	followFixtureID = contentModelID(0)
)

func followSeq(occurs occurrence, particles ...contentParticle) contentModelNode {
	return contentModelNode{Kind: modelSequence, Occurs: occurs, Particles: particles}
}

func followChoice(occurs occurrence, particles ...contentParticle) contentModelNode {
	return contentModelNode{Kind: modelChoice, Occurs: occurs, Particles: particles}
}

func TestFollowTableMatchesPairwiseFollowSets(t *testing.T) {
	t.Parallel()

	el := elementParticle
	model := modelParticle
	for _, tt := range []struct {
		name   string
		models map[contentModelID]contentModelNode
	}{
		{
			name: "nested counted repeats",
			models: map[contentModelID]contentModelNode{
				0: followSeq(followTwoThree, model(1, followUpToTwo), el(4, followOneThree)),
				1: followSeq(followOnce, el(1, followOnce), model(2, followTwoThree)),
				2: followChoice(followOnce, el(2, followOnce), el(3, followUpToTwo)),
			},
		},
		{
			name: "multi-segment duplicates",
			models: map[contentModelID]contentModelNode{
				0: followSeq(followStar, el(1, followOptional), el(2, followOptional), el(3, followStar)),
			},
		},
		{
			name: "prefixed loop guards",
			models: map[contentModelID]contentModelNode{
				0: followSeq(followOnce, model(1, followOnce), el(3, followOnce)),
				1: followChoice(followOneThree, el(1, followTwoThree), el(2, followOptional)),
			},
		},
		{
			name: "wildcards",
			models: map[contentModelID]contentModelNode{
				0: followSeq(followPlus, wildcardParticle(1, followOnce), model(1, followUpToTwo), wildcardParticle(1, followOptional)),
				1: followChoice(followOnce, wildcardParticle(2, followOnce), el(1, followOnce)),
			},
		},
		{
			name: "group referenced from several tails",
			models: map[contentModelID]contentModelNode{
				0: followSeq(followOnce, model(1, followOnce), model(2, followOnce), model(2, followUpToTwo), model(2, followStar)),
				1: followChoice(followOnce, el(1, followOnce), el(2, followOptional)),
				2: followChoice(followOnce, el(3, followOnce), el(4, followOnce)),
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			checkFollowTableMatchesPairwise(t, tt.models)
		})
	}
}

func TestFollowTableMatchesPairwiseFollowSetsOnGeneratedModels(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // Reproducible generated models, not security-sensitive randomness.
	for i := range 200 {
		models := generatedFollowModels(rng)
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			checkFollowTableMatchesPairwise(t, models)
		})
	}
}

// generatedFollowModels returns up to four groups; group i refers only to
// groups above i, so the root, group 0, can reach a group through several
// particles.
func generatedFollowModels(rng *rand.Rand) map[contentModelID]contentModelNode {
	count := 1 + rng.Uint32N(4)
	models := make(map[contentModelID]contentModelNode, count)
	for id := range count {
		particles := make([]contentParticle, 1+rng.IntN(4))
		for j := range particles {
			occurs := followOccurs[rng.IntN(len(followOccurs))]
			switch n := rng.IntN(10); {
			case n < 2 && id+1 < count:
				particles[j] = modelParticle(contentModelID(id+1+rng.Uint32N(count-id-1)), occurs)
			case n < 4:
				particles[j] = wildcardParticle(WildcardID(rng.Uint32N(2)), occurs)
			default:
				particles[j] = elementParticle(ElementID(rng.Uint32N(5)), occurs)
			}
		}
		occurs := followOccurs[rng.IntN(len(followOccurs))]
		if rng.IntN(2) == 0 {
			models[contentModelID(id)] = followSeq(occurs, particles...)
		} else {
			models[contentModelID(id)] = followChoice(occurs, particles...)
		}
	}
	return models
}

func checkFollowTableMatchesPairwise(t *testing.T, models map[contentModelID]contentModelNode) {
	t.Helper()
	rt := compiledModelFixture{models: models}.build()
	root := models[followFixtureID]

	got := followFixtureBuilder(rt)
	gotRoot, err := got.groupNode(root)
	if err != nil {
		t.Fatalf("groupNode() error = %v", err)
	}
	if err := got.appendRootEndFollows(gotRoot.Last); err != nil {
		t.Fatalf("appendRootEndFollows() error = %v", err)
	}
	before := got.c.work.remaining
	if err := got.normalizeFollows(); err != nil {
		t.Fatalf("normalizeFollows() error = %v", err)
	}
	gotCharge := before - got.c.work.remaining

	want := pairwiseFollows{b: followFixtureBuilder(rt), follow: make(map[int][]dfaEntry)}
	wantRoot := want.group(t, root)
	want.rootEnd(wantRoot.Last)
	wantCharge := want.normalize()

	if !slices.Equal(gotRoot.First, wantRoot.First) || !equalDFAEntries(gotRoot.Last, wantRoot.Last) {
		t.Fatalf("root first, last = %v, %v, want %v, %v", gotRoot.First, gotRoot.Last, wantRoot.First, wantRoot.Last)
	}
	if gotCharge != wantCharge {
		t.Fatalf("normalization charge = %d, want %d", gotCharge, wantCharge)
	}
	if len(got.positions) != len(want.b.positions) {
		t.Fatalf("positions = %d, want %d", len(got.positions), len(want.b.positions))
	}
	created := make(map[uint32][]dfaStateEntry)
	for pos := range got.positions {
		ref, err := got.follow.class(pos)
		if err != nil {
			t.Fatalf("class(%d) error = %v", pos, err)
		}
		id := got.follow.classOf[pos].id
		if ref.created {
			created[id] = slices.Clone(ref.entries)
		}
		entries, ok := created[id]
		if !ok {
			t.Fatalf("class(%d) = class %d, which no earlier position created", pos, id)
		}
		if gotFollow := expandStateEntries(&got.follow, entries); !equalDFAEntries(gotFollow, want.follow[pos]) {
			t.Fatalf("follow(%d) = %v, want %v", pos, gotFollow, want.follow[pos])
		}
		key := appendDFAStateKey(nil, entries)
		if decoded := decodeDFAStateKey(nil, string(key)); !slices.Equal(decoded, entries) {
			t.Fatalf("follow(%d) key decodes to %v, want %v", pos, decoded, entries)
		}
		if wantAdmission := entriesCharge(want.follow[pos]); ref.class.charge != wantAdmission {
			t.Fatalf("follow(%d) charge = %d, want %d", pos, ref.class.charge, wantAdmission)
		}
	}
}

func followFixtureBuilder(rt *schemaBuild) *dfaBuilder {
	work := newWorkBudget(contentModelWorkBudget, math.MaxInt)
	return &dfaBuilder{
		c:      &contentModelCompiler{rt: rt, work: &work, retained: testDependencyWork()},
		follow: newFollowTable(),
		states: make(map[string]uint32),
		limit:  math.MaxInt,
	}
}

// pairwiseFollows is the reference position automaton: it stores one composed
// follow entry per tail and first position, independently of followTable.
type pairwiseFollows struct {
	b      *dfaBuilder
	follow map[int][]dfaEntry
}

func (r *pairwiseFollows) group(t *testing.T, model contentModelNode) dfaNode {
	t.Helper()
	node := dfaNode{Nullable: model.Kind != modelChoice}
	for _, p := range model.Particles {
		child := r.particle(t, p)
		if model.Kind == modelSequence {
			node = r.concat(node, child)
			continue
		}
		node = dfaNode{
			First:    pairwisePositionUnion(node.First, child.First),
			Last:     pairwiseUnion(node.Last, child.Last),
			Counters: mergeCounters(node.Counters, child.Counters),
			Nullable: node.Nullable || child.Nullable,
		}
	}
	return r.repeat(t, node, model.Occurs)
}

func (r *pairwiseFollows) particle(t *testing.T, p contentParticle) dfaNode {
	t.Helper()
	if p.Kind == particleModel {
		model, ok := contentModelByID(r.b.c.rt.Models, p.Model)
		if !ok {
			t.Fatalf("missing model %d", p.Model)
		}
		return r.repeat(t, r.group(t, model), p.Occurs)
	}
	return r.repeat(t, r.b.leaf(p), p.Occurs)
}

func (r *pairwiseFollows) concat(a, c dfaNode) dfaNode {
	for _, tail := range a.Last {
		for _, first := range c.First {
			r.add(tail.Pos, tail.Guards, tail.Actions, first)
		}
	}
	first, last := a.First, c.Last
	if a.Nullable {
		first = pairwisePositionUnion(a.First, c.First)
	}
	if c.Nullable {
		last = pairwiseUnion(c.Last, a.Last)
	}
	return dfaNode{First: first, Last: last, Counters: mergeCounters(a.Counters, c.Counters), Nullable: a.Nullable && c.Nullable}
}

func (r *pairwiseFollows) repeat(t *testing.T, child dfaNode, occurs occurrence) dfaNode {
	t.Helper()
	if occurs.IsAbsent() {
		return dfaNode{Nullable: true}
	}
	if occurs.IsExactlyOne() {
		return child
	}
	counter, err := r.b.repeatCounter(occurs)
	if err != nil {
		t.Fatalf("repeatCounter() error = %v", err)
	}
	last, err := r.b.repeatLastEntries(child, occurs, counter)
	if err != nil {
		t.Fatalf("repeatLastEntries() error = %v", err)
	}
	if occurs.Unbounded || occurs.Max > 1 {
		for _, tail := range child.Last {
			guards := slices.Clone(tail.Guards)
			if counter.active() && !occurs.Unbounded {
				guards = append(guards, compiledGuard{Slot: counter.slot, N: occurs.Max, Kind: compiledGuardLoopMax})
			}
			actions := slices.Clone(tail.Actions)
			if counter.active() {
				actions = append(actions, compiledAction{Slot: counter.slot, Kind: compiledActionInc})
			}
			actions = append(actions, resetActions(child.Counters, counter.slot)...)
			for _, first := range child.First {
				r.add(tail.Pos, guards, actions, first)
			}
		}
	}
	return dfaNode{
		First:    child.First,
		Last:     normalizeDFAEntries(last),
		Counters: repeatCounterSet(child.Counters, counter),
		Nullable: occurs.Min == 0 || child.Nullable,
	}
}

func (r *pairwiseFollows) add(pos int, guards []compiledGuard, actions []compiledAction, first int) {
	r.follow[pos] = append(r.follow[pos], dfaEntry{
		Pos:     first,
		Guards:  appendGuards(guards, nil),
		Actions: slices.Clone(actions),
	})
}

func (r *pairwiseFollows) rootEnd(last []dfaEntry) {
	for _, tail := range last {
		r.follow[tail.Pos] = append(r.follow[tail.Pos], dfaEntry{
			Pos: dfaEndPos, Guards: slices.Clone(tail.Guards), Actions: slices.Clone(tail.Actions),
		})
	}
}

// normalize sorts and deduplicates every follow set and returns what that
// costs: one step per position with a follow set and, per position, the
// admission charge of its entries before deduplication.
func (r *pairwiseFollows) normalize() int {
	charge := len(r.follow)
	for pos, entries := range r.follow {
		charge += entriesCharge(entries)
		r.follow[pos] = normalizeDFAEntries(entries)
	}
	return charge
}

func entriesCharge(entries []dfaEntry) int {
	charge := len(entries) + 1
	for _, e := range entries {
		charge += len(e.Guards) + len(e.Actions)
	}
	return charge
}

func pairwiseUnion(a, b []dfaEntry) []dfaEntry {
	return normalizeDFAEntries(slices.Concat(a, b))
}

func pairwisePositionUnion(a, b []int) []int {
	return slices.Compact(slices.Sorted(slices.Values(slices.Concat(a, b))))
}

func expandStateEntries(t *followTable, entries []dfaStateEntry) []dfaEntry {
	out := make([]dfaEntry, 0, len(entries))
	for _, e := range entries {
		p := t.prefixes[e.Prefix]
		out = append(out, dfaEntry{Pos: e.Pos, Guards: p.guards, Actions: p.actions})
	}
	return out
}

func equalDFAEntries(a, b []dfaEntry) bool {
	return slices.EqualFunc(a, b, func(x, y dfaEntry) bool { return compareDFAEntry(x, y) == 0 })
}
