package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) lookupNamespaceHashed(prefix []byte, frames []nsFrame) ([]byte, bool) {
	hash := runtime.HashBytes(prefix)
	if ns, ok, found := s.lookupNamespaceFromCache(prefix, hash); found {
		return ns, ok
	}
	if ns, ok, found := s.lookupNamespaceByHash(prefix, frames, hash); found {
		return ns, ok
	}
	if len(prefix) == 0 {
		s.cachePrefix(prefix, nil, true, hash)
		return nil, true
	}
	s.cachePrefix(prefix, nil, false, hash)
	return nil, false
}

func (s *Session) lookupNamespaceFromCache(prefix []byte, hash uint64) ([]byte, bool, bool) {
	if cache := s.prefixCacheForCurrent(); len(cache) > 0 {
		for i := range cache {
			entry := &cache[i]
			if entry.hash != hash {
				continue
			}
			if entry.prefixLen == 0 {
				if len(prefix) != 0 {
					continue
				}
				if !entry.ok {
					return nil, false, true
				}
				if entry.nsLen == 0 {
					return nil, true, true
				}
				return s.nameNS[entry.nsOff : entry.nsOff+entry.nsLen], true, true
			}
			if len(prefix) != int(entry.prefixLen) {
				continue
			}
			prefixBytes := s.nameLocal[entry.prefixOff : entry.prefixOff+entry.prefixLen]
			if !bytes.Equal(prefixBytes, prefix) {
				continue
			}
			if !entry.ok {
				return nil, false, true
			}
			if entry.nsLen == 0 {
				return nil, true, true
			}
			return s.nameNS[entry.nsOff : entry.nsOff+entry.nsLen], true, true
		}
	}
	return nil, false, false
}

func (s *Session) lookupNamespaceByHash(prefix []byte, frames []nsFrame, hash uint64) ([]byte, bool, bool) {
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		for j := int(frame.off + frame.len); j > int(frame.off); j-- {
			decl := s.nsDecls[j-1]
			if decl.prefixHash != hash {
				continue
			}
			if decl.prefixLen == 0 {
				if len(prefix) != 0 {
					continue
				}
				ns := s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen]
				s.cachePrefixDecl(decl, true, hash)
				return ns, true, true
			}
			if len(prefix) != int(decl.prefixLen) {
				continue
			}
			prefixBytes := s.nameLocal[decl.prefixOff : decl.prefixOff+decl.prefixLen]
			if !bytes.Equal(prefixBytes, prefix) {
				continue
			}
			ns := s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen]
			s.cachePrefixDecl(decl, true, hash)
			return ns, true, true
		}
	}
	return nil, false, false
}
