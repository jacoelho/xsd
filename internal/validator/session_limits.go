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
	s.shrinkByteBuffers()
	s.normStack = shrinkNormStack(s.normStack, maxSessionBuffer, maxSessionEntries)
	s.shrinkEntryBuffers()
	s.shrinkIdentityBuffers()
	dropStacksOverCap(maxSessionEntries, &s.nsStack, &s.icState.frames, &s.icState.scopes)
}

func (s *Session) shrinkByteBuffers() {
	shrinkSliceCapRefs(maxSessionBuffer,
		&s.nameLocal,
		&s.nameNS,
		&s.textBuf,
		&s.normBuf,
		&s.errBuf,
		&s.valueBuf,
		&s.valueScratch,
		&s.keyBuf,
		&s.keyTmp,
	)
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
}

func (s *Session) shrinkIdentityBuffers() {
	s.icState.uncommittedViolations = shrinkSliceCap(s.icState.uncommittedViolations, maxSessionEntries)
	s.icState.committedViolations = shrinkSliceCap(s.icState.committedViolations, maxSessionEntries)
	s.identityAttrNames = shrinkSliceCap(s.identityAttrNames, maxSessionEntries)
}

func shrinkSliceCapRefs[T any](limit int, refs ...*[]T) {
	for _, ref := range refs {
		if ref == nil {
			continue
		}
		*ref = shrinkSliceCap(*ref, limit)
	}
}

func dropStacksOverCap(limit int, stacks ...interface {
	Cap() int
	Drop()
}) {
	for _, stack := range stacks {
		if stack != nil && stack.Cap() > limit {
			stack.Drop()
		}
	}
}

func shrinkSliceCap[T any](in []T, limit int) []T {
	if cap(in) > limit {
		return nil
	}
	return in
}

func shrinkNormStack(stack [][]byte, byteLimit, entryLimit int) [][]byte {
	if len(stack) == 0 {
		return stack
	}
	for i, buf := range stack {
		if cap(buf) > byteLimit {
			stack[i] = nil
		} else {
			stack[i] = buf[:0]
		}
	}
	if len(stack) > entryLimit {
		return nil
	}
	return stack
}
