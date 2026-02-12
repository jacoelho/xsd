package validator

import "bytes"

func (s *Session) lookupNamespaceSmall(prefix []byte, frames []nsFrame) ([]byte, bool) {
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
