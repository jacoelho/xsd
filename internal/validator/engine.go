package validator

import (
	"io"
	"slices"
	"sync"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

// Engine validates XML documents against an immutable runtime schema.
type Engine struct {
	rt   *runtime.Schema
	plan runtime.SessionPlan
	pool sync.Pool
	opts []xmlstream.Option
}

// NewEngine creates an engine backed by pooled validation sessions.
func NewEngine(schema *runtime.Schema, opts ...xmlstream.Option) *Engine {
	engine := &Engine{rt: schema, plan: runtime.NewSessionPlan(schema)}
	if len(opts) != 0 {
		engine.opts = slices.Clone(opts)
	}
	engine.pool.New = func() any {
		return NewSessionWithPlan(schema, engine.plan, engine.opts...)
	}
	return engine
}

// Validate validates one XML document.
func (e *Engine) Validate(r io.Reader) error {
	return e.ValidateWithDocument(r, "")
}

// ValidateWithDocument validates one XML document and attaches a document URI.
func (e *Engine) ValidateWithDocument(r io.Reader, document string) error {
	if e == nil || e.rt == nil {
		return schemaNotLoadedError()
	}
	session := e.acquire()
	err := session.ValidateWithDocument(r, document)
	e.release(session)
	return err
}

func (e *Engine) acquire() *Session {
	if e == nil {
		return nil
	}
	if session := e.pool.Get(); session != nil {
		return session.(*Session)
	}
	return NewSessionWithPlan(e.rt, e.plan, e.opts...)
}

func (e *Engine) release(session *Session) {
	if e == nil || session == nil {
		return
	}
	session.Reset()
	e.pool.Put(session)
}
