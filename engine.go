package xsd

import (
	"io"
	"sync"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type engine struct {
	rt   *runtime.Schema
	pool sync.Pool
	opts []xmlstream.Option
}

func newEngine(rt *runtime.Schema, opts ...xmlstream.Option) *engine {
	e := &engine{rt: rt}
	if len(opts) > 0 {
		e.opts = append([]xmlstream.Option(nil), opts...)
	}
	e.pool.New = func() any {
		return validator.NewSession(rt, e.opts...)
	}
	return e
}

func (e *engine) validate(r io.Reader) error {
	return e.validateDocument(r, "")
}

func (e *engine) validateWithDocument(r io.Reader, document string) error {
	return e.validateDocument(r, document)
}

func (e *engine) validateDocument(r io.Reader, document string) error {
	if e == nil || e.rt == nil {
		return schemaNotLoadedError()
	}
	if r == nil {
		return nilReaderError()
	}

	session := e.acquire()
	err := session.ValidateWithDocument(r, document)
	e.release(session)
	return err
}

func (e *engine) acquire() *validator.Session {
	if e == nil {
		return nil
	}
	if v := e.pool.Get(); v != nil {
		return v.(*validator.Session)
	}
	return validator.NewSession(e.rt, e.opts...)
}

func (e *engine) release(s *validator.Session) {
	if e == nil || s == nil {
		return
	}
	s.Reset()
	e.pool.Put(s)
}

func schemaNotLoadedError() error {
	return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
}

func nilReaderError() error {
	return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, "nil reader", "")}
}
