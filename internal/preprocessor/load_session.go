package preprocessor

import (
	"io"

	"github.com/jacoelho/xsd/internal/loadguard"
	"github.com/jacoelho/xsd/internal/parser"
)

type loadSession struct {
	doc      io.ReadCloser
	loader   *Loader
	key      loadKey
	systemID string
	journal  stateJournal
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

func newLoadSession(loader *Loader, systemID string, key loadKey, doc io.ReadCloser) *loadSession {
	return &loadSession{
		loader:   loader,
		systemID: systemID,
		key:      key,
		doc:      doc,
	}
}

func (s *loadSession) handleCircularLoad() (*parser.Schema, error) {
	return loadguard.CheckCircular[loadKey, *parser.Schema](&s.loader.state, s.key, s.systemID)
}
