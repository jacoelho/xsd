package xsd

import (
	"io"

	"github.com/jacoelho/xsd/internal/validate"
)

// ValidateOptions controls instance validation.
type ValidateOptions struct {
	// MaxErrors caps reported recoverable validation errors. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxErrors int
	// MaxIdentityScopes caps active identity-constraint scopes. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxIdentityScopes int
	// MaxIdentityEntries caps stored identity entries and pending identity matches. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxIdentityEntries int
	// MaxIdentityTupleBytes caps the canonical text bytes of one identity entry. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxIdentityTupleBytes int64
	// MaxInstanceDepth caps nested XML elements. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstanceDepth int
	// MaxInstanceAttributes caps attributes on one XML element. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstanceAttributes int
	// MaxInstanceTextBytes caps retained character data bytes. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstanceTextBytes int64
	// MaxInstanceTokenBytes caps parser-owned bytes for one XML token. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstanceTokenBytes int64
	// MaxInstanceBytes caps aggregate raw XML bytes read. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstanceBytes int64
	// MaxInstanceValueWork caps cumulative lexical and evaluation work for one simple value. Zero uses the finite default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstanceValueWork uint64
	// MaxInstancePatternWork caps regular-expression matching work for one simple value. Zero uses the finite default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstancePatternWork uint64
	// MaxInstanceTotalPatternWork caps regular-expression matching work summed over one document. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#validation.
	MaxInstanceTotalPatternWork uint64
}

// Session validates XML instance documents against one Engine.
//
// Copies of a Session refer to the same reusable state. Concurrent use of one
// session, including through copies, fails with CodeValidationSession. Use
// Engine.Validate or separately constructed sessions for concurrent validation.
// Only Engine.NewSession creates a usable Session: a zero or nil Session
// fails with xsderrors.CodeValidationSession.
type Session struct {
	session *validate.Session
}

// Validate validates one XML instance document.
// A nil r, and an error returned by r other than io.EOF itself (an error
// wrapping io.EOF included), are reported as [xsderrors.CodeValidationRead], with the cause preserved for errors.Is.
func (e *Engine) Validate(r io.Reader) error {
	return e.ValidateWithOptions(r, ValidateOptions{})
}

// ValidateWithOptions validates one XML instance document with options.
// A nil r, and an error returned by r other than io.EOF itself (an error
// wrapping io.EOF included), are reported as [xsderrors.CodeValidationRead], with the cause preserved for errors.Is.
func (e *Engine) ValidateWithOptions(r io.Reader, opts ValidateOptions) error {
	var pool *validate.SessionPool
	if e != nil {
		pool = e.pool
	}
	return pool.Validate(r, validate.Options(opts))
}

// NewSession creates a reusable validation session. Reused sessions retain
// bounded scratch buffers and string caches; create a new session to release
// retained cache contents.
func (e *Engine) NewSession(opts ValidateOptions) (*Session, error) {
	var pool *validate.SessionPool
	if e != nil {
		pool = e.pool
	}
	inner, err := pool.NewSession(validate.Options(opts))
	if err != nil {
		return nil, err
	}
	return &Session{session: inner}, nil
}

// Validate validates one XML instance document. It clears document-local state
// before returning and may retain bounded scratch buffers and string caches for
// reuse.
// A nil r, and an error returned by r other than io.EOF itself (an error
// wrapping io.EOF included), are reported as [xsderrors.CodeValidationRead], with the cause preserved for errors.Is.
func (s *Session) Validate(r io.Reader) error {
	if s == nil {
		return (*validate.Session)(nil).Validate(r)
	}
	return s.session.Validate(r)
}
