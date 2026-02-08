package validator

func (s *Session) pushNormBuf(size int) []byte {
	if s == nil {
		return nil
	}
	idx := s.normDepth
	if idx < len(s.normStack) {
		buf := s.normStack[idx]
		if cap(buf) < size {
			buf = make([]byte, 0, size)
		} else {
			buf = buf[:0]
		}
		s.normStack[idx] = buf
		s.normDepth++
		return buf
	}
	buf := make([]byte, 0, size)
	s.normStack = append(s.normStack, buf)
	s.normDepth++
	return buf
}

func (s *Session) popNormBuf() {
	if s == nil {
		return
	}
	if s.normDepth > 0 {
		s.normDepth--
	}
}

func (s *Session) hasIdentityConstraints() bool {
	return s != nil && s.rt != nil && len(s.rt.ICs) > 1
}
