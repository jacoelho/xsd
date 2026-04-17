package validator

// Reset clears reusable attribute-tracking buffers.
func (t *AttributeTracker) Reset() {
	if t == nil {
		return
	}
	t.attrAppliedBuf = t.attrAppliedBuf[:0]
	t.attrState.Reset()
}

func (t *AttributeTracker) Shrink(entryLimit int) {
	if t == nil {
		return
	}
	t.attrAppliedBuf = shrinkSliceCap(t.attrAppliedBuf, entryLimit)
	t.attrState.Shrink(entryLimit)
}
