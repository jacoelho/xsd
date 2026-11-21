package schema

import (
	"cmp"
	"encoding/binary"
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

// followTable holds the follow sets of the position automaton one dfaBuilder
// constructs. A concatenation or a repeat makes each tail of one entry set
// followed by every position of a first set; the table records that as one
// segment per tail, naming the shared first set and the tail's interned guard
// and action prefix, so it grows with the tails and not with the tail-first
// pairs that automaton construction charges. A follow set is materialized
// once per distinct segment list, when a source row first reaches one of its
// positions, into entries, which the next materialization reuses; the class
// keeps only its charge and state. heads[pos] is one more than the index of
// the last segment added to pos, or zero; each segment links to the one added
// before it.
type followTable struct {
	prefixIDs map[string]uint32
	classIDs  map[string]uint32
	heads     []uint32
	segments  followSegments
	scratch   []followSegment
	sets      []followSet
	prefixes  []followPrefix
	classes   []followClass
	classOf   []dfaStateID
	entries   []dfaStateEntry
	key       []byte
	endSet    dfaStateID
	// entriesOf is the class whose follow set entries holds.
	entriesOf dfaStateID
}

// followSegment stands for one entry per position of set, each carrying the
// guards and actions of prefix.
type followSegment struct {
	prefix uint32
	set    uint32
	next   uint32
}

// followSet is an ascending position set that segments share. class is the
// class of the positions whose only segment is this set without a prefix.
type followSet struct {
	positions []int
	class     dfaStateID
}

// followPrefix holds normalized guards. Prefix 0 is the empty prefix.
type followPrefix struct {
	guards  []compiledGuard
	actions []compiledAction
}

func (p followPrefix) len() int {
	return len(p.guards) + len(p.actions)
}

// dfaStateEntry is one entry of a source automaton state: a position, or
// dfaEndPos, taken with the guards and actions of a followTable prefix.
// Prefixes are interned by content, so two entries are equal exactly when
// their fields are.
type dfaStateEntry struct {
	Pos    int
	Prefix uint32
}

// followClass is the normalized follow set of every position with one
// segment list. charge is what admitting the set as a state costs; state
// caches the state it was interned as.
type followClass struct {
	charge int
	state  dfaStateID
}

func newFollowTable() followTable {
	return followTable{
		prefixes:  []followPrefix{{}},
		prefixIDs: make(map[string]uint32),
		classIDs:  make(map[string]uint32),
	}
}

func (t *followTable) addPosition() {
	t.heads = appendDoubling(t.heads, 0)
}

// addSet registers positions, an ascending set that is never modified, for
// segments to share.
func (t *followTable) addSet(positions []int) (uint32, error) {
	id, err := followTableID(len(t.sets))
	if err != nil {
		return 0, err
	}
	t.sets = appendDoubling(t.sets, followSet{positions: positions})
	return id, nil
}

func (t *followTable) rootEndSet() (uint32, error) {
	if !t.endSet.known {
		id, err := t.addSet([]int{dfaEndPos})
		if err != nil {
			return 0, err
		}
		t.endSet = dfaStateID{id: id, known: true}
	}
	return t.endSet.id, nil
}

func followTableID(n int) (uint32, error) {
	return checkedSchemaUint32(n, "content model follow table limit exceeded")
}

func (t *followTable) setLen(set uint32) int {
	return len(t.sets[set].positions)
}

// prefix interns guards, which must be normalized, with actions.
func (t *followTable) prefix(guards []compiledGuard, actions []compiledAction) (uint32, error) {
	if len(guards) == 0 && len(actions) == 0 {
		return 0, nil
	}
	t.key = t.key[:0]
	t.key = binary.AppendUvarint(t.key, uint64(len(guards)))
	for _, g := range guards {
		t.key = binary.AppendUvarint(t.key, uint64(g.Slot))
		t.key = binary.AppendUvarint(t.key, uint64(g.N))
		t.key = append(t.key, byte(g.Kind))
	}
	for _, a := range actions {
		t.key = binary.AppendUvarint(t.key, uint64(a.Slot))
		t.key = append(t.key, byte(a.Kind))
	}
	if id, ok := t.prefixIDs[string(t.key)]; ok {
		return id, nil
	}
	id, err := followTableID(len(t.prefixes))
	if err != nil {
		return 0, err
	}
	t.prefixes = append(t.prefixes, followPrefix{guards: guards, actions: actions})
	t.prefixIDs[string(t.key)] = id
	return id, nil
}

// add makes pos followed by every position of set, guarded by guards, which
// must be normalized, and taking actions.
func (t *followTable) add(pos int, guards []compiledGuard, actions []compiledAction, set uint32) error {
	prefix, err := t.prefix(guards, actions)
	if err != nil {
		return err
	}
	head, err := followTableID(t.segments.len() + 1)
	if err != nil {
		return err
	}
	t.segments.append(followSegment{prefix: prefix, set: set, next: t.heads[pos]})
	t.heads[pos] = head
	return nil
}

// positionSegments returns the segments of pos, newest first, in scratch
// storage that the next call reuses.
func (t *followTable) positionSegments(pos int) []followSegment {
	t.scratch = t.scratch[:0]
	for i := t.heads[pos]; i != 0; {
		seg := t.segments.at(i - 1)
		t.scratch = append(t.scratch, seg)
		i = seg.next
	}
	return t.scratch
}

// followSegmentChunk is the length of every full chunk of followSegments.
const followSegmentChunk = 1 << 14

// followSegments stores segments in chunks: the first grows by doubling, so
// small tables allocate as a slice would, and the rest are allocated full,
// so the store never copies a full chunk or holds more than one spare chunk.
type followSegments struct {
	chunks [][]followSegment
}

func (s *followSegments) len() int {
	if len(s.chunks) == 0 {
		return 0
	}
	return (len(s.chunks)-1)*followSegmentChunk + len(s.chunks[len(s.chunks)-1])
}

func (s *followSegments) append(seg followSegment) {
	last := len(s.chunks) - 1
	switch {
	case last < 0:
		s.chunks = append(s.chunks, nil)
		last = 0
	case len(s.chunks[last]) == followSegmentChunk:
		s.chunks = append(s.chunks, make([]followSegment, 0, followSegmentChunk))
		last++
	}
	if last == 0 {
		s.chunks[0] = appendDoubling(s.chunks[0], seg)
		return
	}
	s.chunks[last] = append(s.chunks[last], seg)
}

func (s *followSegments) at(i uint32) followSegment {
	return s.chunks[i/followSegmentChunk][i%followSegmentChunk]
}

// chargeNormalization charges normalizing the follow set of every position:
// one step per position with a follow set, and for each the steps
// spendDFAEntries charges for its composed entries before deduplication.
func (t *followTable) chargeNormalization(spend func(int) error) error {
	positions := 0
	for _, head := range t.heads {
		if head != 0 {
			positions++
		}
	}
	if err := spend(positions); err != nil {
		return err
	}
	for pos := range t.heads {
		if err := t.chargePosition(t.positionSegments(pos), spend); err != nil {
			return err
		}
	}
	t.classOf = make([]dfaStateID, len(t.heads))
	return nil
}

func (t *followTable) chargePosition(segs []followSegment, spend func(int) error) error {
	if len(segs) == 0 {
		return nil
	}
	if err := spend(1); err != nil {
		return err
	}
	for _, seg := range segs {
		if err := spend(t.segmentCharge(seg)); err != nil {
			return err
		}
	}
	return nil
}

func (t *followTable) segmentCharge(seg followSegment) int {
	return len(t.sets[seg.set].positions) * (1 + t.prefixes[seg.prefix].len())
}

// class returns the follow class of pos, materializing its follow set the
// first time a position with its segment list is reached.
func (t *followTable) class(pos int) (*followClass, error) {
	if pos < 0 || pos >= len(t.classOf) {
		return nil, xsderrors.InternalInvariant("content model DFA references invalid position")
	}
	ref := &t.classOf[pos]
	if !ref.known {
		id, err := t.classID(t.positionSegments(pos))
		if err != nil {
			return nil, err
		}
		*ref = dfaStateID{id: id, known: true}
	}
	return &t.classes[ref.id], nil
}

// classEntries returns the normalized follow set of pos, which class must
// have resolved, in storage that the next materialization reuses.
func (t *followTable) classEntries(pos int) []dfaStateEntry {
	if t.entriesOf != t.classOf[pos] {
		t.materialize(t.positionSegments(pos))
		t.entriesOf = t.classOf[pos]
	}
	return t.entries
}

func (t *followTable) classID(segs []followSegment) (uint32, error) {
	if len(segs) == 1 && segs[0].prefix == 0 {
		set := &t.sets[segs[0].set]
		if !set.class.known {
			t.entries = t.segmentEntries(segs[0], t.entries[:0])
			id, err := t.addClass()
			if err != nil {
				return 0, err
			}
			set.class = dfaStateID{id: id, known: true}
		}
		return set.class.id, nil
	}
	t.key = t.key[:0]
	for _, seg := range segs {
		t.key = binary.AppendUvarint(t.key, uint64(seg.prefix))
		t.key = binary.AppendUvarint(t.key, uint64(seg.set))
	}
	if id, ok := t.classIDs[string(t.key)]; ok {
		return id, nil
	}
	t.materialize(segs)
	id, err := t.addClass()
	if err != nil {
		return 0, err
	}
	t.classIDs[string(t.key)] = id
	return id, nil
}

// addClass adds the class whose follow set entries holds.
func (t *followTable) addClass() (uint32, error) {
	id, err := followTableID(len(t.classes))
	if err != nil {
		return 0, err
	}
	t.classes = appendDoubling(t.classes, followClass{charge: t.entriesCharge(t.entries)})
	t.entriesOf = dfaStateID{id: id, known: true}
	return id, nil
}

func (t *followTable) entriesCharge(entries []dfaStateEntry) int {
	charge := len(entries) + 1
	for _, e := range entries {
		charge += t.prefixes[e.Prefix].len()
	}
	return charge
}

func (t *followTable) materialize(segs []followSegment) {
	t.entries = t.entries[:0]
	for _, seg := range segs {
		t.entries = t.segmentEntries(seg, t.entries)
	}
	t.entries = t.normalize(t.entries)
}

func (t *followTable) segmentEntries(seg followSegment, entries []dfaStateEntry) []dfaStateEntry {
	for _, pos := range t.sets[seg.set].positions {
		entries = append(entries, dfaStateEntry{Pos: pos, Prefix: seg.prefix})
	}
	return entries
}

// normalize orders entries by prefix content rather than prefix ID, so a set
// normalizes to the same key whatever order its prefixes were interned in.
func (t *followTable) normalize(entries []dfaStateEntry) []dfaStateEntry {
	slices.SortFunc(entries, t.compareEntries)
	return slices.Compact(entries)
}

func (t *followTable) compareEntries(a, b dfaStateEntry) int {
	if n := cmp.Compare(a.Pos, b.Pos); n != 0 {
		return n
	}
	if a.Prefix == b.Prefix {
		return 0
	}
	pa, pb := t.prefixes[a.Prefix], t.prefixes[b.Prefix]
	if n := compareCompiledGuards(pa.guards, pb.guards); n != 0 {
		return n
	}
	return compareCompiledActions(pa.actions, pb.actions)
}
