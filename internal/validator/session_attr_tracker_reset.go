package validator

// Reset clears reusable attribute-tracking buffers.
func (t *AttributeTracker) Reset() {
	if t == nil {
		return
	}
	t.attrAppliedBuf = t.attrAppliedBuf[:0]
	t.attrState.Reset()
}
