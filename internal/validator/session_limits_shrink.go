package validator

func (s *Session) shrinkBuffers() {
	if s == nil {
		return
	}
	s.buffers.Shrink(maxSessionBuffer, maxSessionEntries)
	s.Scratch.Shrink(maxSessionBuffer)
	s.attrs.Shrink(maxSessionEntries)
	s.identity.Shrink(maxSessionEntries)
	s.Names.Shrink(maxSessionBuffer, maxSessionEntries)
	s.elemStack = shrinkSliceCap(s.elemStack, maxSessionEntries)
	s.validationErrors = shrinkSliceCap(s.validationErrors, maxSessionEntries)
}
