package validator

const (
	maxSessionBuffer         = 4 << 20
	maxSessionEntries        = 1 << 14
	maxSessionIDTableEntries = maxSessionEntries
)

func (s *Session) shrinkBuffers() {
	if s == nil {
		return
	}
	if cap(s.nameLocal) > maxSessionBuffer {
		s.nameLocal = nil
	}
	if cap(s.nameNS) > maxSessionBuffer {
		s.nameNS = nil
	}
	if cap(s.textBuf) > maxSessionBuffer {
		s.textBuf = nil
	}
	if cap(s.normBuf) > maxSessionBuffer {
		s.normBuf = nil
	}
	if cap(s.errBuf) > maxSessionBuffer {
		s.errBuf = nil
	}
	if cap(s.valueBuf) > maxSessionBuffer {
		s.valueBuf = nil
	}
	if cap(s.keyBuf) > maxSessionBuffer {
		s.keyBuf = nil
	}
	if cap(s.keyTmp) > maxSessionBuffer {
		s.keyTmp = nil
	}

	if cap(s.attrPresent) > maxSessionEntries {
		s.attrPresent = nil
	}
	if cap(s.attrAppliedBuf) > maxSessionEntries {
		s.attrAppliedBuf = nil
	}
	if cap(s.nameMap) > maxSessionEntries {
		s.nameMap = nil
	}
	if len(s.nameMapSparse) > maxSessionEntries {
		s.nameMapSparse = nil
	}
	if cap(s.attrBuf) > maxSessionEntries {
		s.attrBuf = nil
	}
	if cap(s.attrValidatedBuf) > maxSessionEntries {
		s.attrValidatedBuf = nil
	}
	if cap(s.elemStack) > maxSessionEntries {
		s.elemStack = nil
	}
	if cap(s.nsDecls) > maxSessionEntries {
		s.nsDecls = nil
	}
	if cap(s.idRefs) > maxSessionEntries {
		s.idRefs = nil
	}
	if cap(s.nsStack) > maxSessionEntries {
		s.nsStack = nil
	}
	if cap(s.prefixCache) > maxSessionEntries {
		s.prefixCache = nil
	}
	if cap(s.attrSeenTable) > maxSessionEntries {
		s.attrSeenTable = nil
	}
	if cap(s.validationErrors) > maxSessionEntries {
		s.validationErrors = nil
	}
	if cap(s.icState.frames) > maxSessionEntries {
		s.icState.frames = nil
	}
	if cap(s.icState.scopes) > maxSessionEntries {
		s.icState.scopes = nil
	}
	if cap(s.icState.violations) > maxSessionEntries {
		s.icState.violations = nil
	}
	if cap(s.icState.pending) > maxSessionEntries {
		s.icState.pending = nil
	}

	if len(s.idTable) > maxSessionIDTableEntries {
		s.idTable = nil
	}
}
