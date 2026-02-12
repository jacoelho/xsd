package validator

// Reset clears per-document state while retaining buffer capacity.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.Arena.Reset()
	s.Scratch.Reset()
	s.icState.arena = &s.Arena
	s.elemStack = s.elemStack[:0]
	s.nsStack.Reset()
	s.nsDecls = s.nsDecls[:0]
	s.prefixCache = s.prefixCache[:0]
	s.nameMap = s.nameMap[:0]
	s.nameMapSparse = nil
	s.nameLocal = s.nameLocal[:0]
	s.nameNS = s.nameNS[:0]
	s.textBuf = s.textBuf[:0]
	s.keyBuf = s.keyBuf[:0]
	s.keyTmp = s.keyTmp[:0]
	s.normBuf = s.normBuf[:0]
	s.normDepth = 0
	s.metricsDepth = 0
	s.errBuf = s.errBuf[:0]
	s.validationErrors = s.validationErrors[:0]
	s.valueBuf = s.valueBuf[:0]
	s.valueScratch = s.valueScratch[:0]
	s.AttributeTracker.Reset()
	s.icState.reset()
	s.documentURI = ""
	s.resetIDTable()
	s.idRefs = s.idRefs[:0]
	s.resetIdentityAttrBuckets()
	s.identityAttrNames = s.identityAttrNames[:0]
	s.shrinkBuffers()
}

func (s *Session) resetIDTable() {
	if s == nil || s.idTable == nil {
		return
	}
	if len(s.idTable) > maxSessionIDTableEntries {
		s.idTable = nil
		return
	}
	clear(s.idTable)
}

func (s *Session) resetIdentityAttrBuckets() {
	if s == nil || s.identityAttrBuckets == nil {
		return
	}
	if len(s.identityAttrBuckets) > maxSessionEntries {
		s.identityAttrBuckets = nil
		return
	}
	clear(s.identityAttrBuckets)
}
