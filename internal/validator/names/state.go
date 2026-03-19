package names

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/stack"
	"github.com/jacoelho/xsd/internal/xmlnames"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

const (
	smallNamespaceDeclThreshold = 32
	maxDenseNameMapSize         = 1 << 20
)

var xmlNamespaceBytes = xmlnames.XMLNamespaceBytes()

// ID identifies a name entry within a single document.
type ID uint32

// Entry stores one interned expanded name.
type Entry struct {
	Sym      runtime.SymbolID
	NS       runtime.NamespaceID
	LocalOff uint32
	LocalLen uint32
	NSOff    uint32
	NSLen    uint32
}

// ScopeFrame marks one namespace declaration scope on the element stack.
type ScopeFrame struct {
	Off      uint32
	Len      uint32
	CacheOff uint32
}

// NamespaceDecl stores one declared prefix mapping in the session-local name buffers.
type NamespaceDecl struct {
	PrefixOff  uint32
	PrefixLen  uint32
	NSOff      uint32
	NSLen      uint32
	PrefixHash uint64
}

// PrefixEntry stores one namespace lookup cache entry for the current scope.
type PrefixEntry struct {
	Hash      uint64
	PrefixOff uint32
	PrefixLen uint32
	NSOff     uint32
	NSLen     uint32
	OK        bool
}

// State owns per-session expanded-name storage and namespace lookup caches.
type State struct {
	Sparse         map[ID]Entry
	Dense          []Entry
	Local          []byte
	NS             []byte
	PrefixCache    []PrefixEntry
	NamespaceDecls []NamespaceDecl
	Scopes         stack.Stack[ScopeFrame]
}

// NamespaceID resolves one namespace URI against the compiled runtime schema.
func NamespaceID(rt *runtime.Schema, nsBytes []byte) runtime.NamespaceID {
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

// Intern resolves and stores one expanded element or attribute name.
func (s *State) Intern(rt *runtime.Schema, id xmlstream.NameID, nsBytes, local []byte) Entry {
	if id == 0 {
		return Entry{NS: NamespaceID(rt, nsBytes)}
	}
	idx := int(id)
	if idx >= maxDenseNameMapSize {
		return s.internSparse(rt, ID(id), nsBytes, local)
	}
	if idx >= len(s.Dense) {
		s.Dense = append(s.Dense, make([]Entry, idx-len(s.Dense)+1)...)
	}
	entry := s.Dense[idx]
	if entry.LocalLen != 0 || entry.NSLen != 0 || entry.Sym != 0 || entry.NS != 0 {
		return entry
	}
	entry = s.storeEntry(rt, nsBytes, local)
	s.Dense[idx] = entry
	return entry
}

func (s *State) internSparse(rt *runtime.Schema, id ID, nsBytes, local []byte) Entry {
	if s == nil {
		return Entry{}
	}
	if s.Sparse == nil {
		s.Sparse = make(map[ID]Entry)
	}
	if entry, ok := s.Sparse[id]; ok {
		return entry
	}
	if len(s.Sparse) >= maxDenseNameMapSize {
		nsID := NamespaceID(rt, nsBytes)
		sym := runtime.SymbolID(0)
		if nsID != 0 && rt != nil {
			sym = rt.Symbols.Lookup(nsID, local)
		}
		return Entry{Sym: sym, NS: nsID}
	}
	entry := s.storeEntry(rt, nsBytes, local)
	s.Sparse[id] = entry
	return entry
}

func (s *State) storeEntry(rt *runtime.Schema, nsBytes, local []byte) Entry {
	localOff := len(s.Local)
	s.Local = append(s.Local, local...)
	nsOff := len(s.NS)
	s.NS = append(s.NS, nsBytes...)
	nsID := NamespaceID(rt, nsBytes)
	sym := runtime.SymbolID(0)
	if nsID != 0 && rt != nil {
		sym = rt.Symbols.Lookup(nsID, local)
	}
	return Entry{
		Sym:      sym,
		NS:       nsID,
		LocalOff: uint32(localOff),
		LocalLen: uint32(len(local)),
		NSOff:    uint32(nsOff),
		NSLen:    uint32(len(nsBytes)),
	}
}

// Parts returns the stored namespace and local-name bytes for an interned ID.
func (s *State) Parts(id ID) ([]byte, []byte) {
	if s == nil || id == 0 {
		return nil, nil
	}
	entry, ok := s.entryForID(id)
	if !ok {
		return nil, nil
	}
	return s.EntryBytes(entry)
}

// EntryBytes returns the stored namespace and local-name bytes for one entry.
func (s *State) EntryBytes(entry Entry) ([]byte, []byte) {
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

func (s *State) entryForID(id ID) (Entry, bool) {
	idx := int(id)
	if idx < len(s.Dense) {
		entry := s.Dense[idx]
		if entry.LocalLen != 0 || entry.NSLen != 0 || entry.Sym != 0 || entry.NS != 0 {
			return entry, true
		}
	}
	if s.Sparse == nil {
		return Entry{}, false
	}
	entry, ok := s.Sparse[id]
	return entry, ok
}

// Stabilize stores raw attribute name bytes in the session-local buffers.
func (s *State) Stabilize(local, ns []byte, cached bool) ([]byte, []byte, bool) {
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

// PushNamespaceScope captures the declarations introduced by one start element.
func (s *State) PushNamespaceScope(decls []xmlstream.NamespaceDecl) {
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
		s.NamespaceDecls = append(s.NamespaceDecls, NamespaceDecl{
			PrefixOff:  uint32(prefixOff),
			PrefixLen:  uint32(prefixLen),
			NSOff:      uint32(nsOff),
			NSLen:      uint32(nsLen),
			PrefixHash: runtime.HashBytes(prefixBytes),
		})
	}
	s.Scopes.Push(ScopeFrame{Off: uint32(off), Len: uint32(declLen), CacheOff: uint32(cacheOff)})
}

// PopNamespaceScope discards the namespace declarations introduced by the current element.
func (s *State) PopNamespaceScope() {
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

// LookupNamespace resolves a prefix against the current element scope.
func (s *State) LookupNamespace(prefix []byte) ([]byte, bool) {
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

func (s *State) lookupNamespaceSmall(prefix []byte, frames []ScopeFrame) ([]byte, bool) {
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

func (s *State) lookupNamespaceHashed(prefix []byte, frames []ScopeFrame) ([]byte, bool) {
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

func (s *State) lookupNamespaceFromCache(prefix []byte, hash uint64) ([]byte, bool, bool) {
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

func (s *State) cachedNamespace(entry *PrefixEntry) ([]byte, bool, bool) {
	if !entry.OK {
		return nil, false, true
	}
	if entry.NSLen == 0 {
		return nil, true, true
	}
	return s.NS[entry.NSOff : entry.NSOff+entry.NSLen], true, true
}

func (s *State) lookupNamespaceByHash(prefix []byte, frames []ScopeFrame, hash uint64) ([]byte, bool, bool) {
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

func (s *State) prefixCacheForCurrent() []PrefixEntry {
	frame, ok := s.Scopes.Peek()
	if !ok || int(frame.CacheOff) >= len(s.PrefixCache) {
		return nil
	}
	return s.PrefixCache[frame.CacheOff:]
}

func (s *State) cachePrefix(prefix, ns []byte, ok bool, hash uint64) {
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
	s.PrefixCache = append(s.PrefixCache, PrefixEntry{
		Hash:      hash,
		PrefixOff: uint32(prefixOff),
		PrefixLen: uint32(prefixLen),
		NSOff:     uint32(nsOff),
		NSLen:     uint32(nsLen),
		OK:        ok,
	})
}

func (s *State) cachePrefixDecl(decl NamespaceDecl, ok bool, hash uint64) {
	s.PrefixCache = append(s.PrefixCache, PrefixEntry{
		Hash:      hash,
		PrefixOff: decl.PrefixOff,
		PrefixLen: decl.PrefixLen,
		NSOff:     decl.NSOff,
		NSLen:     decl.NSLen,
		OK:        ok,
	})
}

// Reset clears runtime name and namespace state while retaining capacity.
func (s *State) Reset() {
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

// Shrink releases oversized backing storage after a session reset.
func (s *State) Shrink(bufferLimit, entryLimit int) {
	if s == nil {
		return
	}
	s.Local = shrinkSliceCap(s.Local, bufferLimit)
	s.NS = shrinkSliceCap(s.NS, bufferLimit)
	s.Dense = shrinkSliceCap(s.Dense, entryLimit)
	s.NamespaceDecls = shrinkSliceCap(s.NamespaceDecls, entryLimit)
	s.PrefixCache = shrinkSliceCap(s.PrefixCache, entryLimit)
	if s.Scopes.Cap() > entryLimit {
		s.Scopes.Drop()
	}
}

func isXMLPrefix(prefix []byte) bool {
	return len(prefix) == 3 && prefix[0] == 'x' && prefix[1] == 'm' && prefix[2] == 'l'
}

func checkedSpan(off, ln uint32, size int) (start, end int, ok bool) {
	start = int(off)
	end = start + int(ln)
	if start < 0 || end < start || end > size {
		return 0, 0, false
	}
	return start, end, true
}

func shrinkSliceCap[T any](in []T, limit int) []T {
	if cap(in) > limit {
		return nil
	}
	return in[:0]
}
