package preprocessor

import (
	"io"
)

type loadSession struct {
	doc      io.ReadCloser
	loader   *Loader
	key      loadKey
	systemID string
	journal  Journal[loadKey]
}

func newLoadSession(loader *Loader, systemID string, key loadKey, doc io.ReadCloser) *loadSession {
	return &loadSession{
		loader:   loader,
		systemID: systemID,
		key:      key,
		doc:      doc,
	}
}
