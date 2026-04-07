package validator

import "github.com/jacoelho/xsd/internal/runtime"

// Start carries one start-element attribute and its interned metadata.
type Start struct {
	NSBytes    []byte
	Local      []byte
	Value      []byte
	KeyBytes   []byte
	Sym        runtime.SymbolID
	NS         runtime.NamespaceID
	NameCached bool
	KeyKind    runtime.ValueKind
}

// Class identifies one high-level validation role for an input attribute.
type Class uint8

const (
	ClassOther Class = iota
	ClassXSIKnown
	ClassXSIUnknown
	ClassXML
)

// SmallDuplicateThreshold is the linear-scan cutoff for duplicate detection.
const SmallDuplicateThreshold = 8

// Classification holds precomputed attribute roles plus captured xsi values.
type Classification struct {
	DuplicateErr error
	Classes      []Class
	XSIType      []byte
	XSINil       []byte
}

// SeenEntry stores one duplicate-detection table entry.
type SeenEntry struct {
	Hash  uint64
	Index uint32
}

// Tracker owns reusable buffers for attribute classification.
type Tracker struct {
	Classes   []Class
	Seen      []SeenEntry
	Present   []bool
	Starts    []Start
	Validated []Start
}

// Reset clears reusable classification buffers.
func (t *Tracker) Reset() {
	if t == nil {
		return
	}
	t.Classes = t.Classes[:0]
	t.Seen = t.Seen[:0]
	t.Present = t.Present[:0]
	t.Starts = t.Starts[:0]
	t.Validated = t.Validated[:0]
}

// Shrink releases oversized classification buffers after a session reset.
func (t *Tracker) Shrink(entryLimit int) {
	if t == nil {
		return
	}
	t.Classes = shrinkSliceCap(t.Classes, entryLimit)
	t.Seen = shrinkSliceCap(t.Seen, entryLimit)
	t.Present = shrinkSliceCap(t.Present, entryLimit)
	t.Starts = shrinkSliceCap(t.Starts, entryLimit)
	t.Validated = shrinkSliceCap(t.Validated, entryLimit)
}

// PreparePresent returns a zeroed presence bitmap sized for one attribute-use slice.
func (t *Tracker) PreparePresent(size int) []bool {
	present := t.Present
	if cap(present) < size {
		present = make([]bool, size)
	} else {
		present = present[:size]
		clear(present)
	}
	t.Present = present
	return present
}

// PrepareValidated returns a reusable validated-attribute buffer when storage is enabled.
func (t *Tracker) PrepareValidated(store bool, size int) []Start {
	if !store {
		return nil
	}
	validated := t.Validated
	if cap(validated) < size {
		validated = make([]Start, 0, size)
	} else {
		validated = validated[:0]
	}
	t.Validated = validated
	return validated
}
