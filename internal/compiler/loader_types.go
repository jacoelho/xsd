package compiler

import (
	"io/fs"

	"github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

// LoaderConfig holds configuration for the schema loader.
type LoaderConfig struct {
	FS                          fs.FS
	Resolver                    SchemaResolver
	DocumentPool                *schemaast.DocumentPool
	SchemaParseOptions          []xmlstream.Option
	AllowMissingImportLocations bool
}

// Loader loads XML schemas with import/include resolution.
// It is not safe for concurrent use.
type Loader struct {
	resolver SchemaResolver
	config   LoaderConfig
}

// NewLoader creates a new schema loader with the given configuration
func NewLoader(cfg LoaderConfig) *Loader {
	res := cfg.Resolver
	if res == nil && cfg.FS != nil {
		res = NewFSResolver(cfg.FS)
	}
	if cfg.DocumentPool == nil {
		cfg.DocumentPool = schemaast.NewDocumentPool()
	}
	return &Loader{
		config:   cfg,
		resolver: res,
	}
}

func (l *Loader) loadKey(systemID string, etn schemaast.NamespaceURI) loadKey {
	return loadKey{systemID: systemID, etn: etn}
}

type loadKey struct {
	systemID string
	etn      schemaast.NamespaceURI
}
