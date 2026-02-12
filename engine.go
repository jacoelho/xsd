package xsd

import (
	"io"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validationengine"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type engine struct {
	rt       *runtime.Schema
	validate *validationengine.Engine
}

func newEngine(rt *runtime.Schema, opts ...xmlstream.Option) *engine {
	return &engine{
		rt:       rt,
		validate: validationengine.NewEngine(rt, opts...),
	}
}

func (e *engine) validateDocument(r io.Reader, document string) error {
	if e == nil || e.validate == nil {
		return schemaNotLoadedError()
	}
	if r == nil {
		return nilReaderError()
	}
	return e.validate.ValidateWithDocument(r, document)
}

func schemaNotLoadedError() error {
	return errors.ValidationList{errors.NewValidation(errors.ErrSchemaNotLoaded, "schema not loaded", "")}
}

func nilReaderError() error {
	return errors.ValidationList{errors.NewValidation(errors.ErrXMLParse, "nil reader", "")}
}
