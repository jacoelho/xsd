package validator

import "strings"

func (s *Session) pathString() string {
	if s == nil || len(s.elemStack) == 0 {
		return "/"
	}
	total := 0
	for i := range s.elemStack {
		frame := &s.elemStack[i]
		ns, local := s.nameParts(frame.name)
		if len(local) == 0 {
			local = frame.local
			ns = frame.ns
		}
		if len(local) == 0 {
			continue
		}
		if len(ns) == 0 {
			total += 1 + len(local)
			continue
		}
		total += 1 + len(ns) + len(local) + 2
	}
	if total == 0 {
		return "/"
	}
	var b strings.Builder
	b.Grow(total)
	for i := range s.elemStack {
		frame := &s.elemStack[i]
		ns, local := s.nameParts(frame.name)
		if len(local) == 0 {
			local = frame.local
			ns = frame.ns
		}
		if len(local) == 0 {
			continue
		}
		b.WriteByte('/')
		if len(ns) > 0 {
			b.WriteByte('{')
			b.Write(ns)
			b.WriteByte('}')
		}
		b.Write(local)
	}
	out := b.String()
	if out == "" {
		return "/"
	}
	return out
}

func (s *Session) nameParts(id NameID) ([]byte, []byte) {
	if s == nil || id == 0 {
		return nil, nil
	}
	idx := int(id)
	var entry nameEntry
	if idx < len(s.nameMap) {
		entry = s.nameMap[idx]
	}
	if entry.LocalLen == 0 && entry.NSLen == 0 && entry.Sym == 0 && entry.NS == 0 {
		if s.nameMapSparse != nil {
			if sparse, ok := s.nameMapSparse[id]; ok {
				entry = sparse
			} else {
				return nil, nil
			}
		} else {
			return nil, nil
		}
	}
	var local []byte
	if entry.LocalLen != 0 {
		start, end, ok := checkedSpan(entry.LocalOff, entry.LocalLen, len(s.nameLocal))
		if ok {
			local = s.nameLocal[start:end]
		}
	}
	var ns []byte
	if entry.NSLen != 0 {
		start, end, ok := checkedSpan(entry.NSOff, entry.NSLen, len(s.nameNS))
		if ok {
			ns = s.nameNS[start:end]
		}
	}
	return ns, local
}
