package preprocessor

import (
	"github.com/jacoelho/xsd/internal/objects"
	parser "github.com/jacoelho/xsd/internal/parser"
)

type loadKey struct {
	systemID string
	etn      objects.NamespaceURI
}

type loadState struct {
	entries map[loadKey]*schemaEntry
}

func newLoadState() loadState {
	return loadState{entries: make(map[loadKey]*schemaEntry)}
}

type schemaLoadState int

const (
	schemaStateUnknown schemaLoadState = iota
	schemaStateLoading
	schemaStateLoaded
)

type schemaEntry struct {
	schema            *parser.Schema
	pendingDirectives []pendingDirective
	includeInserted   []int
	state             schemaLoadState
	pendingCount      int
}

func (s *loadState) entry(key loadKey) (*schemaEntry, bool) {
	entry, ok := s.entries[key]
	return entry, ok
}

func (s *loadState) ensureEntry(key loadKey) *schemaEntry {
	if entry, ok := s.entries[key]; ok {
		return entry
	}
	entry := &schemaEntry{}
	s.entries[key] = entry
	return entry
}

func (s *loadState) deleteEntry(key loadKey) {
	delete(s.entries, key)
}

func (s *loadState) loadedSchema(key loadKey) (*parser.Schema, bool) {
	entry, ok := s.entries[key]
	if !ok || entry.state != schemaStateLoaded || entry.schema == nil {
		return nil, false
	}
	return entry.schema, true
}

func (s *loadState) IsLoading(key loadKey) bool {
	entry, ok := s.entries[key]
	return ok && entry.state == schemaStateLoading
}

func (s *loadState) LoadingValue(key loadKey) (*parser.Schema, bool) {
	entry, ok := s.entries[key]
	if !ok || entry.state != schemaStateLoading {
		return nil, false
	}
	return entry.schema, true
}

func (s *loadState) schemaForKey(key loadKey) *parser.Schema {
	entry, ok := s.entries[key]
	if !ok {
		return nil
	}
	return entry.schema
}

type pendingDirective struct {
	targetKey         loadKey
	schemaLocation    string
	expectedNamespace string
	includeDeclIndex  int
	includeIndex      int
	kind              parser.DirectiveKind
}
