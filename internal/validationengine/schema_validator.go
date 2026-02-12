package validationengine

import (
	"io"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// SchemaValidator is a non-pooled validator around one mutable session.
type SchemaValidator struct {
	session *validator.Session
}

// NewSchemaValidator creates a mutable, single-session schema validator.
func NewSchemaValidator(schema *runtime.Schema, opts ...xmlstream.Option) *SchemaValidator {
	return &SchemaValidator{session: validator.NewSession(schema, opts...)}
}

// Validate validates one XML document.
func (v *SchemaValidator) Validate(r io.Reader) error {
	return v.ValidateWithDocument(r, "")
}

// ValidateWithDocument validates one XML document with a source URI.
func (v *SchemaValidator) ValidateWithDocument(r io.Reader, document string) error {
	if v == nil || v.session == nil {
		return validator.NewSession(nil).ValidateWithDocument(r, document)
	}
	return v.session.ValidateWithDocument(r, document)
}

// Reset clears current session state while retaining buffers.
func (v *SchemaValidator) Reset() {
	if v == nil || v.session == nil {
		return
	}
	v.session.Reset()
}
