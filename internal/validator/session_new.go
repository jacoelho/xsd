package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// NewSession creates a new runtime validation session.
func NewSession(rt *runtime.Schema, opts ...xmlstream.Option) *Session {
	sess := &Session{rt: rt}
	if len(opts) > 0 {
		sess.parseOptions = append([]xmlstream.Option(nil), opts...)
	}
	sess.readerFactory = xmlstream.NewReader
	sess.icState.arena = &sess.Arena
	return sess
}
