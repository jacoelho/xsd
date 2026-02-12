package validator

type attrSeenEntry struct {
	hash uint64
	idx  uint32
}

type AttributeTracker struct {
	attrAppliedBuf   []AttrApplied
	attrClassBuf     []attrClass
	attrPresent      []bool
	attrBuf          []StartAttr
	attrValidatedBuf []StartAttr
	attrSeenTable    []attrSeenEntry
}
