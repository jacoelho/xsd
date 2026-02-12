package validator

// Reset is an exported function.
func (t *AttributeTracker) Reset() {
	if t == nil {
		return
	}
	t.attrAppliedBuf = t.attrAppliedBuf[:0]
	t.attrClassBuf = t.attrClassBuf[:0]
	t.attrBuf = t.attrBuf[:0]
	t.attrValidatedBuf = t.attrValidatedBuf[:0]
	t.attrPresent = t.attrPresent[:0]
	t.attrSeenTable = t.attrSeenTable[:0]
}
