package schema

import (
	"encoding/binary"
	"slices"

	"github.com/jacoelho/xsd/xsderrors"
)

// followTable holds the follow sets of the position automaton one dfaBuilder
// constructs. A concatenation or a repeat makes each tail of one entry set
// followed by every entry of another; the table records that as one segment
// per tail, naming the shared first set and the tail's interned guard and
// action prefix, so it grows with the tails and not with the tail-first
// pairs that automaton construction charges. A follow set is materialized
// once per distinct segment list, when a source row first reaches one of
// its positions. heads[pos] is one more than the index of the last segment
// added to pos, or zero; each segment links to the one added before it.
type followTable struct {
	prefixIDs map[string]uint32
	classIDs  map[string]uint32
	heads     []uint32
	segments  []followSegment
	scratch   []followSegment
	sets      []followSet
	prefixes  []followPrefix
	classes   []followClass
	classOf   []dfaStateID
	key       []byte
	endSet    dfaStateID
}

// followSegment stands for the entries composeEntry builds from the guards
// and actions of prefix and each entry of set.
type followSegment struct {
	prefix uint32
	set    uint32
	next   uint32
}

// followSet is a normalized entry set that segments share. work is the sum
// of its entries' guard and action counts. class is the class of the
// positions whose only segment is this set without a prefix.
type followSet struct {
	entries []dfaEntry
	work    int
	class   dfaStateID
}

// followPrefix holds normalized guards. Prefix 0 is the empty prefix.
type followPrefix struct {
	guards  []compiledGuard
	actions []compiledAction
}

// followClass is the normalized follow set of every position with one
// segment list. charge is what admitting the set as a state costs; state
// caches the state it was interned as, after which entries is released to
// the state's queued row.
type followClass struct {
	entries []dfaEntry
	charge  int
	state   dfaStateID
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

// addSet registers entries, a normalized set that is never modified, for
// segments to share.
func (t *followTable) addSet(entries []dfaEntry) (uint32, error) {
	id, err := followTableID(len(t.sets))
	if err != nil {
		return 0, err
	}
	work := 0
	for _, e := range entries {
		work += len(e.Guards) + len(e.Actions)
	}
	t.sets = appendDoubling(t.sets, followSet{entries: entries, work: work})
	return id, nil
}

func (t *followTable) rootEndSet() (uint32, error) {
	if !t.endSet.known {
		id, err := t.addSet([]dfaEntry{{Pos: dfaEndPos}})
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
	return len(t.sets[set].entries)
}

func (t *followTable) setWork(set uint32) int {
	return t.sets[set].work
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

// add makes pos followed by the entries of set composed with guards, which
// must be normalized, and actions.
func (t *followTable) add(pos int, guards []compiledGuard, actions []compiledAction, set uint32) error {
	prefix, err := t.prefix(guards, actions)
	if err != nil {
		return err
	}
	head, err := followTableID(len(t.segments) + 1)
	if err != nil {
		return err
	}
	t.segments = appendDoubling(t.segments, followSegment{prefix: prefix, set: set, next: t.heads[pos]})
	t.heads[pos] = head
	return nil
}

// positionSegments returns the segments of pos, newest first, in scratch
// storage that the next call reuses.
func (t *followTable) positionSegments(pos int) []followSegment {
	t.scratch = t.scratch[:0]
	for i := t.heads[pos]; i != 0; i = t.segments[i-1].next {
		t.scratch = append(t.scratch, t.segments[i-1])
	}
	return t.scratch
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

// segmentCharge counts one step per composed entry plus its guards and
// actions. Composition unions the prefix guards with each entry's guards.
func (t *followTable) segmentCharge(seg followSegment) int {
	set := t.sets[seg.set]
	p := t.prefixes[seg.prefix]
	charge := len(set.entries)*(1+len(p.actions)) + set.work
	if len(p.guards) == 0 {
		return charge
	}
	for _, e := range set.entries {
		charge += guardUnionLen(p.guards, e.Guards) - len(e.Guards)
	}
	return charge
}

// guardUnionLen returns the length of the normalized union of two
// normalized guard lists.
func guardUnionLen(a, b []compiledGuard) int {
	n, i, j := 0, 0, 0
	for i < len(a) && j < len(b) {
		switch c := compareCompiledGuard(a[i], b[j]); {
		case c < 0:
			i++
		case c > 0:
			j++
		default:
			i++
			j++
		}
		n++
	}
	return n + len(a) - i + len(b) - j
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

func (t *followTable) classID(segs []followSegment) (uint32, error) {
	if len(segs) == 1 && segs[0].prefix == 0 {
		set := &t.sets[segs[0].set]
		if !set.class.known {
			id, err := t.addClass(set.entries)
			if err != nil {
				return 0, err
			}
			set.class = dfaStateID{id: id, known: true}
		}
		return set.class.id, nil
	}
	t.key = t.key[:0]
	for _, seg := range segs {
		t.key = binary.LittleEndian.AppendUint32(t.key, seg.prefix)
		t.key = binary.LittleEndian.AppendUint32(t.key, seg.set)
	}
	if id, ok := t.classIDs[string(t.key)]; ok {
		return id, nil
	}
	id, err := t.addClass(t.materialize(segs))
	if err != nil {
		return 0, err
	}
	t.classIDs[string(t.key)] = id
	return id, nil
}

func (t *followTable) addClass(entries []dfaEntry) (uint32, error) {
	id, err := followTableID(len(t.classes))
	if err != nil {
		return 0, err
	}
	charge := len(entries) + 1
	for _, e := range entries {
		charge += len(e.Guards) + len(e.Actions)
	}
	t.classes = appendDoubling(t.classes, followClass{entries: entries, charge: charge})
	return id, nil
}

func (t *followTable) materialize(segs []followSegment) []dfaEntry {
	n := 0
	for _, seg := range segs {
		n += len(t.sets[seg.set].entries)
	}
	entries := make([]dfaEntry, 0, n)
	for _, seg := range segs {
		set := t.sets[seg.set].entries
		if seg.prefix == 0 {
			entries = append(entries, set...)
			continue
		}
		p := t.prefixes[seg.prefix]
		for _, e := range set {
			entries = append(entries, composeEntry(p.guards, p.actions, e))
		}
	}
	return slices.Clip(normalizeDFAEntries(entries))
}
