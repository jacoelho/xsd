package source

import (
	"io/fs"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Config holds configuration for the schema loader
type Config struct {
	FS                          fs.FS
	Resolver                    Resolver
	Merger                      loadmerge.SchemaMerger
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
	// DocumentPool owns parser DOM scratch documents for this loader instance.
	DocumentPool *xsdxml.DocumentPool
}

// SchemaLoader loads XML schemas with import/include resolution.
// It is not safe for concurrent use.
type SchemaLoader struct {
	resolver Resolver
	merger   loadmerge.SchemaMerger
	imports  importTracker
	state    loadState
	config   Config
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg Config) *SchemaLoader {
	res := cfg.Resolver
	if res == nil && cfg.FS != nil {
		res = NewFSResolver(cfg.FS)
	}
	merger := cfg.Merger
	if merger == nil {
		merger = loadmerge.DefaultMerger{}
	}
	if cfg.DocumentPool == nil {
		cfg.DocumentPool = xsdxml.NewDocumentPool()
	}
	return &SchemaLoader{
		config:   cfg,
		state:    newLoadState(),
		imports:  newImportTracker(),
		merger:   merger,
		resolver: res,
	}
}

func (l *SchemaLoader) loadKey(systemID string, etn model.NamespaceURI) loadKey {
	return loadKey{systemID: systemID, etn: etn}
}

func (l *SchemaLoader) cleanupEntryIfUnused(key loadKey) {
	entry, ok := l.state.entry(key)
	if !ok || entry == nil {
		return
	}
	if entry.state != schemaStateUnknown || entry.schema != nil {
		return
	}
	if entry.pendingCount != 0 || len(entry.pendingDirectives) != 0 {
		return
	}
	l.state.deleteEntry(key)
}
