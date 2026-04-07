package compiler

import (
	"io/fs"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// LoaderConfig holds configuration for the schema loader.
type LoaderConfig struct {
	FS                          fs.FS
	Resolver                    SchemaResolver
	DocumentPool                *parser.DocumentPool
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
}

// Loader loads XML schemas with import/include resolution.
// It is not safe for concurrent use.
type Loader struct {
	resolver SchemaResolver
	imports  Tracker[loadKey]
	state    loadState
	config   LoaderConfig
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg LoaderConfig) *Loader {
	res := cfg.Resolver
	if res == nil && cfg.FS != nil {
		res = NewFSResolver(cfg.FS)
	}
	if cfg.DocumentPool == nil {
		cfg.DocumentPool = parser.NewDocumentPool()
	}
	return &Loader{
		config:   cfg,
		state:    newLoadState(),
		imports:  NewTracker[loadKey](),
		resolver: res,
	}
}

func (l *Loader) loadKey(systemID string, etn model.NamespaceURI) loadKey {
	return loadKey{systemID: systemID, etn: etn}
}

func (l *Loader) cleanupEntryIfUnused(key loadKey) {
	entry, ok := l.state.entry(key)
	if !ok || entry == nil {
		return
	}
	if entry.state != schemaStateUnknown || entry.schema != nil {
		return
	}
	if entry.pending.Count != 0 || len(entry.pending.Directives) != 0 {
		return
	}
	l.state.deleteEntry(key)
}
