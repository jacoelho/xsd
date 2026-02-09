package validator

import (
	"bytes"
	"iter"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xmlnames"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

var xmlNamespaceBytes = xmlnames.XMLNamespaceBytes()

func (s *Session) pushNamespaceScope(decls iter.Seq[xmlstream.NamespaceDecl]) {
	off := len(s.nsDecls)
	cacheOff := len(s.prefixCache)
	declLen := 0
	if decls != nil {
		for decl := range decls {
			declLen++
			prefixOff := len(s.nameLocal)
			s.nameLocal = append(s.nameLocal, decl.Prefix...)
			prefixLen := len(decl.Prefix)
			nsOff := len(s.nameNS)
			s.nameNS = append(s.nameNS, decl.URI...)
			nsLen := len(decl.URI)
			prefixBytes := s.nameLocal[prefixOff : prefixOff+prefixLen]
			s.nsDecls = append(s.nsDecls, nsDecl{
				prefixOff:  uint32(prefixOff),
				prefixLen:  uint32(prefixLen),
				nsOff:      uint32(nsOff),
				nsLen:      uint32(nsLen),
				prefixHash: runtime.HashBytes(prefixBytes),
			})
		}
	}
	s.nsStack.Push(nsFrame{off: uint32(off), len: uint32(declLen), cacheOff: uint32(cacheOff)})
}

func (s *Session) popNamespaceScope() {
	frame, ok := s.nsStack.Pop()
	if !ok {
		return
	}
	if int(frame.off) <= len(s.nsDecls) {
		s.nsDecls = s.nsDecls[:frame.off]
	}
	if int(frame.cacheOff) <= len(s.prefixCache) {
		s.prefixCache = s.prefixCache[:frame.cacheOff]
	}
}

func (s *Session) lookupNamespace(prefix []byte) ([]byte, bool) {
	if isXMLPrefix(prefix) {
		return xmlNamespaceBytes, true
	}
	const smallNSDeclThreshold = 32
	frames := s.nsStack.Items()
	if len(s.nsDecls) <= smallNSDeclThreshold {
		if len(prefix) == 0 {
			for i := len(frames) - 1; i >= 0; i-- {
				frame := frames[i]
				for j := int(frame.off + frame.len); j > int(frame.off); j-- {
					decl := s.nsDecls[j-1]
					if decl.prefixLen != 0 {
						continue
					}
					return s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen], true
				}
			}
			return nil, true
		}
		for i := len(frames) - 1; i >= 0; i-- {
			frame := frames[i]
			for j := int(frame.off + frame.len); j > int(frame.off); j-- {
				decl := s.nsDecls[j-1]
				if decl.prefixLen == 0 {
					continue
				}
				prefixBytes := s.nameLocal[decl.prefixOff : decl.prefixOff+decl.prefixLen]
				if bytes.Equal(prefixBytes, prefix) {
					return s.nameNS[decl.nsOff : decl.nsOff+decl.nsLen], true
				}
			}
		}
		return nil, false
	}

	hash := runtime.HashBytes(prefix)
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
				if entry.ok {
					if entry.nsLen == 0 {
						return nil, true
					}
					return s.nameNS[entry.nsOff : entry.nsOff+entry.nsLen], true
				}
				return nil, false
			}
			if len(prefix) != int(entry.prefixLen) {
				continue
			}
			prefixBytes := s.nameLocal[entry.prefixOff : entry.prefixOff+entry.prefixLen]
			if !bytes.Equal(prefixBytes, prefix) {
				continue
			}
			if entry.ok {
				if entry.nsLen == 0 {
					return nil, true
				}
				return s.nameNS[entry.nsOff : entry.nsOff+entry.nsLen], true
			}
			return nil, false
		}
	}
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
				return ns, true
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
			return ns, true
		}
	}
	if len(prefix) == 0 {
		s.cachePrefix(prefix, nil, true, hash)
		return nil, true
	}
	s.cachePrefix(prefix, nil, false, hash)
	return nil, false
}

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

func isXMLPrefix(prefix []byte) bool {
	return len(prefix) == 3 && prefix[0] == 'x' && prefix[1] == 'm' && prefix[2] == 'l'
}
