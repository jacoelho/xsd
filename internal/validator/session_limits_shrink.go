package validator

func (s *Session) shrinkBuffers() {
	if s == nil {
		return
	}
	s.shrinkByteBuffers()
	s.normStack = shrinkNormStack(s.normStack, maxSessionBuffer, maxSessionEntries)
	s.shrinkEntryBuffers()
	s.shrinkIdentityBuffers()
	dropStacksOverCap(maxSessionEntries, &s.nsStack, &s.icState.frames, &s.icState.scopes)
}

func (s *Session) shrinkByteBuffers() {
	s.nameLocal = shrinkSliceCap(s.nameLocal, maxSessionBuffer)
	s.nameNS = shrinkSliceCap(s.nameNS, maxSessionBuffer)
	s.textBuf = shrinkSliceCap(s.textBuf, maxSessionBuffer)
	s.normBuf = shrinkSliceCap(s.normBuf, maxSessionBuffer)
	s.errBuf = shrinkSliceCap(s.errBuf, maxSessionBuffer)
	s.valueBuf = shrinkSliceCap(s.valueBuf, maxSessionBuffer)
	s.valueScratch = shrinkSliceCap(s.valueScratch, maxSessionBuffer)
	s.keyBuf = shrinkSliceCap(s.keyBuf, maxSessionBuffer)
	s.keyTmp = shrinkSliceCap(s.keyTmp, maxSessionBuffer)
}

func (s *Session) shrinkEntryBuffers() {
	s.attrPresent = shrinkSliceCap(s.attrPresent, maxSessionEntries)
	s.attrAppliedBuf = shrinkSliceCap(s.attrAppliedBuf, maxSessionEntries)
	s.nameMap = shrinkSliceCap(s.nameMap, maxSessionEntries)
	s.attrBuf = shrinkSliceCap(s.attrBuf, maxSessionEntries)
	s.attrValidatedBuf = shrinkSliceCap(s.attrValidatedBuf, maxSessionEntries)
	s.attrClassBuf = shrinkSliceCap(s.attrClassBuf, maxSessionEntries)
	s.elemStack = shrinkSliceCap(s.elemStack, maxSessionEntries)
	s.nsDecls = shrinkSliceCap(s.nsDecls, maxSessionEntries)
	s.idRefs = shrinkSliceCap(s.idRefs, maxSessionEntries)
	s.prefixCache = shrinkSliceCap(s.prefixCache, maxSessionEntries)
	s.attrSeenTable = shrinkSliceCap(s.attrSeenTable, maxSessionEntries)
	s.validationErrors = shrinkSliceCap(s.validationErrors, maxSessionEntries)
	s.metricsPool = shrinkSliceCap(s.metricsPool, maxSessionEntries)
}

func (s *Session) shrinkIdentityBuffers() {
	s.icState.uncommittedViolations = shrinkSliceCap(s.icState.uncommittedViolations, maxSessionEntries)
	s.icState.committedViolations = shrinkSliceCap(s.icState.committedViolations, maxSessionEntries)
	s.identityAttrNames = shrinkSliceCap(s.identityAttrNames, maxSessionEntries)
}
