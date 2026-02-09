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
	if len(s.normStack) > 0 {
		for i, buf := range s.normStack {
			if cap(buf) > maxSessionBuffer {
				s.normStack[i] = nil
			} else {
				s.normStack[i] = buf[:0]
			}
		}
		if len(s.normStack) > maxSessionEntries {
			s.normStack = nil
		}
	}
	if cap(s.errBuf) > maxSessionBuffer {
		s.errBuf = nil
	}
	if cap(s.valueBuf) > maxSessionBuffer {
		s.valueBuf = nil
	}
	if cap(s.valueScratch) > maxSessionBuffer {
		s.valueScratch = nil
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
	if s.nsStack.Cap() > maxSessionEntries {
		s.nsStack.Drop()
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
	if s.icState.frames.Cap() > maxSessionEntries {
		s.icState.frames.Drop()
	}
	if s.icState.scopes.Cap() > maxSessionEntries {
		s.icState.scopes.Drop()
	}
	if cap(s.icState.uncommittedViolations) > maxSessionEntries {
		s.icState.uncommittedViolations = nil
	}
	if cap(s.icState.committedViolations) > maxSessionEntries {
		s.icState.committedViolations = nil
	}

	if len(s.idTable) > maxSessionIDTableEntries {
		s.idTable = nil
	}
	if cap(s.identityAttrNames) > maxSessionEntries {
		s.identityAttrNames = nil
	}
	if len(s.identityAttrBuckets) > maxSessionEntries {
		s.identityAttrBuckets = nil
	}
}
