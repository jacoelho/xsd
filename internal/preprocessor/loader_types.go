package preprocessor

import (
	"io/fs"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/objects"
	"github.com/jacoelho/xsd/internal/xmltree"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Config holds configuration for the schema loader
type Config struct {
	FS                          fs.FS
	Resolver                    Resolver
	Merger                      loadmerge.SchemaMerger
	DocumentPool                *xmltree.DocumentPool
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
}

// Loader loads XML schemas with import/include resolution.
// It is not safe for concurrent use.
type Loader struct {
	resolver Resolver
	merger   loadmerge.SchemaMerger
	imports  importTracker
	state    loadState
	config   Config
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg Config) *Loader {
	res := cfg.Resolver
	if res == nil && cfg.FS != nil {
		res = NewFSResolver(cfg.FS)
	}
	merger := cfg.Merger
	if merger == nil {
		merger = loadmerge.DefaultMerger{}
	}
	if cfg.DocumentPool == nil {
		cfg.DocumentPool = xmltree.NewDocumentPool()
	}
	return &Loader{
		config:   cfg,
		state:    newLoadState(),
		imports:  newImportTracker(),
		merger:   merger,
		resolver: res,
	}
}

func (l *Loader) loadKey(systemID string, etn objects.NamespaceURI) loadKey {
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
	if entry.pendingCount != 0 || len(entry.pendingDirectives) != 0 {
		return
	}
	l.state.deleteEntry(key)
}
