package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

const smallNamespaceDeclThreshold = 32

var xmlNamespaceBytes = value.XMLNamespaceBytes()

type NameID uint32

type NameEntry struct {
	Sym      runtime.SymbolID
	NS       runtime.NamespaceID
	LocalOff uint32
	LocalLen uint32
	NSOff    uint32
	NSLen    uint32
}

type NamespaceScopeFrame struct {
	Off      uint32
	Len      uint32
	CacheOff uint32
}

type namespaceDeclEntry struct {
	PrefixOff  uint32
	PrefixLen  uint32
	NSOff      uint32
	NSLen      uint32
	PrefixHash uint64
}

type prefixCacheEntry struct {
	Hash      uint64
	PrefixOff uint32
	PrefixLen uint32
	NSOff     uint32
	NSLen     uint32
	OK        bool
}

type NameState struct {
	Sparse         map[NameID]NameEntry
	Dense          []NameEntry
	Local          []byte
	NS             []byte
	PrefixCache    []prefixCacheEntry
	NamespaceDecls []namespaceDeclEntry
	Scopes         Stack[NamespaceScopeFrame]
}

func schemaNamespaceID(rt *runtime.Schema, nsBytes []byte) runtime.NamespaceID {
	if len(nsBytes) == 0 {
		if rt != nil {
			return rt.PredefNS.Empty
		}
		return 0
	}
	if rt == nil {
		return 0
	}
	return rt.Namespaces.Lookup(nsBytes)
}

func (s *NameState) Intern(rt *runtime.Schema, id xmlstream.NameID, nsBytes, local []byte) NameEntry {
	if id == 0 {
		return NameEntry{NS: schemaNamespaceID(rt, nsBytes)}
	}
	idx := int(id)
	if idx >= maxNameMapSize {
		return s.internSparse(rt, NameID(id), nsBytes, local)
	}
	if idx >= len(s.Dense) {
		s.Dense = append(s.Dense, make([]NameEntry, idx-len(s.Dense)+1)...)
	}
	entry := s.Dense[idx]
	if entry.LocalLen != 0 || entry.NSLen != 0 || entry.Sym != 0 || entry.NS != 0 {
		return entry
	}
	entry = s.storeEntry(rt, nsBytes, local)
	s.Dense[idx] = entry
	return entry
}

func (s *NameState) internSparse(rt *runtime.Schema, id NameID, nsBytes, local []byte) NameEntry {
	if s == nil {
		return NameEntry{}
	}
	if s.Sparse == nil {
		s.Sparse = make(map[NameID]NameEntry)
	}
	if entry, ok := s.Sparse[id]; ok {
		return entry
	}
	if len(s.Sparse) >= maxNameMapSize {
		nsID := schemaNamespaceID(rt, nsBytes)
		sym := runtime.SymbolID(0)
		if nsID != 0 && rt != nil {
			sym = rt.Symbols.Lookup(nsID, local)
		}
		return NameEntry{Sym: sym, NS: nsID}
	}
	entry := s.storeEntry(rt, nsBytes, local)
	s.Sparse[id] = entry
	return entry
}

func (s *NameState) storeEntry(rt *runtime.Schema, nsBytes, local []byte) NameEntry {
	localOff := len(s.Local)
	s.Local = append(s.Local, local...)
	nsOff := len(s.NS)
	s.NS = append(s.NS, nsBytes...)
	nsID := schemaNamespaceID(rt, nsBytes)
	sym := runtime.SymbolID(0)
	if nsID != 0 && rt != nil {
		sym = rt.Symbols.Lookup(nsID, local)
	}
	return NameEntry{
		Sym:      sym,
		NS:       nsID,
		LocalOff: uint32(localOff),
		LocalLen: uint32(len(local)),
		NSOff:    uint32(nsOff),
		NSLen:    uint32(len(nsBytes)),
	}
}

func (s *NameState) Parts(id NameID) ([]byte, []byte) {
	if s == nil || id == 0 {
		return nil, nil
	}
	entry, ok := s.entryForID(id)
	if !ok {
		return nil, nil
	}
	return s.EntryBytes(entry)
}

func (s *NameState) EntryBytes(entry NameEntry) ([]byte, []byte) {
	if s == nil {
		return nil, nil
	}
	var local []byte
	if entry.LocalLen != 0 {
		start, end, ok := checkedSpan(entry.LocalOff, entry.LocalLen, len(s.Local))
		if ok {
			local = s.Local[start:end]
		}
	}
	var ns []byte
	if entry.NSLen != 0 {
		start, end, ok := checkedSpan(entry.NSOff, entry.NSLen, len(s.NS))
		if ok {
			ns = s.NS[start:end]
		}
	}
	return ns, local
}

func (s *NameState) entryForID(id NameID) (NameEntry, bool) {
	idx := int(id)
	if idx < len(s.Dense) {
		entry := s.Dense[idx]
		if entry.LocalLen != 0 || entry.NSLen != 0 || entry.Sym != 0 || entry.NS != 0 {
			return entry, true
		}
	}
	if s.Sparse == nil {
		return NameEntry{}, false
	}
	entry, ok := s.Sparse[id]
	return entry, ok
}

func (s *NameState) Stabilize(local, ns []byte, cached bool) ([]byte, []byte, bool) {
	if s == nil || cached {
		return local, ns, cached
	}
	if len(local) > 0 {
		off := len(s.Local)
		s.Local = append(s.Local, local...)
		local = s.Local[off : off+len(local)]
	}
	if len(ns) > 0 {
		off := len(s.NS)
		s.NS = append(s.NS, ns...)
		ns = s.NS[off : off+len(ns)]
	}
	return local, ns, true
}

func (s *NameState) PushNamespaceScope(decls []xmlstream.NamespaceDecl) {
	if s == nil {
		return
	}
	off := len(s.NamespaceDecls)
	cacheOff := len(s.PrefixCache)
	declLen := 0
	for _, decl := range decls {
		declLen++
		prefixOff := len(s.Local)
		s.Local = append(s.Local, decl.Prefix...)
		prefixLen := len(decl.Prefix)
		nsOff := len(s.NS)
		s.NS = append(s.NS, decl.URI...)
		nsLen := len(decl.URI)
		prefixBytes := s.Local[prefixOff : prefixOff+prefixLen]
		s.NamespaceDecls = append(s.NamespaceDecls, namespaceDeclEntry{
			PrefixOff:  uint32(prefixOff),
			PrefixLen:  uint32(prefixLen),
			NSOff:      uint32(nsOff),
			NSLen:      uint32(nsLen),
			PrefixHash: runtime.HashBytes(prefixBytes),
		})
	}
	s.Scopes.Push(NamespaceScopeFrame{Off: uint32(off), Len: uint32(declLen), CacheOff: uint32(cacheOff)})
}

func (s *NameState) PopNamespaceScope() {
	if s == nil {
		return
	}
	frame, ok := s.Scopes.Pop()
	if !ok {
		return
	}
	if int(frame.Off) <= len(s.NamespaceDecls) {
		s.NamespaceDecls = s.NamespaceDecls[:frame.Off]
	}
	if int(frame.CacheOff) <= len(s.PrefixCache) {
		s.PrefixCache = s.PrefixCache[:frame.CacheOff]
	}
}

func (s *NameState) LookupNamespace(prefix []byte) ([]byte, bool) {
	if s == nil {
		return nil, false
	}
	if isXMLPrefix(prefix) {
		return xmlNamespaceBytes, true
	}
	frames := s.Scopes.Items()
	if len(s.NamespaceDecls) <= smallNamespaceDeclThreshold {
		return s.lookupNamespaceSmall(prefix, frames)
	}
	return s.lookupNamespaceHashed(prefix, frames)
}

func (s *NameState) lookupNamespaceSmall(prefix []byte, frames []NamespaceScopeFrame) ([]byte, bool) {
	if len(prefix) == 0 {
		for i := len(frames) - 1; i >= 0; i-- {
			frame := frames[i]
			for j := int(frame.Off + frame.Len); j > int(frame.Off); j-- {
				decl := s.NamespaceDecls[j-1]
				if decl.PrefixLen != 0 {
					continue
				}
				return s.NS[decl.NSOff : decl.NSOff+decl.NSLen], true
			}
		}
		return nil, true
	}
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		for j := int(frame.Off + frame.Len); j > int(frame.Off); j-- {
			decl := s.NamespaceDecls[j-1]
			if decl.PrefixLen == 0 {
				continue
			}
			prefixBytes := s.Local[decl.PrefixOff : decl.PrefixOff+decl.PrefixLen]
			if bytes.Equal(prefixBytes, prefix) {
				return s.NS[decl.NSOff : decl.NSOff+decl.NSLen], true
			}
		}
	}
	return nil, false
}

func (s *NameState) lookupNamespaceHashed(prefix []byte, frames []NamespaceScopeFrame) ([]byte, bool) {
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

func (s *NameState) lookupNamespaceFromCache(prefix []byte, hash uint64) ([]byte, bool, bool) {
	cache := s.prefixCacheForCurrent()
	for i := range cache {
		entry := &cache[i]
		if entry.Hash != hash {
			continue
		}
		if entry.PrefixLen == 0 {
			if len(prefix) != 0 {
				continue
			}
			return s.cachedNamespace(entry)
		}
		if len(prefix) != int(entry.PrefixLen) {
			continue
		}
		prefixBytes := s.Local[entry.PrefixOff : entry.PrefixOff+entry.PrefixLen]
		if !bytes.Equal(prefixBytes, prefix) {
			continue
		}
		return s.cachedNamespace(entry)
	}
	return nil, false, false
}

func (s *NameState) cachedNamespace(entry *prefixCacheEntry) ([]byte, bool, bool) {
	if !entry.OK {
		return nil, false, true
	}
	if entry.NSLen == 0 {
		return nil, true, true
	}
	return s.NS[entry.NSOff : entry.NSOff+entry.NSLen], true, true
}

func (s *NameState) lookupNamespaceByHash(prefix []byte, frames []NamespaceScopeFrame, hash uint64) ([]byte, bool, bool) {
	for i := len(frames) - 1; i >= 0; i-- {
		frame := frames[i]
		for j := int(frame.Off + frame.Len); j > int(frame.Off); j-- {
			decl := s.NamespaceDecls[j-1]
			if decl.PrefixHash != hash {
				continue
			}
			if decl.PrefixLen == 0 {
				if len(prefix) != 0 {
					continue
				}
				ns := s.NS[decl.NSOff : decl.NSOff+decl.NSLen]
				s.cachePrefixDecl(decl, true, hash)
				return ns, true, true
			}
			if len(prefix) != int(decl.PrefixLen) {
				continue
			}
			prefixBytes := s.Local[decl.PrefixOff : decl.PrefixOff+decl.PrefixLen]
			if !bytes.Equal(prefixBytes, prefix) {
				continue
			}
			ns := s.NS[decl.NSOff : decl.NSOff+decl.NSLen]
			s.cachePrefixDecl(decl, true, hash)
			return ns, true, true
		}
	}
	return nil, false, false
}

func (s *NameState) prefixCacheForCurrent() []prefixCacheEntry {
	frame, ok := s.Scopes.Peek()
	if !ok || int(frame.CacheOff) >= len(s.PrefixCache) {
		return nil
	}
	return s.PrefixCache[frame.CacheOff:]
}

func (s *NameState) cachePrefix(prefix, ns []byte, ok bool, hash uint64) {
	prefixLen := len(prefix)
	prefixOff := 0
	if prefixLen > 0 {
		prefixOff = len(s.Local)
		s.Local = append(s.Local, prefix...)
	}
	nsLen := len(ns)
	nsOff := 0
	if ok && nsLen > 0 {
		nsOff = len(s.NS)
		s.NS = append(s.NS, ns...)
	}
	s.PrefixCache = append(s.PrefixCache, prefixCacheEntry{
		Hash:      hash,
		PrefixOff: uint32(prefixOff),
		PrefixLen: uint32(prefixLen),
		NSOff:     uint32(nsOff),
		NSLen:     uint32(nsLen),
		OK:        ok,
	})
}

func (s *NameState) cachePrefixDecl(decl namespaceDeclEntry, ok bool, hash uint64) {
	s.PrefixCache = append(s.PrefixCache, prefixCacheEntry{
		Hash:      hash,
		PrefixOff: decl.PrefixOff,
		PrefixLen: decl.PrefixLen,
		NSOff:     decl.NSOff,
		NSLen:     decl.NSLen,
		OK:        ok,
	})
}

func (s *NameState) Reset() {
	if s == nil {
		return
	}
	s.Scopes.Reset()
	s.NamespaceDecls = s.NamespaceDecls[:0]
	s.PrefixCache = s.PrefixCache[:0]
	s.Dense = s.Dense[:0]
	s.Sparse = nil
	s.Local = s.Local[:0]
	s.NS = s.NS[:0]
}

func (s *NameState) Shrink(bufferLimit, entryLimit int) {
	if s == nil {
		return
	}
	s.Local = shrinkSliceCap(s.Local, bufferLimit)[:0]
	s.NS = shrinkSliceCap(s.NS, bufferLimit)[:0]
	s.Dense = shrinkSliceCap(s.Dense, entryLimit)[:0]
	s.NamespaceDecls = shrinkSliceCap(s.NamespaceDecls, entryLimit)[:0]
	s.PrefixCache = shrinkSliceCap(s.PrefixCache, entryLimit)[:0]
	if s.Scopes.Cap() > entryLimit {
		s.Scopes.Drop()
	}
}

func isXMLPrefix(prefix []byte) bool {
	return len(prefix) == 3 && prefix[0] == 'x' && prefix[1] == 'm' && prefix[2] == 'l'
}
