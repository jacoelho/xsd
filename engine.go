package xsd

import (
	"io"
	"sync"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
)

type engine struct {
	rt   *runtime.Schema
	pool sync.Pool
}

func newEngine(rt *runtime.Schema) *engine {
	e := &engine{rt: rt}
	e.pool.New = func() any {
		return validator.NewSession(rt)
	}
	return e
}

func (e *engine) validate(r io.Reader) error {
	if e == nil || e.rt == nil {
		return schemaNotLoadedError()
	}
	if r == nil {
		return nilReaderError()
	}

	session := e.acquire()
	err := session.Validate(r)
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
	return validator.NewSession(e.rt)
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
