package validator

type attrSeenEntry struct {
	hash uint64
	idx  uint32
}

// AttributeTracker defines an exported type.
type AttributeTracker struct {
	attrAppliedBuf   []AttrApplied
	attrClassBuf     []attrClass
	attrPresent      []bool
	attrBuf          []StartAttr
	attrValidatedBuf []StartAttr
	attrSeenTable    []attrSeenEntry
}
