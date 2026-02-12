package validator

// ResetText is an exported function.
func (s *Session) ResetText(state *TextState) {
	if s == nil || state == nil {
		return
	}
	state.Off = uint32(len(s.textBuf))
	state.Len = 0
	state.HasText = false
	state.HasNonWS = false
}

// TextSlice is an exported function.
func (s *Session) TextSlice(state TextState) []byte {
	if s == nil {
		return nil
	}
	start := int(state.Off)
	end := start + int(state.Len)
	if start < 0 || end < start || end > len(s.textBuf) {
		return nil
	}
	return s.textBuf[start:end]
}

func (s *Session) releaseText(state TextState) {
	if s == nil {
		return
	}
	start := int(state.Off)
	end := start + int(state.Len)
	if start < 0 || end < start || end != len(s.textBuf) {
		return
	}
	s.textBuf = s.textBuf[:start]
}
