package validator

type attrSeenEntry struct {
	hash uint64
	idx  uint32
}

// AttributeTracker owns reusable buffers used during attribute validation.
type AttributeTracker struct {
	attrAppliedBuf   []AttrApplied
	attrClassBuf     []attrClass
	attrPresent      []bool
	attrBuf          []StartAttr
	attrValidatedBuf []StartAttr
	attrSeenTable    []attrSeenEntry
}
