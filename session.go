package xsd

import (
	"io"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validate"
)

// ValidateOptions controls instance validation.
type ValidateOptions struct {
	// MaxErrors limits collected validation errors. Zero means unlimited.
	MaxErrors int
	// MaxIdentityScopes limits active identity-constraint scopes. Zero means unlimited.
	MaxIdentityScopes int
	// MaxIdentityEntries limits stored ID, IDREF, key, unique, and keyref entries. Zero means unlimited.
	MaxIdentityEntries int
	// MaxIdentityTupleBytes limits the byte length of one stored identity key. Zero means unlimited.
	MaxIdentityTupleBytes int64
	// MaxInstanceDepth limits nested XML elements. Zero means unlimited.
	MaxInstanceDepth int
	// MaxInstanceAttributes limits attributes on one XML element. Zero means unlimited.
	MaxInstanceAttributes int
	// MaxInstanceTextBytes limits retained character data bytes. Zero means unlimited.
	MaxInstanceTextBytes int64
	// MaxInstanceTokenBytes limits retained XML token payload bytes. Zero means unlimited.
	MaxInstanceTokenBytes int64
}

// Session validates XML instance documents against one Engine.
//
// A Session is not goroutine-safe. Use Engine.Validate or separate sessions for
// concurrent validation.
type Session struct {
	session validate.Session
}

// Validate validates one XML instance document.
func (e *Engine) Validate(r io.Reader) error {
	return e.ValidateWithOptions(r, ValidateOptions{})
}

// ValidateWithOptions validates one XML instance document with options.
func (e *Engine) ValidateWithOptions(r io.Reader, opts ValidateOptions) error {
	var rt *runtime.Schema
	if e != nil {
		rt = e.rt
	}
	return validate.Validate(rt, r, internalValidateOptions(opts))
}

// NewSession creates a reusable validation session. Reused sessions retain
// bounded scratch buffers and string caches; create a new session to release
// retained cache contents.
func (e *Engine) NewSession(opts ValidateOptions) (*Session, error) {
	var rt *runtime.Schema
	if e != nil {
		rt = e.rt
	}
	inner, err := validate.NewSession(rt, internalValidateOptions(opts))
	if err != nil {
		return nil, err
	}
	return &Session{session: inner}, nil
}

// Validate validates one XML instance document and resets validation state
// first. It may retain bounded scratch buffers and string caches for reuse.
func (s *Session) Validate(r io.Reader) error {
	if s == nil {
		return (*validate.Session)(nil).Validate(r)
	}
	return s.session.Validate(r)
}

// Reset clears validation state while preserving options. It may retain bounded
// scratch buffers and string caches; create a new session to release retained
// cache contents.
func (s *Session) Reset() {
	if s == nil {
		return
	}
	s.session.Reset()
}

func internalValidateOptions(opts ValidateOptions) validate.Options {
	return validate.Options{
		MaxErrors:             opts.MaxErrors,
		MaxIdentityScopes:     opts.MaxIdentityScopes,
		MaxIdentityEntries:    opts.MaxIdentityEntries,
		MaxIdentityTupleBytes: opts.MaxIdentityTupleBytes,
		MaxInstanceDepth:      opts.MaxInstanceDepth,
		MaxInstanceAttributes: opts.MaxInstanceAttributes,
		MaxInstanceTextBytes:  opts.MaxInstanceTextBytes,
		MaxInstanceTokenBytes: opts.MaxInstanceTokenBytes,
	}
}
