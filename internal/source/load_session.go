package source

import (
	"io"

	"github.com/jacoelho/xsd/internal/loadgraph"
	"github.com/jacoelho/xsd/internal/parser"
)

type loadSession struct {
	doc      io.ReadCloser
	loader   *SchemaLoader
	key      loadKey
	systemID string
	pending  []pendingChange
	merged   mergedChanges
}

type directiveLoadStatus uint8

const (
	directiveLoadStatusLoaded directiveLoadStatus = iota
	directiveLoadStatusDeferred
	directiveLoadStatusSkippedMissing
)

type directiveLoadResult struct {
	schema *parser.Schema
	target loadKey
	status directiveLoadStatus
}

type pendingChange struct {
	sourceKey loadKey
	targetKey loadKey
	kind      parser.DirectiveKind
}

type mergedChanges struct {
	includes []mergeRecord
	imports  []mergeRecord
}

type mergeRecord struct {
	base   loadKey
	target loadKey
}

func newLoadSession(loader *SchemaLoader, systemID string, key loadKey, doc io.ReadCloser) *loadSession {
	return &loadSession{
		loader:   loader,
		systemID: systemID,
		key:      key,
		doc:      doc,
	}
}

func (s *loadSession) handleCircularLoad() (*parser.Schema, error) {
	return loadgraph.CheckCircular[loadKey, *parser.Schema](&s.loader.state, s.key, s.systemID)
}
