package validator

func (s *Session) prefixCacheForCurrent() []prefixEntry {
	frame, ok := s.nsStack.Peek()
	if !ok {
		return nil
	}
	if int(frame.cacheOff) >= len(s.prefixCache) {
		return nil
	}
	return s.prefixCache[frame.cacheOff:]
}

func (s *Session) cachePrefix(prefix, ns []byte, ok bool, hash uint64) {
	if s == nil {
		return
	}
	prefixLen := len(prefix)
	prefixOff := 0
	if prefixLen > 0 {
		prefixOff = len(s.nameLocal)
		s.nameLocal = append(s.nameLocal, prefix...)
	}
	nsLen := len(ns)
	nsOff := 0
	if ok && nsLen > 0 {
		nsOff = len(s.nameNS)
		s.nameNS = append(s.nameNS, ns...)
	}
	s.prefixCache = append(s.prefixCache, prefixEntry{
		hash:      hash,
		prefixOff: uint32(prefixOff),
		prefixLen: uint32(prefixLen),
		nsOff:     uint32(nsOff),
		nsLen:     uint32(nsLen),
		ok:        ok,
	})
}

func (s *Session) cachePrefixDecl(decl nsDecl, ok bool, hash uint64) {
	if s == nil {
		return
	}
	s.prefixCache = append(s.prefixCache, prefixEntry{
		hash:      hash,
		prefixOff: decl.prefixOff,
		prefixLen: decl.prefixLen,
		nsOff:     decl.nsOff,
		nsLen:     decl.nsLen,
		ok:        ok,
	})
}
