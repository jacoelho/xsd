package validator

func (s *Session) shrinkBuffers() {
	if s == nil {
		return
	}
	s.shrinkByteBuffers()
	s.normStack = shrinkNormStack(s.normStack, maxSessionBuffer, maxSessionEntries)
	s.shrinkEntryBuffers()
	s.shrinkIdentityBuffers()
	s.Names.Shrink(maxSessionBuffer, maxSessionEntries)
	dropStacksOverCap(maxSessionEntries, &s.icState.Frames, &s.icState.Scopes)
}

func (s *Session) shrinkByteBuffers() {
	s.textBuf = shrinkSliceCap(s.textBuf, maxSessionBuffer)
	s.normBuf = shrinkSliceCap(s.normBuf, maxSessionBuffer)
	s.errBuf = shrinkSliceCap(s.errBuf, maxSessionBuffer)
	s.valueBuf = shrinkSliceCap(s.valueBuf, maxSessionBuffer)
	s.valueScratch = shrinkSliceCap(s.valueScratch, maxSessionBuffer)
	s.keyBuf = shrinkSliceCap(s.keyBuf, maxSessionBuffer)
	s.keyTmp = shrinkSliceCap(s.keyTmp, maxSessionBuffer)
}

func (s *Session) shrinkEntryBuffers() {
	s.attrAppliedBuf = shrinkSliceCap(s.attrAppliedBuf, maxSessionEntries)
	s.attrState.Shrink(maxSessionEntries)
	s.elemStack = shrinkSliceCap(s.elemStack, maxSessionEntries)
	s.idRefs = shrinkSliceCap(s.idRefs, maxSessionEntries)
	s.validationErrors = shrinkSliceCap(s.validationErrors, maxSessionEntries)
	s.metricsPool = shrinkSliceCap(s.metricsPool, maxSessionEntries)
}

func (s *Session) shrinkIdentityBuffers() {
	s.icState.Uncommitted = shrinkSliceCap(s.icState.Uncommitted, maxSessionEntries)
	s.icState.Committed = shrinkSliceCap(s.icState.Committed, maxSessionEntries)
}
