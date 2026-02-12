package validationengine

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// NewSession creates a standalone validation session.
func NewSession(schema *runtime.Schema, opts ...xmlstream.Option) *validator.Session {
	return validator.NewSession(schema, opts...)
}
