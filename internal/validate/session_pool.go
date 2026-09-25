package validate

import (
	"io"
	"sync/atomic"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
)

// SessionPool owns automatic validation scratch reuse for one immutable schema.
// It retains at most one idle raw session; concurrent callers that miss the
// slot allocate private scratch and never wait for another validation.
type SessionPool struct {
	rt   *xsdSchema.Schema
	idle atomic.Pointer[session]
}

// NewSessionPool creates an automatic validation-session owner for rt. A nil
// runtime is retained so callers can preserve option-error precedence before
// the owner reports its missing-schema invariant.
func NewSessionPool(rt *xsdSchema.Schema) *SessionPool {
	return &SessionPool{rt: rt}
}

// Validate validates one document with options and returns ordinary-call
// scratch to the single idle slot after reset. Options are normalized before
// the schema or idle-session state is inspected.
func (p *SessionPool) Validate(r io.Reader, opts Options) error {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return err
	}
	if p == nil || p.rt == nil {
		return xsderrors.InternalInvariant("nil validation schema")
	}

	scratch := p.idle.Swap(nil)
	if scratch == nil {
		scratch = new(session)
		initializeSession(scratch, p.rt, limits)
	} else {
		scratch.setLimits(limits)
	}

	err = scratch.validate(r)
	scratch.reset()
	p.idle.CompareAndSwap(nil, scratch)
	return err
}

// NewSession creates an independently guarded reusable session for the pool's
// immutable schema. It does not use or populate the automatic idle slot.
func (p *SessionPool) NewSession(opts Options) (*Session, error) {
	limits, err := NormalizeOptions(opts)
	if err != nil {
		return nil, err
	}
	if p == nil || p.rt == nil {
		return nil, xsderrors.InternalInvariant("nil validation schema")
	}
	result := new(Session)
	initializeSession(&result.session, p.rt, limits)
	return result, nil
}

func (s *session) setLimits(limits Limits) {
	s.limits = limits
	s.doc.identity.limits = identityLimits{
		Entries:    limits.IdentityEntries,
		TupleBytes: limits.IdentityTupleBytes,
	}
	s.doc.identity.maxScopes = limits.IdentityScopes
}
